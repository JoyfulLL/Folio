package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/feishu"
	"github.com/mayswind/ezbookkeeping/pkg/httpclient"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

const dailyReportMaxLastErrorLength = 200
const dailyReportUnknownCurrency = "UNKNOWN"
const dailyReportDefaultCurrencyWhenEmpty = "CNY"

var dailyReportWeekdayNames = []string{"月曜日", "火曜日", "水曜日", "木曜日", "金曜日", "土曜日", "日曜日"}

var dailyReportCurrencySymbols = map[string]string{
	"CNY": "¥", "RMB": "¥", "CNH": "¥",
	"JPY": "¥", "USD": "$", "EUR": "€", "GBP": "£",
	"HKD": "HK$", "TWD": "NT$", "CAD": "C$", "AUD": "A$",
	"SGD": "S$", "CHF": "CHF ", "KRW": "₩",
}

// dailyReportCurrencySummary represents the income and expense amounts of one currency for a
// date range, in the smallest currency unit of that currency (the amounts are not exchanged,
// currencies are never added together)
type dailyReportCurrencySummary struct {
	Currency      string
	IncomeAmount  int64
	ExpenseAmount int64
	IncomeCount   int64
	ExpenseCount  int64
}

// DailyReportService represents daily report service
type DailyReportService struct {
	ServiceUsingDB
	ServiceUsingConfig
	ServiceUsingUuid
	accounts     *AccountService
	transactions *TransactionService
	userSettings *UserDailyReportSettingsService
}

// Initialize a daily report service singleton instance
var (
	DailyReports = &DailyReportService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
		accounts:     Accounts,
		transactions: Transactions,
		userSettings: UserDailyReportSettings,
	}
)

// dailyReportUserConfig represents the effective sending configuration of one user, resolved
// from either the user's own daily report settings saved in the database or the server-wide
// configuration as the fallback
type dailyReportUserConfig struct {
	uid                  int64
	webhookUrl           string
	signingSecret        string
	keyword              string
	sendTimeMinutesOfDay int
	location             *time.Location
}

// RunDailyReports sends the daily income and expense report of every configured user to the
// user's Feishu custom bot webhook. Users with their own daily report settings saved in the
// database (managed in the web ui) always use their own webhook; users listed in the
// server-wide configuration without own settings fall back to the server-wide webhook. The job
// ticks every 15 minutes like the credit card notification job, and each user's local reporting
// date is derived from the configured timezone, so the report is always generated once per
// user-local day. The unique index of uid and report date keeps every report sent only once, so
// restarting the service or running the job more than once never sends the same report twice
func (s *DailyReportService) RunDailyReports(c core.Context, currentUnixTime int64) error {
	config := s.CurrentConfig()

	if !config.EnableDailyReport {
		log.Warnf(c, "[daily_reports.RunDailyReports] daily reports are skipped, because daily report is not enabled")
		return nil
	}

	userConfigs, err := s.resolveUserConfigs(c, config)

	if err != nil {
		log.Errorf(c, "[daily_reports.RunDailyReports] failed to resolve the daily report user configs, because %s", err.Error())
		return err
	}

	if len(userConfigs) < 1 {
		log.Warnf(c, "[daily_reports.RunDailyReports] daily reports are skipped, because no user has enabled the daily report")
		return nil
	}

	var firstError error

	for i := 0; i < len(userConfigs); i++ {
		if err := s.runDailyReportForUser(c, userConfigs[i], config, currentUnixTime); err != nil {
			log.Errorf(c, "[daily_reports.RunDailyReports] failed to send the daily report of user \"uid:%d\", because %s", userConfigs[i].uid, err.Error())

			if firstError == nil {
				firstError = err
			}
		}
	}

	return firstError
}

