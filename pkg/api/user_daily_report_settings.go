package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin/binding"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// UserDailyReportSettingsApi represents the api which manages the daily report (feishu webhook)
// settings of the current user
type UserDailyReportSettingsApi struct {
	ApiUsingConfig
	userDailyReportSettings *services.UserDailyReportSettingsService
	dailyReports            *services.DailyReportService
}

// Initialize a user daily report settings api singleton instance
var (
	UserDailyReportSettings = &UserDailyReportSettingsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		userDailyReportSettings: services.UserDailyReportSettings,
		dailyReports:            services.DailyReports,
	}
)

// DailyReportSettingsGetHandler returns the daily report settings of the current user. The
// saved webhook url and signing secret are never returned in plaintext, only whether they have
// been set and a masked preview of the url
func (a *UserDailyReportSettingsApi) DailyReportSettingsGetHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()

	setting, err := a.userDailyReportSettings.GetSettingByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[user_daily_report_settings.DailyReportSettingsGetHandler] failed to get the daily report settings of user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.ErrSystemError
	}

	response := &models.UserDailyReportSettingGetResponse{
		Enabled:              false,
		WebhookUrlSet:        false,
		WebhookUrlMasked:     "",
		SigningSecretSet:     false,
		Keyword:              "",
		SendTimeMinutesOfDay: a.CurrentConfig().DailyReportSendTimeMinutesOfDay,
		TimezoneName:         "",
	}

	if setting != nil {
		response.Enabled = setting.Enabled
		response.WebhookUrlSet = a.userDailyReportSettings.HasWebhookUrl(setting)
		response.SigningSecretSet = a.userDailyReportSettings.HasSigningSecret(setting)
		response.Keyword = setting.Keyword
		response.SendTimeMinutesOfDay = setting.SendTimeMinutesOfDay

		if setting.SendTimeMinutesOfDay <= 0 {
			response.SendTimeMinutesOfDay = a.CurrentConfig().DailyReportSendTimeMinutesOfDay
		}

		response.TimezoneName = setting.TimezoneName

		if response.WebhookUrlSet {
			response.WebhookUrlMasked = maskUserDailyReportWebhookUrl(c, a.userDailyReportSettings, setting)
		}
	}

	return response, nil
}

// DailyReportSettingsUpdateHandler saves the daily report settings of the current user. The
// webhook url and the signing secret are encrypted with a key derived from the server secret
// key before being stored, and leaving them empty in the request keeps the saved values
// unchanged, because the saved values are never returned to the client
func (a *UserDailyReportSettingsApi) DailyReportSettingsUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var updateReq models.UserDailyReportSettingUpdateRequest
	err := c.ShouldBindBodyWith(&updateReq, binding.JSON)

	if err != nil {
		log.Warnf(c, "[user_daily_report_settings.DailyReportSettingsUpdateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	updateReq.WebhookUrl = strings.TrimSpace(updateReq.WebhookUrl)
	updateReq.Keyword = strings.TrimSpace(updateReq.Keyword)
	updateReq.TimezoneName = strings.TrimSpace(updateReq.TimezoneName)

	if updateReq.TimezoneName != "" {
		if _, err := time.LoadLocation(updateReq.TimezoneName); err != nil {
			return nil, errs.ErrDailyReportTimezoneNameInvalid
		}
	}

	uid := c.GetCurrentUid()

	setting, err := a.userDailyReportSettings.SaveSetting(c, uid, &updateReq)

	if err != nil {
		log.Errorf(c, "[user_daily_report_settings.DailyReportSettingsUpdateHandler] failed to save the daily report settings of user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrSystemError)
	}

	log.Infof(c, "[user_daily_report_settings.DailyReportSettingsUpdateHandler] the daily report settings of user \"uid:%d\" have been saved", uid)

	response := &models.UserDailyReportSettingGetResponse{
		Enabled:              setting.Enabled,
		WebhookUrlSet:        a.userDailyReportSettings.HasWebhookUrl(setting),
		SigningSecretSet:     a.userDailyReportSettings.HasSigningSecret(setting),
		Keyword:              setting.Keyword,
		SendTimeMinutesOfDay: setting.SendTimeMinutesOfDay,
		TimezoneName:         setting.TimezoneName,
	}

	if response.SendTimeMinutesOfDay <= 0 {
		response.SendTimeMinutesOfDay = a.CurrentConfig().DailyReportSendTimeMinutesOfDay
	}

	return response, nil
}

// DailyReportSettingsTestSendHandler sends a test card to the feishu webhook of the current
// user immediately
func (a *UserDailyReportSettingsApi) DailyReportSettingsTestSendHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()

	err := a.dailyReports.SendTestReport(c, uid)

	if err != nil {
		log.Errorf(c, "[user_daily_report_settings.DailyReportSettingsTestSendHandler] failed to send the test message of user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrSystemError)
	}

	return &models.UserDailyReportSettingTestSendResponse{
		Result: "success",
	}, nil
}

// maskUserDailyReportWebhookUrl returns a masked preview of the saved webhook url, keeping only
// the part before the secret token
func maskUserDailyReportWebhookUrl(c core.Context, service *services.UserDailyReportSettingsService, setting *models.UserDailyReportSetting) string {
	url, err := service.DecryptWebhookUrl(c, setting)

	if err != nil {
		// the saved secrets cannot be decrypted (most likely the server secret key has been
		// changed), so there is no preview to show
		return ""
	}

	return dailyReportMaskWebhookUrl(url)
}

// dailyReportMaskWebhookUrl returns a masked preview of the webhook url, keeping only the part
// before the secret token
func dailyReportMaskWebhookUrl(url string) string {
	lastSlashIndex := strings.LastIndex(url, "/")

	if lastSlashIndex < 0 || len(url)-lastSlashIndex <= 5 {
		return "****"
	}

	return url[:lastSlashIndex+1] + "****" + url[len(url)-4:]
}
