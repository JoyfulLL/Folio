package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestDailyReportAggregateCurrencySummaries(t *testing.T) {
	location := time.FixedZone("Test Timezone", 8*60*60)

	// 2026-11-10 00:00:00 through 2026-11-10 23:59:59 in UTC+8; the production code passes the
	// local time of the last second of the reporting date because the day boundary itself
	// (2026-11-11 00:00:00) belongs to the next report
	startLocalDateTime := int64(20261110000000)
	endLocalDateTime := int64(20261110235959)

	localTime := func(year int, month time.Month, day int, hour int, minute int) time.Time {
		return time.Date(year, month, day, hour, minute, 0, 0, location)
	}

	newTransaction := func(transactionType models.TransactionDbType, accountId int64, transactionTime time.Time, amount int64) *models.Transaction {
		return &models.Transaction{
			Type:            transactionType,
			AccountId:       accountId,
			TransactionTime: utils.GetMinTransactionTimeFromUnixTime(transactionTime.Unix()),
			Amount:          amount,
		}
	}

	transactions := []*models.Transaction{
		// yesterday's CNY expense and income
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 1, localTime(2026, time.November, 10, 10, 0), 100000),
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 1, localTime(2026, time.November, 10, 12, 0), 30000),
		newTransaction(models.TRANSACTION_DB_TYPE_INCOME, 1, localTime(2026, time.November, 10, 18, 0), 500000),
		// yesterday's JPY expense (amounts are stored as cents-like values for every currency)
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 2, localTime(2026, time.November, 10, 9, 0), 125500),
		// a transaction of an account without currency falls back to UNKNOWN
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 3, localTime(2026, time.November, 10, 9, 30), 700),
		// the day before the reporting date is excluded
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 1, localTime(2026, time.November, 9, 23, 0), 99900),
		// the reporting date end boundary is exclusive
		newTransaction(models.TRANSACTION_DB_TYPE_EXPENSE, 1, localTime(2026, time.November, 11, 0, 0), 88800),
		// transfers are never included in the report totals
		newTransaction(models.TRANSACTION_DB_TYPE_TRANSFER_OUT, 1, localTime(2026, time.November, 10, 15, 0), 77700),
	}

	accountMap := map[int64]*models.Account{
		1: {AccountId: 1, Currency: "CNY"},
		2: {AccountId: 2, Currency: "JPY"},
		3: {AccountId: 3},
	}

	summaries := dailyReportAggregateCurrencySummaries(transactions, accountMap, startLocalDateTime, endLocalDateTime, location)

	assert.Len(t, summaries, 3)

	// currencies are sorted by code, and amounts stay in their own currency
	assert.Equal(t, "CNY", summaries[0].Currency)
	assert.Equal(t, int64(130000), summaries[0].ExpenseAmount)
	assert.Equal(t, int64(2), summaries[0].ExpenseCount)
	assert.Equal(t, int64(500000), summaries[0].IncomeAmount)
	assert.Equal(t, int64(1), summaries[0].IncomeCount)

	assert.Equal(t, "JPY", summaries[1].Currency)
	assert.Equal(t, int64(125500), summaries[1].ExpenseAmount)
	assert.Equal(t, int64(1), summaries[1].ExpenseCount)

	assert.Equal(t, "UNKNOWN", summaries[2].Currency)
	assert.Equal(t, int64(700), summaries[2].ExpenseAmount)
}