// resolveUserConfigs returns the effective sending configuration of every user who should
// receive a daily report. Users with enabled settings in the database take precedence; users
// only listed in the server-wide configuration fall back to the server-wide webhook
func (s *DailyReportService) resolveUserConfigs(c core.Context, config *settings.Config) ([]*dailyReportUserConfig, error) {
	var userConfigs []*dailyReportUserConfig
	uidsOwnSettings := make(map[int64]bool)

	rows, err := s.userSettings.GetEnabledSettings(c)

	if err != nil {
		return nil, err
	}

	for i := 0; i < len(rows); i++ {
		row := rows[i]
		userConfig, err := s.buildUserConfigFromRow(c, config, row)

		if err != nil {
			// the stored secrets cannot be decrypted, most likely because the server secret key
			// has been changed; skip the user instead of failing the whole job
			log.Errorf(c, "[daily_reports.resolveUserConfigs] failed to resolve the daily report config of user \"uid:%d\" from the saved settings, because %s", row.Uid, err.Error())
			continue
		}

		userConfigs = append(userConfigs, userConfig)
		uidsOwnSettings[row.Uid] = true
	}

	globalWebhookConfigured := config.DailyReportWebhookUrl != "" && config.DailyReportSigningSecret != ""

	for _, uid := range dailyReportParseUids(config.DailyReportUids) {
		if uidsOwnSettings[uid] {
			continue
		}

		if !globalWebhookConfigured {
			log.Warnf(c, "[daily_reports.resolveUserConfigs] the server-wide fallback of user \"uid:%d\" is skipped, because the feishu webhook url or the signing secret is not configured", uid)
			continue
		}

		userConfigs = append(userConfigs, &dailyReportUserConfig{
			uid:                  uid,
			webhookUrl:           config.DailyReportWebhookUrl,
			signingSecret:        config.DailyReportSigningSecret,
			keyword:              config.DailyReportKeyword,
			sendTimeMinutesOfDay: config.DailyReportSendTimeMinutesOfDay,
			location:             dailyReportLocation(config),
		})
	}

	return userConfigs, nil
}

// buildUserConfigFromRow decrypts the saved webhook url and signing secret of one user and
// fills the unset fields with the server-wide defaults
func (s *DailyReportService) buildUserConfigFromRow(c core.Context, config *settings.Config, row *models.UserDailyReportSetting) (*dailyReportUserConfig, error) {
	webhookUrl, err := s.userSettings.DecryptWebhookUrl(c, row)

	if err != nil {
		return nil, err
	}

	signingSecret, err := s.userSettings.DecryptSigningSecret(c, row)

	if err != nil {
		return nil, err
	}

	return dailyReportBuildUserConfig(config, row, webhookUrl, signingSecret), nil
}

// dailyReportBuildUserConfig merges the saved settings of one user with the server-wide
// defaults: every field the user has not set falls back to the server-wide configuration
func dailyReportBuildUserConfig(config *settings.Config, row *models.UserDailyReportSetting, webhookUrl string, signingSecret string) *dailyReportUserConfig {
	keyword := row.Keyword

	if keyword == "" {
		keyword = config.DailyReportKeyword
	}

	sendTimeMinutesOfDay := config.DailyReportSendTimeMinutesOfDay

	if row.SendTimeMinutesOfDay > 0 {
		sendTimeMinutesOfDay = row.SendTimeMinutesOfDay
	}

	location := dailyReportLocation(config)

	if row.TimezoneName != "" {
		if rowLocation, err := time.LoadLocation(row.TimezoneName); err == nil {
			location = rowLocation
		}
	}

	return &dailyReportUserConfig{
		uid:                  row.Uid,
		webhookUrl:           webhookUrl,
		signingSecret:        signingSecret,
		keyword:              keyword,
		sendTimeMinutesOfDay: sendTimeMinutesOfDay,
		location:             location,
	}
}

