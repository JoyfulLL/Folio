package models

// UserDailyReportSetting represents the per-user daily report (feishu webhook) settings, stored
// in database. The webhook url and the signing secret are stored encrypted with AES-256-GCM
// using a key derived from the server-side secret key in the configuration file, which lives
// outside the database, so a database-only compromise cannot recover the plaintext secrets
type UserDailyReportSetting struct {
	SettingId            int64  `xorm:"PK"`
	Uid                  int64  `xorm:"UNIQUE NOT NULL"`
	Enabled              bool   `xorm:"NOT NULL"`
	WebhookUrl           string `xorm:"TEXT NOT NULL"` // stored encrypted
	SigningSecret        string `xorm:"TEXT NOT NULL"` // stored encrypted
	Keyword              string `xorm:"VARCHAR(64) NOT NULL"`
	SendTimeMinutesOfDay int    `xorm:"NOT NULL"`             // 0 means falling back to the server default
	TimezoneName         string `xorm:"VARCHAR(64) NOT NULL"` // empty means falling back to the server default
	CreatedUnixTime      int64
	UpdatedUnixTime      int64
}

// UserDailyReportSettingUpdateRequest represents the request of updating the daily report
// settings of the current user. Leaving the webhook url or the signing secret empty keeps the
// saved value unchanged, because the saved values are never returned to the client
type UserDailyReportSettingUpdateRequest struct {
	Enabled              bool   `json:"enabled"`
	WebhookUrl           string `json:"webhookUrl" binding:"max=512"`
	SigningSecret        string `json:"signingSecret" binding:"max=128"`
	Keyword              string `json:"keyword" binding:"max=32"`
	SendTimeMinutesOfDay int    `json:"sendTimeMinutesOfDay" binding:"min=0,max=1439"`
	TimezoneName         string `json:"timezoneName" binding:"max=64"`
}

// UserDailyReportSettingGetResponse represents the response of getting the daily report
// settings of the current user. The saved webhook url and signing secret are never returned in
// plaintext, only whether they have been set and a masked preview of the url
type UserDailyReportSettingGetResponse struct {
	Enabled              bool   `json:"enabled"`
	WebhookUrlSet        bool   `json:"webhookUrlSet"`
	WebhookUrlMasked     string `json:"webhookUrlMasked"`
	SigningSecretSet     bool   `json:"signingSecretSet"`
	Keyword              string `json:"keyword"`
	SendTimeMinutesOfDay int    `json:"sendTimeMinutesOfDay"`
	TimezoneName         string `json:"timezoneName"`
}

// UserDailyReportSettingTestSendResponse represents the response of sending a test message to
// the feishu webhook of the current user
type UserDailyReportSettingTestSendResponse struct {
	Result string `json:"result"`
}
