package models

// CreditCardRepaymentStatusSource represents how a credit card billing cycle was marked as repaid
type CreditCardRepaymentStatusSource byte

// Credit card repayment status sources
const (
	CREDIT_CARD_REPAYMENT_STATUS_SOURCE_AUTO   CreditCardRepaymentStatusSource = 1
	CREDIT_CARD_REPAYMENT_STATUS_SOURCE_MANUAL CreditCardRepaymentStatusSource = 2
)

// CreditCardRepaymentStatus represents a billing cycle of a credit card account which has been
// fully repaid. One row stands for one repayment date of one account, so the advance reminder and
// the due reminder of the same cycle are suppressed together. A manual row is never changed by
// the automatic detection, an automatic row is removed again when the repayment transaction is
// edited or deleted.
type CreditCardRepaymentStatus struct {
	StatusId        int64                           `xorm:"PK"`
	Uid             int64                           `xorm:"UNIQUE(UQE_credit_card_repayment_uid_account_date) NOT NULL"`
	Deleted         bool                            `xorm:"NOT NULL"`
	AccountId       int64                           `xorm:"INDEX(IDX_credit_card_repayment_uid_deleted_account) UNIQUE(UQE_credit_card_repayment_uid_account_date) NOT NULL"`
	ScheduledDate   int32                           `xorm:"UNIQUE(UQE_credit_card_repayment_uid_account_date) NOT NULL"`
	StatementDate   int32                           `xorm:"NOT NULL"`
	Source          CreditCardRepaymentStatusSource `xorm:"NOT NULL"`
	PaidUnixTime    int64                           `xorm:"NOT NULL"`
	CreatedUnixTime int64                           `xorm:"NOT NULL"`
	UpdatedUnixTime int64                           `xorm:"NOT NULL"`
	DeletedUnixTime int64                           `xorm:"NOT NULL"`
}

// CreditCardRepaymentStatusRequest represents all parameters of a credit card repayment status
// operation request. ScheduledDate is the repayment date of the billing cycle and is optional so
// the current cycle can be used when it is omitted.
type CreditCardRepaymentStatusRequest struct {
	Id            int64 `json:"id,string" binding:"required,min=1"`
	ScheduledDate int32 `json:"scheduledDate" binding:"omitempty,min=19000101,max=29991231"`
}

// CreditCardRepaymentStatusGetRequest represents all parameters of getting a credit card
// repayment status. ScheduledDate is optional so the current cycle can be used when omitted.
type CreditCardRepaymentStatusGetRequest struct {
	Id            int64 `form:"id,string" binding:"required,min=1"`
	ScheduledDate int32 `form:"scheduledDate" binding:"omitempty,min=19000101,max=29991231"`
}

// RepaymentStatusInfo is a short alias for the repayment status embedded in account responses
type RepaymentStatusInfo = CreditCardRepaymentStatusResponse

// CreditCardRepaymentStatusResponse represents the repayment status of one credit card cycle
type CreditCardRepaymentStatusResponse struct {
	ScheduledDate int32                           `json:"scheduledDate"`
	StatementDate int32                           `json:"statementDate"`
	Paid          bool                            `json:"paid"`
	Source        CreditCardRepaymentStatusSource `json:"source"`
}