func (s *DailyReportService) runDailyReportForUser(c core.Context, userConfig *dailyReportUserConfig, config *settings.Config, currentUnixTime int64) error {
	location := userConfig.location
	now := time.Unix(currentUnixTime, 0).In(location)

	// wait until the configured local time of the reporting timezone has passed, so yesterday's
	// data is complete before the report is generated
	minutesOfDay := now.Hour()*60 + now.Minute()

	if minutesOfDay < userConfig.sendTimeMinutesOfDay {
		return nil
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	reportDate := todayStart.AddDate(0, 0, -1)
	reportDateNumeric := utils.FormatUnixTimeToNumericYearMonthDay(reportDate.Unix(), location)

	reportLog, err := s.getOrAddDailyReportLog(c, userConfig.uid, reportDateNumeric, currentUnixTime)

	if err != nil {
		return err
	}

	if reportLog.Status == models.DAILY_REPORT_LOG_STATUS_SUCCESS {
		return nil
	}

	if reportLog.RetryCount >= config.DailyReportMaxRetryCount {
		// the report keeps failing after the configured retries, so it is skipped until the next
		// day to avoid sending an alert every 15 minutes; the last error is kept in the log row
		return nil
	}

	monthStart := time.Date(reportDate.Year(), reportDate.Month(), 1, 0, 0, 0, 0, location)

	dailySummaries, err := s.getCurrencySummaries(c, userConfig.uid, reportDate, todayStart, location)

	if err != nil {
		s.failDailyReport(c, userConfig, reportLog, err, currentUnixTime, config)
		return err
	}

	monthlySummaries, err := s.getCurrencySummaries(c, userConfig.uid, monthStart, todayStart, location)

	if err != nil {
		s.failDailyReport(c, userConfig, reportLog, err, currentUnixTime, config)
		return err
	}

	card := s.buildDailyReportSuccessCard(reportDate, dailySummaries, monthlySummaries)
	client := s.createFeishuClientForUser(userConfig, config)
	sendErr := client.SendCard(card)

	if sendErr != nil {
		s.failDailyReport(c, userConfig, reportLog, sendErr, currentUnixTime, config)
		return sendErr
	}

	log.Infof(c, "[daily_reports.runDailyReportForUser] the daily report of user \"uid:%d\" for date \"%s\" has been sent to feishu", userConfig.uid, utils.FormatNumericYearMonthDayToLongDate(reportDateNumeric))
	s.updateDailyReportLogResult(c, userConfig.uid, reportLog, models.DAILY_REPORT_LOG_STATUS_SUCCESS, "", currentUnixTime)

	return nil
}

// SendTestReport sends a test card to the feishu webhook of the given user immediately, used by
// the "send test message" button in the web ui. It does not touch the daily report log
func (s *DailyReportService) SendTestReport(c core.Context, uid int64) error {
	config := s.CurrentConfig()

	if !config.EnableDailyReport {
		return errs.ErrDailyReportNotEnabled
	}

	row, err := s.userSettings.GetSettingByUid(c, uid)

	if err != nil {
		return err
	}

	var userConfig *dailyReportUserConfig

	if row != nil && row.Enabled {
		userConfig, err = s.buildUserConfigFromRow(c, config, row)

		if err != nil {
			return errs.ErrDailyReportSavedSecretNotDecodable
		}
	} else {
		if config.DailyReportWebhookUrl == "" || config.DailyReportSigningSecret == "" {
			return errs.ErrDailyReportNotConfigured
		}

		userConfig = &dailyReportUserConfig{
			uid:                  uid,
			webhookUrl:           config.DailyReportWebhookUrl,
			signingSecret:        config.DailyReportSigningSecret,
			keyword:              config.DailyReportKeyword,
			sendTimeMinutesOfDay: config.DailyReportSendTimeMinutesOfDay,
			location:             dailyReportLocation(config),
		}
	}

	now := time.Now()
	card := s.buildDailyReportTestCard(now.In(userConfig.location))
	client := s.createFeishuClientForUser(userConfig, config)

	return client.SendCard(card)
}

// buildDailyReportTestCard builds a simple card for testing the webhook settings
func (s *DailyReportService) buildDailyReportTestCard(now time.Time) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"template": "blue",
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "推送测试成功",
			},
		},
		"elements": []any{
			dailyReportCardRow("服务器时间", now.Format("2006-01-02 15:04:05 (MST)"), ""),
			dailyReportCardRow("说明", "收到这张卡片说明日报推送配置正确，每日日报将按设置的发送时间推送到本群。", ""),
		},
	}
}

