package models

// CreditCardNotificationType represents the type of credit card notification
type CreditCardNotificationType byte

// Credit card notification types
const (
	CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER CreditCardNotificationType = 1
	CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL    CreditCardNotificationType = 2
	CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE  CreditCardNotificationType = 3
	CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE      CreditCardNotificationType = 4
)

// String returns a textual representation of the credit card notification types
func (t CreditCardNotificationType) String() string {
	switch t {
	case CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER:
		return "Statement Reminder"
	case CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL:
		return "Statement Email"
	case CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE:
		return "Repayment Advance Reminder"
	case CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE:
		return "Repayment Due Reminder"
	default:
		return "Invalid"
	}
}

// CreditCardNotificationStatus represents the sending status of a credit card notification
type CreditCardNotificationStatus byte

// Credit card notification statuses
const (
	CREDIT_CARD_NOTIFICATION_STATUS_PENDING CreditCardNotificationStatus = 0
	CREDIT_CARD_NOTIFICATION_STATUS_SUCCESS CreditCardNotificationStatus = 1
	CREDIT_CARD_NOTIFICATION_STATUS_FAILED  CreditCardNotificationStatus = 2

	// CREDIT_CARD_NOTIFICATION_STATUS_CANCELLED means the notification is not sent because it does
	// not apply any more, for example the notification is turned off after it has been planned
	CREDIT_CARD_NOTIFICATION_STATUS_CANCELLED CreditCardNotificationStatus = 3
)

// CREDIT_CARD_NOTIFICATION_DEFAULT_INTERVAL is the account scan interval in minutes used when the
// user has not chosen one. The cron job ticks every 15 minutes, so it is also the shortest value
const CREDIT_CARD_NOTIFICATION_DEFAULT_INTERVAL int16 = 15

// IsValidCreditCardNotificationInterval returns whether the account scan interval in minutes is one
// of the values the user can choose
func IsValidCreditCardNotificationInterval(interval int16) bool {
	switch interval {
	case 15, 30, 60, 180, 360, 720:
		return true
	default:
		return false
	}
}

// GetValidCreditCardNotificationInterval returns the account scan interval in minutes, falling back
// to the default for users whose interval has never been saved
func GetValidCreditCardNotificationInterval(interval int16) int16 {
	if IsValidCreditCardNotificationInterval(interval) {
		return interval
	}

	return CREDIT_CARD_NOTIFICATION_DEFAULT_INTERVAL
}

// CreditCardNotificationLog represents a credit card notification which should be sent, stored in
// database. One row stands for one notification of one account on one day, so the same account
// will never receive the same notification twice on the same day
type CreditCardNotificationLog struct {
	NotificationLogId int64                        `xorm:"PK"`
	Uid               int64                        `xorm:"INDEX(IDX_credit_card_notification_uid_deleted_account_id_type_date) UNIQUE(UQE_credit_card_notification_uid_account_id_type_date) NOT NULL"`
	Deleted           bool                         `xorm:"INDEX(IDX_credit_card_notification_uid_deleted_account_id_type_date) NOT NULL"`
	AccountId         int64                        `xorm:"INDEX(IDX_credit_card_notification_uid_deleted_account_id_type_date) INDEX(IDX_credit_card_notification_status_send_time) UNIQUE(UQE_credit_card_notification_uid_account_id_type_date) NOT NULL"`
	NotificationType  CreditCardNotificationType   `xorm:"INDEX(IDX_credit_card_notification_uid_deleted_account_id_type_date) UNIQUE(UQE_credit_card_notification_uid_account_id_type_date) NOT NULL"`
	ScheduledDate     int32                        `xorm:"INDEX(IDX_credit_card_notification_uid_deleted_account_id_type_date) UNIQUE(UQE_credit_card_notification_uid_account_id_type_date) NOT NULL"`
	SendUnixTime      int64                        `xorm:"INDEX(IDX_credit_card_notification_status_send_time) NOT NULL"`
	Status            CreditCardNotificationStatus `xorm:"INDEX(IDX_credit_card_notification_status_send_time) NOT NULL"`
	RetryCount        int16                        `xorm:"NOT NULL"`
	LastError         string                       `xorm:"VARCHAR(255) NOT NULL"`
	SentUnixTime      int64
	CreatedUnixTime   int64
	UpdatedUnixTime   int64
	DeletedUnixTime   int64
}
