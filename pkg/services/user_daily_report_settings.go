package services

import (
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// dailyReportSettingEncryptionDomain separates the derived encryption key of the daily report
// webhook settings from other purposes sharing the same server secret key. Bumping the version
// invalidates every stored ciphertext, so it must only be changed together with a migration
const dailyReportSettingEncryptionDomain = "ezbookkeeping:user-daily-report-setting:v1"

const dailyReportSettingWebhookUrlAdditionalData = "user-daily-report-setting:webhook-url"
const dailyReportSettingSigningSecretAdditionalData = "user-daily-report-setting:signing-secret"

// UserDailyReportSettingsService represents the service which manages the per-user daily report
// (feishu webhook) settings
type UserDailyReportSettingsService struct {
	ServiceUsingDB
	ServiceUsingConfig
	ServiceUsingUuid
}

// Initialize a user daily report settings service singleton instance
var (
	UserDailyReportSettings = &UserDailyReportSettingsService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetSettingByUid returns the daily report settings of the given user, or nil when the user has
// never saved any settings
func (s *UserDailyReportSettingsService) GetSettingByUid(c core.Context, uid int64) (*models.UserDailyReportSetting, error) {
	setting := &models.UserDailyReportSetting{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Get(setting)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, nil
	}

	return setting, nil
}

// GetEnabledSettings returns the enabled daily report settings of every user across all data
// stores, so the cron job can iterate per-user settings on sharded deployments
func (s *UserDailyReportSettingsService) GetEnabledSettings(c core.Context) ([]*models.UserDailyReportSetting, error) {
	var allSettings []*models.UserDailyReportSetting

	for i := 0; i < s.UserDataDBCount(); i++ {
		db := s.UserDataDBByIndex(i)
		var settingsOfDataStore []*models.UserDailyReportSetting

		if err := db.NewSession(c).Where("enabled=?", true).Find(&settingsOfDataStore); err != nil {
			return nil, err
		}

		allSettings = append(allSettings, settingsOfDataStore...)
	}

	return allSettings, nil
}

// SaveSetting saves the daily report settings of the given user. The webhook url and the
// signing secret are encrypted before being stored, and leaving them empty in the request keeps
// the saved values unchanged, because the saved values are never returned to the client
func (s *UserDailyReportSettingsService) SaveSetting(c core.Context, uid int64, req *models.UserDailyReportSettingUpdateRequest) (*models.UserDailyReportSetting, error) {
	key, err := s.encryptionKey()

	if err != nil {
		return nil, err
	}

	existing, err := s.GetSettingByUid(c, uid)

	if err != nil {
		return nil, err
	}

	newWebhookUrl, err := s.encryptField(key, dailyReportSettingWebhookUrlAdditionalData, req.WebhookUrl, getWebhookUrlCiphertext(existing))

	if err != nil {
		return nil, err
	}

	newSigningSecret, err := s.encryptField(key, dailyReportSettingSigningSecretAdditionalData, req.SigningSecret, getSigningSecretCiphertext(existing))

	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()

	if existing == nil {
		if req.Enabled && req.WebhookUrl == "" {
			return nil, errs.ErrDailyReportWebhookUrlIsEmpty
		}

		setting := &models.UserDailyReportSetting{
			SettingId:            s.GenerateUuid(uuid.UUID_TYPE_DAILY_REPORT_LOG),
			Uid:                  uid,
			Enabled:              req.Enabled,
			WebhookUrl:           newWebhookUrl,
			SigningSecret:        newSigningSecret,
			Keyword:              strings.TrimSpace(req.Keyword),
			SendTimeMinutesOfDay: req.SendTimeMinutesOfDay,
			TimezoneName:         strings.TrimSpace(req.TimezoneName),
			CreatedUnixTime:      now,
			UpdatedUnixTime:      now,
		}

		if _, err := s.UserDataDB(uid).NewSession(c).Insert(setting); err != nil {
			return nil, err
		}

		return setting, nil
	}

	if req.Enabled && getWebhookUrlCiphertext(existing) == "" && req.WebhookUrl == "" {
		return nil, errs.ErrDailyReportWebhookUrlIsEmpty
	}

	existing.Enabled = req.Enabled
	existing.WebhookUrl = newWebhookUrl
	existing.SigningSecret = newSigningSecret
	existing.Keyword = strings.TrimSpace(req.Keyword)
	existing.SendTimeMinutesOfDay = req.SendTimeMinutesOfDay
	existing.TimezoneName = strings.TrimSpace(req.TimezoneName)
	existing.UpdatedUnixTime = now

	_, err = s.UserDataDB(uid).NewSession(c).Cols("enabled", "webhook_url", "signing_secret", "keyword",
		"send_time_minutes_of_day", "timezone_name", "updated_unix_time").ID(existing.SettingId).Update(existing)

	if err != nil {
		return nil, err
	}

	return existing, nil
}

// DecryptWebhookUrl decrypts the stored webhook url for use, e.g. sending the report
func (s *UserDailyReportSettingsService) DecryptWebhookUrl(c core.Context, setting *models.UserDailyReportSetting) (string, error) {
	key, err := s.encryptionKey()

	if err != nil {
		return "", err
	}

	return utils.DecryptStringWithAESGCM(key, dailyReportSettingWebhookUrlAdditionalData, getWebhookUrlCiphertext(setting))
}

// DecryptSigningSecret decrypts the stored signing secret for use, e.g. signing the report
func (s *UserDailyReportSettingsService) DecryptSigningSecret(c core.Context, setting *models.UserDailyReportSetting) (string, error) {
	key, err := s.encryptionKey()

	if err != nil {
		return "", err
	}

	return utils.DecryptStringWithAESGCM(key, dailyReportSettingSigningSecretAdditionalData, getSigningSecretCiphertext(setting))
}

// HasWebhookUrl returns whether the given setting has a saved (encrypted) webhook url
func (s *UserDailyReportSettingsService) HasWebhookUrl(setting *models.UserDailyReportSetting) bool {
	return getWebhookUrlCiphertext(setting) != ""
}

// HasSigningSecret returns whether the given setting has a saved (encrypted) signing secret
func (s *UserDailyReportSettingsService) HasSigningSecret(setting *models.UserDailyReportSetting) bool {
	return getSigningSecretCiphertext(setting) != ""
}

// encryptionKey derives the encryption key from the server secret key. It fails when the
// server secret key has not been set, because encrypting with the well-known default secret
// would give no protection at all
func (s *UserDailyReportSettingsService) encryptionKey() ([]byte, error) {
	config := s.CurrentConfig()

	if config.SecretKeyNoSet {
		log.Errorf(nil, "[user_daily_report_settings.encryptionKey] the server secret key is not set, refusing to use the webhook settings encryption")
		return nil, errs.ErrDailyReportSecretKeyNotSet
	}

	return utils.DeriveStorageEncryptionKey(config.SecretKey, dailyReportSettingEncryptionDomain), nil
}

// encryptField encrypts the given plaintext, keeping the saved ciphertext unchanged when the
// plaintext is empty (which means the client did not send a new value)
func (s *UserDailyReportSettingsService) encryptField(key []byte, additionalData string, plaintext string, savedCiphertext string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)

	if trimmed == "" {
		return savedCiphertext, nil
	}

	return utils.EncryptStringWithAESGCM(key, additionalData, trimmed)
}

// getWebhookUrlCiphertext and getSigningSecretCiphertext return the stored ciphertexts of a
// possibly nil setting, so both the create and the update paths share the same logic
func getWebhookUrlCiphertext(setting *models.UserDailyReportSetting) string {
	if setting == nil {
		return ""
	}

	return setting.WebhookUrl
}

func getSigningSecretCiphertext(setting *models.UserDailyReportSetting) string {
	if setting == nil {
		return ""
	}

	return setting.SigningSecret
}