// failDailyReport records the failure and sends a feishu alert only for the first failure of
// one reporting date, so a broken webhook does not trigger an alert every 15 minutes
func (s *DailyReportService) failDailyReport(c core.Context, userConfig *dailyReportUserConfig, reportLog *models.DailyReportLog, sendErr error, currentUnixTime int64, config *settings.Config) {
	uid := userConfig.uid
	isFirstFailure := reportLog.RetryCount == 0
	s.updateDailyReportLogResult(c, uid, reportLog, models.DAILY_REPORT_LOG_STATUS_FAILED, sendErr.Error(), currentUnixTime)

	if !isFirstFailure {
		return
	}

	reportDateText := utils.FormatNumericYearMonthDayToLongDate(reportLog.ReportDate)
	alertText := "ezBookkeeping 日报任务失败\n" +
		"目标日期：" + reportDateText + "\n" +
		"最近错误：\n" +
		dailyReportTrimLastError(sendErr.Error()) + "\n" +
		"请检查服务器日志与飞书机器人配置。"

	client := s.createFeishuClientForUser(userConfig, config)
	alertErr := client.SendText(alertText)

	if alertErr != nil {
		log.Errorf(c, "[daily_reports.failDailyReport] failed to send the failure alert of user \"uid:%d\", because %s", uid, alertErr.Error())
	} else {
		log.Infof(c, "[daily_reports.failDailyReport] the failure alert of user \"uid:%d\" has been sent to feishu", uid)
	}
}

func (s *DailyReportService) getOrAddDailyReportLog(c core.Context, uid int64, reportDate int32, currentUnixTime int64) (*models.DailyReportLog, error) {
	db := s.UserDataDB(uid)
	reportLog := &models.DailyReportLog{}
	has, err := db.NewSession(c).Where("uid=? AND report_date=?", uid, reportDate).Get(reportLog)

	if err != nil {
		return nil, err
	}

	if has {
		return reportLog, nil
	}

	reportLog = &models.DailyReportLog{
		ReportLogId:     s.GenerateUuid(uuid.UUID_TYPE_DAILY_REPORT_LOG),
		Uid:             uid,
		ReportDate:      reportDate,
		Status:          models.DAILY_REPORT_LOG_STATUS_PENDING,
		RetryCount:      0,
		CreatedUnixTime: currentUnixTime,
		UpdatedUnixTime: currentUnixTime,
	}

	// the unique index of uid and report date also prevents duplicated reports when multiple
	// instances are running at the same time, so a conflict here only means the other instance
	// has already created the row
	if _, err := db.NewSession(c).Insert(reportLog); err != nil {
		has, getErr := db.NewSession(c).Where("uid=? AND report_date=?", uid, reportDate).Get(reportLog)

		if getErr != nil || !has {
			return nil, err
		}
	}

	return reportLog, nil
}

func (s *DailyReportService) updateDailyReportLogResult(c core.Context, uid int64, reportLog *models.DailyReportLog, status models.DailyReportLogStatus, lastError string, currentUnixTime int64) {
	if status != models.DAILY_REPORT_LOG_STATUS_SUCCESS {
		reportLog.RetryCount++
	}

	reportLog.Status = status

	updateModel := &models.DailyReportLog{
		Status:          reportLog.Status,
		RetryCount:      reportLog.RetryCount,
		LastError:       dailyReportTrimLastError(lastError),
		UpdatedUnixTime: currentUnixTime,
	}

	cols := []string{"status", "retry_count", "last_error", "updated_unix_time"}

	if status == models.DAILY_REPORT_LOG_STATUS_SUCCESS {
		cols = append(cols, "sent_unix_time")
		updateModel.SentUnixTime = currentUnixTime
	}

	_, err := s.UserDataDB(uid).NewSession(c).Cols(cols...).ID(reportLog.ReportLogId).Update(updateModel)

	if err != nil {
		log.Errorf(c, "[daily_reports.updateDailyReportLogResult] failed to update daily report log \"id:%d\", because %s", reportLog.ReportLogId, err.Error())
	}
}

