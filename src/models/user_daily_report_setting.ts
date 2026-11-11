// UserDailyReportSettingGetResponse represents the daily report settings of the current user.
// The saved webhook url and signing secret are never returned in plaintext, only whether they
// have been set and a masked preview of the url
export interface UserDailyReportSettingGetResponse {
    readonly enabled: boolean;
    readonly webhookUrlSet: boolean;
    readonly webhookUrlMasked: string;
    readonly signingSecretSet: boolean;
    readonly keyword: string;
    readonly sendTimeMinutesOfDay: number;
    readonly timezoneName: string;
}

// UserDailyReportSettingUpdateRequest represents the request of updating the daily report
// settings of the current user. Leaving the webhook url or the signing secret empty keeps the
// saved value unchanged
export interface UserDailyReportSettingUpdateRequest {
    readonly enabled: boolean;
    readonly webhookUrl: string;
    readonly signingSecret: string;
    readonly keyword: string;
    readonly sendTimeMinutesOfDay: number;
    readonly timezoneName: string;
}

// UserDailyReportSettingTestSendResponse represents the response of sending a test message to
// the feishu webhook of the current user
export interface UserDailyReportSettingTestSendResponse {
    readonly result: string;
}