func TestDailyReportBuildSuccessCard(t *testing.T) {
	location := time.FixedZone("Test Timezone", 8*60*60)
	// 2026-11-10 is a Tuesday
	reportDate := time.Date(2026, time.November, 10, 0, 0, 0, 0, location)

	dailySummaries := []*dailyReportCurrencySummary{
		{Currency: "CNY", IncomeAmount: 500000, ExpenseAmount: 100000, IncomeCount: 1, ExpenseCount: 2},
	}
	monthlySummaries := []*dailyReportCurrencySummary{
		{Currency: "CNY", IncomeAmount: 5000000, ExpenseAmount: 1000000},
	}

	card := DailyReports.buildDailyReportSuccessCard(reportDate, dailySummaries, monthlySummaries)

	header, ok := card["header"].(map[string]any)
	assert.True(t, ok)

	title, ok := header["title"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "记账日报", title["content"])

	subtitle, ok := header["subtitle"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "11月10日(火曜日)日报", subtitle["content"])

	assert.Equal(t, "wathet", header["template"])

	config, ok := card["config"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, config["wide_screen_mode"])

	elements, ok := card["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, elements, 4)
	assert.Equal(t, "hr", elements[0].(map[string]any)["tag"])

	assertDailyReportCardRowValue(t, elements[1], "<font color='green'>¥1000.00，共2笔</font>")
	assertDailyReportCardRowValue(t, elements[2], "<font color='red'>¥5000.00</font>")
	assertDailyReportCardRowValue(t, elements[3], "11月已支出<font color='green'>¥10000.00</font>，已入账<font color='red'>¥50000.00</font>")
}

func TestDailyReportBuildSuccessCardWithCurrenciesOnlyInMonthlySummary(t *testing.T) {
	location := time.FixedZone("Test Timezone", 8*60*60)
	reportDate := time.Date(2026, time.November, 10, 0, 0, 0, 0, location)

	monthlySummaries := []*dailyReportCurrencySummary{
		{Currency: "USD", IncomeAmount: 10000, ExpenseAmount: 20000},
	}

	card := DailyReports.buildDailyReportSuccessCard(reportDate, nil, monthlySummaries)

	elements, ok := card["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, elements, 4)

	// a currency which has no transactions on the reporting date still shows its monthly
	// statistics with zero daily amounts
	assertDailyReportCardRowValue(t, elements[1], "<font color='green'>$0.00，共0笔</font>")
	assertDailyReportCardRowValue(t, elements[3], "11月已支出<font color='green'>$200.00</font>，已入账<font color='red'>$100.00</font>")
}

func TestDailyReportBuildSuccessCardWithEmptySummaries(t *testing.T) {
	location := time.FixedZone("Test Timezone", 8*60*60)
	reportDate := time.Date(2026, time.November, 10, 0, 0, 0, 0, location)

	card := DailyReports.buildDailyReportSuccessCard(reportDate, nil, nil)

	elements, ok := card["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, elements, 4)

	// an empty day falls back to the default currency instead of rendering an empty card
	assertDailyReportCardRowValue(t, elements[1], "<font color='green'>¥0.00，共0笔</font>")
}

func assertDailyReportCardRowValue(t *testing.T, row any, expectedContent string) {
	t.Helper()

	rowMap, ok := row.(map[string]any)
	assert.True(t, ok)

	columns, ok := rowMap["columns"].([]any)
	assert.True(t, ok)
	assert.Len(t, columns, 2)

	valueColumn, ok := columns[1].(map[string]any)
	assert.True(t, ok)

	valueElements, ok := valueColumn["elements"].([]any)
	assert.True(t, ok)
	assert.Len(t, valueElements, 1)

	valueDiv, ok := valueElements[0].(map[string]any)
	assert.True(t, ok)

	valueText, ok := valueDiv["text"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, expectedContent, valueText["content"])
}

func TestDailyReportFormatAmountValue(t *testing.T) {
	assert.Equal(t, "0.00", dailyReportFormatAmountValue(0))
	assert.Equal(t, "1255.00", dailyReportFormatAmountValue(125500))
	assert.Equal(t, "33.00", dailyReportFormatAmountValue(3300))
	assert.Equal(t, "0.07", dailyReportFormatAmountValue(7))
	assert.Equal(t, "-125.50", dailyReportFormatAmountValue(-12550))
}

func TestDailyReportCurrencySymbol(t *testing.T) {
	assert.Equal(t, "¥", dailyReportCurrencySymbol("CNY"))
	assert.Equal(t, "¥", dailyReportCurrencySymbol("JPY"))
	assert.Equal(t, "HK$", dailyReportCurrencySymbol("hkd"))
	assert.Equal(t, "XYZ ", dailyReportCurrencySymbol("XYZ"))
}

func TestDailyReportParseUids(t *testing.T) {
	assert.Equal(t, []int64{1, 2}, dailyReportParseUids("1, 2"))
	assert.Equal(t, []int64{5}, dailyReportParseUids(" 5 "))
	assert.Equal(t, []int64{1, 3}, dailyReportParseUids("1,,3"))

	assert.Nil(t, dailyReportParseUids(""))
	assert.Empty(t, dailyReportParseUids("  "))
	assert.Empty(t, dailyReportParseUids("not-a-number"))
	assert.Empty(t, dailyReportParseUids("0,-1"))
}

func TestDailyReportBuildUserConfig(t *testing.T) {
	config := &settings.Config{
		DailyReportKeyword:              "记账",
		DailyReportSendTimeMinutesOfDay: 300,
		DailyReportTimezoneName:         "Asia/Shanghai",
	}

	// a row with every field set uses the saved values
	row := &models.UserDailyReportSetting{
		Uid:                  1,
		Keyword:              "流水",
		SendTimeMinutesOfDay: 600,
		TimezoneName:         "Asia/Tokyo",
	}

	userConfig := dailyReportBuildUserConfig(config, row, "https://example.com/hook", "secret")
	assert.Equal(t, int64(1), userConfig.uid)
	assert.Equal(t, "https://example.com/hook", userConfig.webhookUrl)
	assert.Equal(t, "secret", userConfig.signingSecret)
	assert.Equal(t, "流水", userConfig.keyword)
	assert.Equal(t, 600, userConfig.sendTimeMinutesOfDay)

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	assert.Nil(t, err)
	assert.Equal(t, tokyo, userConfig.location)

	// a row without optional fields falls back to the server-wide configuration
	emptyRow := &models.UserDailyReportSetting{Uid: 2}
	userConfig = dailyReportBuildUserConfig(config, emptyRow, "https://example.com/hook2", "secret2")
	assert.Equal(t, "记账", userConfig.keyword)
	assert.Equal(t, 300, userConfig.sendTimeMinutesOfDay)

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	assert.Nil(t, err)
	assert.Equal(t, shanghai, userConfig.location)

	// an invalid timezone name falls back to the server-wide timezone
	invalidTzRow := &models.UserDailyReportSetting{Uid: 3, TimezoneName: "Not/AZone"}
	userConfig = dailyReportBuildUserConfig(config, invalidTzRow, "https://example.com/hook3", "secret3")
	assert.Equal(t, shanghai, userConfig.location)
}

func TestDailyReportResolveTestSendConfig(t *testing.T) {
	// the daily report switch in the server configuration is off, which is the default
	config := &settings.Config{
		DailyReportKeyword:              "记账",
		DailyReportSendTimeMinutesOfDay: 300,
		DailyReportTimezoneName:         "Asia/Shanghai",
	}

	// a user who saved the webhook in the web ui can always test it, even when the daily report
	// is not enabled on the server and the user has not enabled it yet
	row := &models.UserDailyReportSetting{
		Uid:                  7,
		Enabled:              false,
		Keyword:              "日报",
		SendTimeMinutesOfDay: 791,
	}

	userConfig, err := dailyReportResolveTestSendConfig(7, config, row, "https://example.com/hook7", "secret7")
	assert.Nil(t, err)
	assert.Equal(t, int64(7), userConfig.uid)
	assert.Equal(t, "https://example.com/hook7", userConfig.webhookUrl)
	assert.Equal(t, "secret7", userConfig.signingSecret)
	assert.Equal(t, "日报", userConfig.keyword)
	assert.Equal(t, 791, userConfig.sendTimeMinutesOfDay)

	// without a saved webhook, the server-wide configuration is the only fallback
	_, err = dailyReportResolveTestSendConfig(7, config, nil, "", "")
	assert.Equal(t, errs.ErrDailyReportNotEnabled, err)

	// the user has saved settings but no webhook yet, so the missing webhook is reported instead
	// of the server-wide switch
	_, err = dailyReportResolveTestSendConfig(7, config, &models.UserDailyReportSetting{Uid: 7}, "", "")
	assert.Equal(t, errs.ErrDailyReportNotConfigured, err)

	// the server-wide switch is on, but the server-wide webhook is not configured
	config.EnableDailyReport = true
	_, err = dailyReportResolveTestSendConfig(7, config, nil, "", "")
	assert.Equal(t, errs.ErrDailyReportNotConfigured, err)

	// the server-wide webhook is used as the fallback
	config.DailyReportWebhookUrl = "https://example.com/global"
	config.DailyReportSigningSecret = "globalsecret"

	userConfig, err = dailyReportResolveTestSendConfig(9, config, nil, "", "")
	assert.Nil(t, err)
	assert.Equal(t, int64(9), userConfig.uid)
	assert.Equal(t, "https://example.com/global", userConfig.webhookUrl)
	assert.Equal(t, "globalsecret", userConfig.signingSecret)
	assert.Equal(t, "记账", userConfig.keyword)
	assert.Equal(t, 300, userConfig.sendTimeMinutesOfDay)
}

// dailyReportTestDatabase is a minimal in-memory database which only returns one saved daily
// report setting, so the test does not need a real database
type dailyReportTestDatabase struct {
	datastore.Database
	setting *models.UserDailyReportSetting
}

type dailyReportTestSession struct {
	datastore.DatabaseSession
	database *dailyReportTestDatabase
	query    any
	args     []any
}

func (db *dailyReportTestDatabase) NewSession(c core.Context) datastore.DatabaseSession {
	return &dailyReportTestSession{
		database: db,
	}
}

func (db *dailyReportTestDatabase) DoTransaction(c core.Context, fn func(datastore.DatabaseSession) error) error {
	return fn(db.NewSession(c))
}

func (s *dailyReportTestSession) Where(query any, args ...any) datastore.DatabaseSession {
	s.query = query
	s.args = args
	return s
}

func (s *dailyReportTestSession) Get(beans ...any) (bool, error) {
	if len(beans) != 1 {
		return false, nil
	}

	setting, ok := beans[0].(*models.UserDailyReportSetting)

	if !ok || setting == nil {
		return false, nil
	}

	if s.query != "uid=?" || len(s.args) != 1 {
		return false, nil
	}

	uid, ok := s.args[0].(int64)

	if !ok || s.database.setting == nil || s.database.setting.Uid != uid {
		return false, nil
	}

	*setting = *s.database.setting
	return true, nil
}

// TestDailyReportSendTestReportWithPerUserSettings verifies the whole "send test message" path
// of the web ui when the daily report is not enabled in the server configuration, which is the
// default: the webhook saved by the user must be used and must actually receive the test card
func TestDailyReportSendTestReportWithPerUserSettings(t *testing.T) {
	received := make(chan map[string]any, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := make(map[string]any)
		_ = json.Unmarshal(body, &payload)
		received <- payload
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	const secretKey = "daily-report-test-server-secret"

	encryptionKey := utils.DeriveStorageEncryptionKey(secretKey, dailyReportSettingEncryptionDomain)

	webhookUrl, err := utils.EncryptStringWithAESGCM(encryptionKey, dailyReportSettingWebhookUrlAdditionalData, server.URL)

	if err != nil {
		t.Fatal(err)
	}

	signingSecret, err := utils.EncryptStringWithAESGCM(encryptionKey, dailyReportSettingSigningSecretAdditionalData, "test-signing-secret")

	if err != nil {
		t.Fatal(err)
	}

	// EnableDailyReport is false on purpose, because the per-user settings must work without it
	config := &settings.Config{
		SecretKey:                       secretKey,
		DailyReportSendTimeMinutesOfDay: 300,
		DailyReportTimezoneName:         "Asia/Shanghai",
		DailyReportRequestTimeout:       5000,
		DailyReportProxy:                "none",
	}
	assert.False(t, config.EnableDailyReport)

	configContainer := settings.NewConfigContainer(config)

	userDataStore, err := datastore.NewDataStore(&dailyReportTestDatabase{
		setting: &models.UserDailyReportSetting{
			SettingId:            1,
			Uid:                  42,
			Enabled:              true,
			WebhookUrl:           webhookUrl,
			SigningSecret:        signingSecret,
			Keyword:              "记账日报",
			SendTimeMinutesOfDay: 791,
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	userSettings := &UserDailyReportSettingsService{
		ServiceUsingDB:     NewServiceUsingDB(&datastore.DataStoreContainer{UserDataStore: userDataStore}),
		ServiceUsingConfig: NewServiceUsingConfig(configContainer),
	}

	service := &DailyReportService{
		ServiceUsingConfig: NewServiceUsingConfig(configContainer),
		userSettings:       userSettings,
	}

	err = service.SendTestReport(core.NewNullContext(), 42)
	assert.Nil(t, err)

	select {
	case payload := <-received:
		assert.Equal(t, "interactive", payload["msg_type"])
		assert.NotEmpty(t, payload["sign"])

		card, ok := payload["card"].(map[string]any)
		assert.True(t, ok)

		elements, ok := card["elements"].([]any)
		assert.True(t, ok)
		assert.NotEmpty(t, elements)

		keywordElement, ok := elements[0].(map[string]any)
		assert.True(t, ok)

		keywordText, ok := keywordElement["text"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "[记账日报]", keywordText["content"])
	case <-time.After(5 * time.Second):
		t.Fatal("the test message was not received by the webhook")
	}
}