// getCurrencySummaries returns the income and expense amounts of every currency for the local
// date range [startTime, endTime) of the given timezone. The amounts are aggregated in memory
// from the transactions returned by the shared query, so the report works on every supported
// database without raw SQL
func (s *DailyReportService) getCurrencySummaries(c core.Context, uid int64, startTime time.Time, endTime time.Time, location *time.Location) ([]*dailyReportCurrencySummary, error) {
	startUnixTime := startTime.Unix()
	endUnixTime := endTime.Add(-1 * time.Nanosecond).Unix()

	allTransactions, err := s.transactions.getAllTransactionsInSpecifiedDateRange(c, uid, startUnixTime, endUnixTime, nil, nil, nil, location)

	if err != nil {
		return nil, err
	}

	allAccounts, err := s.accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	accountMap := s.accounts.GetAccountMapByList(allAccounts)
	startLocalDateTime := utils.FormatUnixTimeToNumericLocalDateTime(startUnixTime, location)
	endLocalDateTime := utils.FormatUnixTimeToNumericLocalDateTime(endUnixTime, location)

	// the shared query returns a superset of the requested local date range when transactions
	// were recorded with other timezones, so filter again by the exact reporting timezone in
	// the aggregation below
	return dailyReportAggregateCurrencySummaries(allTransactions, accountMap, startLocalDateTime, endLocalDateTime, location), nil
}

// dailyReportAggregateCurrencySummaries aggregates the income and expense amounts of every
// currency in memory. Currencies are never added together and the amounts are not exchanged,
// which mirrors the semantics of the previous external report script
func dailyReportAggregateCurrencySummaries(transactions []*models.Transaction, accountMap map[int64]*models.Account, startLocalDateTime int64, endLocalDateTime int64, location *time.Location) []*dailyReportCurrencySummary {
	incomeAmounts := make(map[string]int64)
	expenseAmounts := make(map[string]int64)
	incomeCounts := make(map[string]int64)
	expenseCounts := make(map[string]int64)
	currencySet := make(map[string]bool)

	for i := 0; i < len(transactions); i++ {
		transaction := transactions[i]

		localDateTime := utils.FormatUnixTimeToNumericLocalDateTime(utils.GetUnixTimeFromTransactionTime(transaction.TransactionTime), location)

		if localDateTime < startLocalDateTime || localDateTime > endLocalDateTime {
			continue
		}

		currency := dailyReportUnknownCurrency

		if account, exists := accountMap[transaction.AccountId]; exists && account.Currency != "" {
			currency = account.Currency
		}

		currencySet[currency] = true

		if transaction.Type == models.TRANSACTION_DB_TYPE_INCOME {
			incomeAmounts[currency] += transaction.Amount
			incomeCounts[currency]++
		} else if transaction.Type == models.TRANSACTION_DB_TYPE_EXPENSE {
			expenseAmounts[currency] += transaction.Amount
			expenseCounts[currency]++
		}
	}

	currencies := make([]string, 0, len(currencySet))

	for currency := range currencySet {
		currencies = append(currencies, currency)
	}

	sort.Strings(currencies)

	summaries := make([]*dailyReportCurrencySummary, 0, len(currencies))

	for i := 0; i < len(currencies); i++ {
		currency := currencies[i]
		summaries = append(summaries, &dailyReportCurrencySummary{
			Currency:      currency,
			IncomeAmount:  incomeAmounts[currency],
			ExpenseAmount: expenseAmounts[currency],
			IncomeCount:   incomeCounts[currency],
			ExpenseCount:  expenseCounts[currency],
		})
	}

	return summaries
}

// buildDailyReportSuccessCard builds the feishu interactive card of the daily accounting
// report, which mirrors the card layout of the previous external report script: yesterday's
// expense, yesterday's income and the month-to-date statistics of every currency
func (s *DailyReportService) buildDailyReportSuccessCard(reportDate time.Time, dailySummaries []*dailyReportCurrencySummary, monthlySummaries []*dailyReportCurrencySummary) map[string]any {
	subtitle := fmt.Sprintf("%d月%d日(%s)日报", int(reportDate.Month()), reportDate.Day(), dailyReportWeekdayNames[(int(reportDate.Weekday())+6)%7])
	monthLabel := fmt.Sprintf("%d月", int(reportDate.Month()))

	dailyByCurrency := make(map[string]*dailyReportCurrencySummary)

	for i := 0; i < len(dailySummaries); i++ {
		dailyByCurrency[dailySummaries[i].Currency] = dailySummaries[i]
	}

	monthlyByCurrency := make(map[string]*dailyReportCurrencySummary)

	for i := 0; i < len(monthlySummaries); i++ {
		monthlyByCurrency[monthlySummaries[i].Currency] = monthlySummaries[i]
	}

	currencies := make([]string, 0, len(dailySummaries)+len(monthlySummaries))

	for i := 0; i < len(dailySummaries); i++ {
		currencies = append(currencies, dailySummaries[i].Currency)
	}

	for i := 0; i < len(monthlySummaries); i++ {
		currency := monthlySummaries[i].Currency

		if _, exists := dailyByCurrency[currency]; !exists {
			currencies = append(currencies, currency)
		}
	}

	if len(currencies) < 1 {
		currencies = []string{dailyReportDefaultCurrencyWhenEmpty}
	}

	expenseLines := make([]string, 0, len(currencies))
	incomeLines := make([]string, 0, len(currencies))
	monthLines := make([]string, 0, len(currencies))

	for i := 0; i < len(currencies); i++ {
		currency := currencies[i]

		dailySummary, dailyExists := dailyByCurrency[currency]
		monthlySummary, monthlyExists := monthlyByCurrency[currency]

		dailyExpenseCents, dailyExpenseCount := int64(0), int64(0)
		dailyIncomeCents := int64(0)

		if dailyExists {
			dailyExpenseCents = dailySummary.ExpenseAmount
			dailyExpenseCount = dailySummary.ExpenseCount
			dailyIncomeCents = dailySummary.IncomeAmount
		}

		monthlyExpenseCents, monthlyIncomeCents := int64(0), int64(0)

		if monthlyExists {
			monthlyExpenseCents = monthlySummary.ExpenseAmount
			monthlyIncomeCents = monthlySummary.IncomeAmount
		}

		expenseLines = append(expenseLines, fmt.Sprintf("%s，共%d笔",
			dailyReportFormatCardAmount(currency, dailyExpenseCents), dailyExpenseCount))
		incomeLines = append(incomeLines, dailyReportFormatCardAmount(currency, dailyIncomeCents))
		monthLines = append(monthLines, fmt.Sprintf("%s已支出<font color='green'>%s</font>，已入账<font color='red'>%s</font>",
			monthLabel,
			dailyReportFormatCardAmount(currency, monthlyExpenseCents),
			dailyReportFormatCardAmount(currency, monthlyIncomeCents)))
	}

	elements := []any{
		map[string]any{"tag": "hr"},
		dailyReportCardRow("昨日支出", strings.Join(expenseLines, "\n"), "green"),
		dailyReportCardRow("昨日入账", strings.Join(incomeLines, "\n"), "red"),
		dailyReportCardRow("本月统计", strings.Join(monthLines, "\n"), ""),
	}

	return map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": "记账日报"},
			"subtitle": map[string]any{"tag": "plain_text", "content": subtitle},
			"template": "wathet",
		},
		"elements": elements,
	}
}

func dailyReportCardRow(label string, value string, valueColor string) map[string]any {
	valueMd := value

	if valueColor != "" {
		valueMd = fmt.Sprintf("<font color='%s'>%s</font>", valueColor, value)
	}

	labelMd := fmt.Sprintf("<font color='grey'>%s</font>", label)

	return map[string]any{
		"tag":              "column_set",
		"flex_mode":        "none",
		"background_style": "default",
		"columns": []any{
			map[string]any{
				"tag":            "column",
				"width":          "weighted",
				"weight":         1,
				"vertical_align": "center",
				"elements": []any{
					map[string]any{
						"tag":  "div",
						"text": map[string]any{"tag": "lark_md", "content": labelMd},
					},
				},
			},
			map[string]any{
				"tag":            "column",
				"width":          "weighted",
				"weight":         2,
				"vertical_align": "center",
				"elements": []any{
					map[string]any{
						"tag":  "div",
						"text": map[string]any{"tag": "lark_md", "content": valueMd},
					},
				},
			},
		},
	}
}

// dailyReportCurrencySymbol returns a familiar symbol for common currencies, retaining the
// currency code for unknown ones
func dailyReportCurrencySymbol(currency string) string {
	code := strings.ToUpper(strings.TrimSpace(currency))

	if code == "" {
		code = dailyReportUnknownCurrency
	}

	if symbol, exists := dailyReportCurrencySymbols[code]; exists {
		return symbol
	}

	return code + " "
}

func dailyReportFormatCardAmount(currency string, cents int64) string {
	return dailyReportCurrencySymbol(currency) + dailyReportFormatAmountValue(cents)
}

// dailyReportFormatAmountValue formats a confirmed cent-based integer amount without float
// rounding, e.g. -12550 is formatted as "-125.50"
func dailyReportFormatAmountValue(cents int64) string {
	sign := ""

	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// dailyReportLocation returns the location used to calculate the reporting date. The timezone
// name is preferred, and the configured utc offset is only used as a fallback when the
// timezone database is not available on the current server, which mirrors the behavior of the
// credit card notification timezone
func dailyReportLocation(config *settings.Config) *time.Location {
	if config.DailyReportTimezoneName != "" {
		location, err := time.LoadLocation(config.DailyReportTimezoneName)

		if err == nil {
			return location
		}
	}

	return time.FixedZone("Daily Report Timezone", int(config.DailyReportUtcOffsetMinutes)*60)
}

func dailyReportParseUids(uidsText string) []int64 {
	uidsText = strings.TrimSpace(uidsText)

	if uidsText == "" {
		return nil
	}

	uidTexts := strings.Split(uidsText, ",")
	uids := make([]int64, 0, len(uidTexts))

	for i := 0; i < len(uidTexts); i++ {
		uidText := strings.TrimSpace(uidTexts[i])

		if uidText == "" {
			continue
		}

		uid, err := utils.StringToInt64(uidText)

		if err != nil || uid <= 0 {
			continue
		}

		uids = append(uids, uid)
	}

	return uids
}

func (s *DailyReportService) createFeishuClientForUser(userConfig *dailyReportUserConfig, config *settings.Config) *feishu.FeishuClient {
	httpClient := httpclient.NewHttpClient(config.DailyReportRequestTimeout, config.DailyReportProxy, false, core.GetOutgoingUserAgent(), false, []uint16{})

	return feishu.NewFeishuClient(userConfig.webhookUrl, userConfig.signingSecret, userConfig.keyword, httpClient)
}

func dailyReportTrimLastError(lastError string) string {
	lastErrorRunes := []rune(lastError)

	if len(lastErrorRunes) > dailyReportMaxLastErrorLength {
		return string(lastErrorRunes[:dailyReportMaxLastErrorLength])
	}

	return lastError
}
