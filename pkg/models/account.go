package models

import (
	"encoding/json/v2"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// LevelOneAccountParentId represents the parent id of level-one account
const LevelOneAccountParentId = 0

// AccountCategory represents account category
type AccountCategory byte

// Account categories
const (
	ACCOUNT_CATEGORY_CASH                   AccountCategory = 1
	ACCOUNT_CATEGORY_CHECKING_ACCOUNT       AccountCategory = 2
	ACCOUNT_CATEGORY_CREDIT_CARD            AccountCategory = 3
	ACCOUNT_CATEGORY_VIRTUAL                AccountCategory = 4
	ACCOUNT_CATEGORY_DEBT                   AccountCategory = 5
	ACCOUNT_CATEGORY_RECEIVABLES            AccountCategory = 6
	ACCOUNT_CATEGORY_INVESTMENT             AccountCategory = 7
	ACCOUNT_CATEGORY_SAVINGS_ACCOUNT        AccountCategory = 8
	ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT AccountCategory = 9
)

var assetAccountCategory = map[AccountCategory]bool{
	ACCOUNT_CATEGORY_CASH:                   true,
	ACCOUNT_CATEGORY_CHECKING_ACCOUNT:       true,
	ACCOUNT_CATEGORY_CREDIT_CARD:            false,
	ACCOUNT_CATEGORY_VIRTUAL:                true,
	ACCOUNT_CATEGORY_DEBT:                   false,
	ACCOUNT_CATEGORY_RECEIVABLES:            true,
	ACCOUNT_CATEGORY_INVESTMENT:             true,
	ACCOUNT_CATEGORY_SAVINGS_ACCOUNT:        true,
	ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT: true,
}

var liabilityAccountCategory = map[AccountCategory]bool{
	ACCOUNT_CATEGORY_CASH:                   false,
	ACCOUNT_CATEGORY_CHECKING_ACCOUNT:       false,
	ACCOUNT_CATEGORY_CREDIT_CARD:            true,
	ACCOUNT_CATEGORY_VIRTUAL:                false,
	ACCOUNT_CATEGORY_DEBT:                   true,
	ACCOUNT_CATEGORY_RECEIVABLES:            false,
	ACCOUNT_CATEGORY_INVESTMENT:             false,
	ACCOUNT_CATEGORY_SAVINGS_ACCOUNT:        false,
	ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT: false,
}

// IsAsset returns whether the account category is an asset category
func (c AccountCategory) IsAsset() bool {
	return assetAccountCategory[c]
}

// IsLiability returns whether the account category is a liability category
func (c AccountCategory) IsLiability() bool {
	return liabilityAccountCategory[c]
}

// AccountType represents account type
type AccountType byte

// Account types
const (
	ACCOUNT_TYPE_SINGLE_ACCOUNT     AccountType = 1
	ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS AccountType = 2
)

var defaultCreditCardAccountStatementDate = 0
var defaultCreditCardAccountLimit = "0"

// Account represents account data stored in database
type Account struct {
	AccountId       int64           `xorm:"PK"`
	Uid             int64           `xorm:"INDEX(IDX_account_uid_deleted_parent_account_id_order) NOT NULL"`
	Deleted         bool            `xorm:"INDEX(IDX_account_uid_deleted_parent_account_id_order) NOT NULL"`
	Category        AccountCategory `xorm:"NOT NULL"`
	Type            AccountType     `xorm:"NOT NULL"`
	ParentAccountId int64           `xorm:"INDEX(IDX_account_uid_deleted_parent_account_id_order) NOT NULL"`
	Name            string          `xorm:"VARCHAR(64) NOT NULL"`
	DisplayOrder    int32           `xorm:"INDEX(IDX_account_uid_deleted_parent_account_id_order) NOT NULL"`
	Icon            int64           `xorm:"NOT NULL"`
	IconType        core.IconType
	Color           string         `xorm:"VARCHAR(6) NOT NULL"`
	Currency        string         `xorm:"VARCHAR(3) NOT NULL"`
	Balance         int64          `xorm:"NOT NULL"`
	Comment         string         `xorm:"VARCHAR(255) NOT NULL"`
	Extend          *AccountExtend `xorm:"MEDIUMBLOB"`
	Hidden          bool           `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// AccountExtend represents account extend data stored in database
type AccountExtend struct {
	LastReconciledTime       *int64  `json:"lastReconciledTime,omitempty"`
	CreditCardStatementDate  *int    `json:"creditCardStatementDate,omitempty"`
	CreditCardLimit          *int64  `json:"creditCardLimit,string,omitempty"`
	CreditCardBankName       *string `json:"creditCardBankName,omitempty"`
	CreditCardCardName       *string `json:"creditCardCardName,omitempty"`
	CreditCardLastFourDigits *string `json:"creditCardLastFourDigits,omitempty"`
	CreditCardExpireMonth    *int    `json:"creditCardExpireMonth,omitempty"`
	CreditCardExpireYear     *int    `json:"creditCardExpireYear,omitempty"`
	CreditCardCardholderName *string `json:"creditCardCardholderName,omitempty"`
	// CreditCardPicture is the deprecated base64 picture data url, kept only for converting it into a picture file
	CreditCardPicture                         *string `json:"creditCardPicture,omitempty"`
	CreditCardPictureId                       *int64  `json:"creditCardPictureId,string,omitempty"`
	CreditCardStatementDateIncludeTransaction *bool   `json:"creditCardStatementDateIncludeTransaction,omitempty"`
	CreditCardRepaymentDateType               *int    `json:"creditCardRepaymentDateType,omitempty"`
	CreditCardRepaymentDaysAfterStatement     *int    `json:"creditCardRepaymentDaysAfterStatement,omitempty"`
	CreditCardRepaymentDayOfMonth             *int    `json:"creditCardRepaymentDayOfMonth,omitempty"`
	CreditCardStatementReminderEnabled        *bool   `json:"creditCardStatementReminderEnabled,omitempty"`
	CreditCardStatementEmailEnabled           *bool   `json:"creditCardStatementEmailEnabled,omitempty"`
	CreditCardRepaymentReminderEnabled        *bool   `json:"creditCardRepaymentReminderEnabled,omitempty"`
	CreditCardRepaymentReminderDaysBefore     *int    `json:"creditCardRepaymentReminderDaysBefore,omitempty"`
	CreditCardNotificationMinutesOfDay        *int    `json:"creditCardNotificationMinutesOfDay,omitempty"`
	CreditCardNotificationTimezoneName        *string `json:"creditCardNotificationTimezoneName,omitempty"`
	CreditCardNotificationTimezoneUtcOffset   *int16  `json:"creditCardNotificationTimezoneUtcOffset,omitempty"`
	// CreditCardIsEntityCard indicates whether this account itself represents a physical credit card.
	// A nil value means true for compatibility with accounts created before this field existed.
	// Only a level-one account can be set to false, which turns it into a pure credit limit pool.
	CreditCardIsEntityCard *bool `json:"creditCardIsEntityCard,omitempty"`
	// CreditCardCurrencyLimit is the shared pool limit stated by the bank in this card's own
	// currency (for example CNY 8600 / JPY 129000 on the same pool). The ratio of two stated
	// limits is the bank's own conversion rate, which the client uses to share the used limit
	// between all cards of the pool.
	CreditCardCurrencyLimit *string `json:"creditCardCurrencyLimit,omitempty"`
}

// AccountCreateRequest represents all parameters of account creation request
type AccountCreateRequest struct {
	Name                                      string                  `json:"name" binding:"required,notBlank,max=64"`
	Category                                  AccountCategory         `json:"category" binding:"required"`
	Type                                      AccountType             `json:"type" binding:"required"`
	Icon                                      int64                   `json:"icon,string" binding:"required,min=1"`
	IconType                                  core.IconType           `json:"iconType" binding:"min=0,max=1"`
	Color                                     string                  `json:"color" binding:"required,len=6,validHexRGBColor"`
	Currency                                  string                  `json:"currency" binding:"required,len=3,validCurrency"`
	Balance                                   string                  `json:"balance" binding:"validTransactionAmount"`
	BalanceTime                               int64                   `json:"balanceTime"`
	Comment                                   string                  `json:"comment" binding:"max=255"`
	CreditCardStatementDate                   int                     `json:"creditCardStatementDate" binding:"min=0,max=28"`
	CreditCardLimit                           string                  `json:"creditCardLimit" binding:"omitempty,validTransactionAmount"`
	CreditCardBankName                        string                  `json:"creditCardBankName" binding:"max=64"`
	CreditCardCardName                        string                  `json:"creditCardCardName" binding:"max=64"`
	CreditCardLastFourDigits                  string                  `json:"creditCardLastFourDigits" binding:"omitempty,len=4,numeric"`
	CreditCardExpireMonth                     int                     `json:"creditCardExpireMonth" binding:"min=0,max=12"`
	CreditCardExpireYear                      int                     `json:"creditCardExpireYear" binding:"min=0,max=9999"`
	CreditCardCardholderName                  string                  `json:"creditCardCardholderName" binding:"max=64"`
	CreditCardPictureId                       int64                   `json:"creditCardPictureId,string" binding:"min=0"`
	CreditCardStatementDateIncludeTransaction bool                    `json:"creditCardStatementDateIncludeTransaction"`
	CreditCardRepaymentDateType               int                     `json:"creditCardRepaymentDateType" binding:"min=0,max=2"`
	CreditCardRepaymentDaysAfterStatement     int                     `json:"creditCardRepaymentDaysAfterStatement" binding:"min=0,max=99"`
	CreditCardRepaymentDayOfMonth             int                     `json:"creditCardRepaymentDayOfMonth" binding:"min=0,max=28"`
	CreditCardStatementReminderEnabled        bool                    `json:"creditCardStatementReminderEnabled"`
	CreditCardStatementEmailEnabled           bool                    `json:"creditCardStatementEmailEnabled"`
	CreditCardRepaymentReminderEnabled        bool                    `json:"creditCardRepaymentReminderEnabled"`
	CreditCardRepaymentReminderDaysBefore     int                     `json:"creditCardRepaymentReminderDaysBefore" binding:"min=0,max=99"`
	CreditCardNotificationMinutesOfDay        *int                    `json:"creditCardNotificationMinutesOfDay" binding:"omitempty,min=0,max=1439"`
	CreditCardNotificationTimezoneName        *string                 `json:"creditCardNotificationTimezoneName" binding:"omitempty,min=3,max=100"`
	CreditCardNotificationTimezoneUtcOffset   *int16                  `json:"creditCardNotificationTimezoneUtcOffset" binding:"omitempty,min=-720,max=840"`
	CreditCardIsEntityCard                    *bool                   `json:"creditCardIsEntityCard"`
	CreditCardCurrencyLimit                   string                  `json:"creditCardCurrencyLimit" binding:"omitempty,max=64"`
	SubAccounts                               []*AccountCreateRequest `json:"subAccounts" binding:"omitempty"`
	ClientSessionId                           string                  `json:"clientSessionId"`
}

// AccountModifyRequest represents all parameters of account modification request
type AccountModifyRequest struct {
	Id                                        int64                   `json:"id,string" binding:"required,min=0"`
	Name                                      string                  `json:"name" binding:"required,notBlank,max=64"`
	Category                                  AccountCategory         `json:"category" binding:"required"`
	Icon                                      int64                   `json:"icon,string" binding:"min=1"`
	IconType                                  core.IconType           `json:"iconType" binding:"min=0,max=1"`
	Color                                     string                  `json:"color" binding:"required,len=6,validHexRGBColor"`
	Currency                                  *string                 `json:"currency" binding:"omitempty,len=3,validCurrency"`
	Balance                                   *string                 `json:"balance" binding:"omitempty,validTransactionAmount"`
	BalanceTime                               *int64                  `json:"balanceTime" binding:"omitempty"`
	LastReconciledTime                        *int64                  `json:"lastReconciledTime" binding:"omitempty"`
	Comment                                   string                  `json:"comment" binding:"max=255"`
	CreditCardStatementDate                   int                     `json:"creditCardStatementDate" binding:"min=0,max=28"`
	CreditCardLimit                           string                  `json:"creditCardLimit" binding:"omitempty,validTransactionAmount"`
	CreditCardBankName                        string                  `json:"creditCardBankName" binding:"max=64"`
	CreditCardCardName                        string                  `json:"creditCardCardName" binding:"max=64"`
	CreditCardLastFourDigits                  string                  `json:"creditCardLastFourDigits" binding:"omitempty,len=4,numeric"`
	CreditCardExpireMonth                     int                     `json:"creditCardExpireMonth" binding:"min=0,max=12"`
	CreditCardExpireYear                      int                     `json:"creditCardExpireYear" binding:"min=0,max=9999"`
	CreditCardCardholderName                  string                  `json:"creditCardCardholderName" binding:"max=64"`
	CreditCardPictureId                       int64                   `json:"creditCardPictureId,string" binding:"min=0"`
	CreditCardStatementDateIncludeTransaction bool                    `json:"creditCardStatementDateIncludeTransaction"`
	CreditCardRepaymentDateType               int                     `json:"creditCardRepaymentDateType" binding:"min=0,max=2"`
	CreditCardRepaymentDaysAfterStatement     int                     `json:"creditCardRepaymentDaysAfterStatement" binding:"min=0,max=99"`
	CreditCardRepaymentDayOfMonth             int                     `json:"creditCardRepaymentDayOfMonth" binding:"min=0,max=28"`
	CreditCardStatementReminderEnabled        bool                    `json:"creditCardStatementReminderEnabled"`
	CreditCardStatementEmailEnabled           bool                    `json:"creditCardStatementEmailEnabled"`
	CreditCardRepaymentReminderEnabled        bool                    `json:"creditCardRepaymentReminderEnabled"`
	CreditCardRepaymentReminderDaysBefore     int                     `json:"creditCardRepaymentReminderDaysBefore" binding:"min=0,max=99"`
	CreditCardNotificationMinutesOfDay        *int                    `json:"creditCardNotificationMinutesOfDay" binding:"omitempty,min=0,max=1439"`
	CreditCardNotificationTimezoneName        *string                 `json:"creditCardNotificationTimezoneName" binding:"omitempty,min=3,max=100"`
	CreditCardNotificationTimezoneUtcOffset   *int16                  `json:"creditCardNotificationTimezoneUtcOffset" binding:"omitempty,min=-720,max=840"`
	CreditCardIsEntityCard                    *bool                   `json:"creditCardIsEntityCard"`
	CreditCardCurrencyLimit                   string                  `json:"creditCardCurrencyLimit" binding:"omitempty,max=64"`
	Hidden                                    bool                    `json:"hidden"`
	SubAccounts                               []*AccountModifyRequest `json:"subAccounts" binding:"omitempty"`
	ClientSessionId                           string                  `json:"clientSessionId"`
}

// AccountUpdateLastReconciledTimeRequest represents all parameters of account updating last reconciled time request
type AccountUpdateLastReconciledTimeRequest struct {
	Id                 int64 `json:"id,string" binding:"required,min=1"`
	LastReconciledTime int64 `json:"lastReconciledTime" binding:"required"`
}

// AccountListRequest represents all parameters of account listing request
type AccountListRequest struct {
	VisibleOnly bool `form:"visible_only"`
}

// AccountGetRequest represents all parameters of account getting request
type AccountGetRequest struct {
	Id int64 `form:"id,string" binding:"required,min=1"`
}

// AccountHideRequest represents all parameters of account hiding request
type AccountHideRequest struct {
	Id     int64 `json:"id,string" binding:"required,min=1"`
	Hidden bool  `json:"hidden"`
}

// AccountMoveRequest represents all parameters of account moving request
type AccountMoveRequest struct {
	NewDisplayOrders []*AccountNewDisplayOrderRequest `json:"newDisplayOrders" binding:"required,min=1"`
}

// AccountNewDisplayOrderRequest represents a data pair of id and display order
type AccountNewDisplayOrderRequest struct {
	Id           int64 `json:"id,string" binding:"required,min=1"`
	DisplayOrder int32 `json:"displayOrder"`
}

// AccountDeleteRequest represents all parameters of account deleting request
type AccountDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// AccountInfoResponse represents a view-object of account
type AccountInfoResponse struct {
	Id                                        int64                    `json:"id,string"`
	Name                                      string                   `json:"name"`
	ParentId                                  int64                    `json:"parentId,string"`
	Category                                  AccountCategory          `json:"category"`
	Type                                      AccountType              `json:"type"`
	Icon                                      int64                    `json:"icon,string"`
	IconType                                  core.IconType            `json:"iconType"`
	Color                                     string                   `json:"color"`
	Currency                                  string                   `json:"currency"`
	Balance                                   string                   `json:"balance"`
	LastReconciledTime                        *int64                   `json:"lastReconciledTime,omitempty"`
	Comment                                   string                   `json:"comment"`
	CreditCardStatementDate                   *int                     `json:"creditCardStatementDate,omitempty"`
	CreditCardLimit                           *string                  `json:"creditCardLimit,omitempty"`
	CreditCardBankName                        *string                  `json:"creditCardBankName,omitempty"`
	CreditCardCardName                        *string                  `json:"creditCardCardName,omitempty"`
	CreditCardLastFourDigits                  *string                  `json:"creditCardLastFourDigits,omitempty"`
	CreditCardExpireMonth                     *int                     `json:"creditCardExpireMonth,omitempty"`
	CreditCardExpireYear                      *int                     `json:"creditCardExpireYear,omitempty"`
	CreditCardCardholderName                  *string                  `json:"creditCardCardholderName,omitempty"`
	CreditCardPictureId                       *int64                   `json:"creditCardPictureId,string,omitempty"`
	CreditCardPictureUrl                      string                   `json:"creditCardPictureUrl,omitempty"`
	CreditCardStatementDateIncludeTransaction *bool                    `json:"creditCardStatementDateIncludeTransaction,omitempty"`
	CreditCardRepaymentDateType               *int                     `json:"creditCardRepaymentDateType,omitempty"`
	CreditCardRepaymentDaysAfterStatement     *int                     `json:"creditCardRepaymentDaysAfterStatement,omitempty"`
	CreditCardRepaymentDayOfMonth             *int                     `json:"creditCardRepaymentDayOfMonth,omitempty"`
	CreditCardStatementReminderEnabled        *bool                    `json:"creditCardStatementReminderEnabled,omitempty"`
	CreditCardStatementEmailEnabled           *bool                    `json:"creditCardStatementEmailEnabled,omitempty"`
	CreditCardRepaymentReminderEnabled        *bool                    `json:"creditCardRepaymentReminderEnabled,omitempty"`
	CreditCardRepaymentReminderDaysBefore     *int                     `json:"creditCardRepaymentReminderDaysBefore,omitempty"`
	CreditCardNotificationMinutesOfDay        *int                     `json:"creditCardNotificationMinutesOfDay,omitempty"`
	CreditCardNotificationTimezoneName        *string                  `json:"creditCardNotificationTimezoneName,omitempty"`
	CreditCardNotificationTimezoneUtcOffset   *int16                   `json:"creditCardNotificationTimezoneUtcOffset,omitempty"`
	CreditCardIsEntityCard                    *bool                    `json:"creditCardIsEntityCard,omitempty"`
	CreditCardCurrencyLimit                   *string                  `json:"creditCardCurrencyLimit,omitempty"`
	DisplayOrder                              int32                    `json:"displayOrder"`
	IsAsset                                   bool                     `json:"isAsset,omitempty"`
	IsLiability                               bool                     `json:"isLiability,omitempty"`
	Hidden                                    bool                     `json:"hidden"`
	SubAccounts                               AccountInfoResponseSlice `json:"subAccounts,omitempty"`
}

// GetLastReconciledTime returns the last reconciled time of the account
func (a *Account) GetLastReconciledTime() int64 {
	if a.Extend != nil && a.Extend.LastReconciledTime != nil {
		return *a.Extend.LastReconciledTime
	}

	return 0
}

// ToAccountInfoResponse returns a view-object according to database model
func (a *Account) ToAccountInfoResponse() *AccountInfoResponse {
	var lastReconciledTime *int64
	var creditCardStatementDate *int
	var creditCardLimit *string
	var creditCardBankName *string
	var creditCardCardName *string
	var creditCardLastFourDigits *string
	var creditCardExpireMonth *int
	var creditCardExpireYear *int
	var creditCardCardholderName *string
	var creditCardPictureId *int64
	var creditCardStatementDateIncludeTransaction *bool
	var creditCardRepaymentDateType *int
	var creditCardRepaymentDaysAfterStatement *int
	var creditCardRepaymentDayOfMonth *int
	var creditCardStatementReminderEnabled *bool
	var creditCardStatementEmailEnabled *bool
	var creditCardRepaymentReminderEnabled *bool
	var creditCardRepaymentReminderDaysBefore *int
	var creditCardNotificationMinutesOfDay *int
	var creditCardNotificationTimezoneName *string
	var creditCardNotificationTimezoneUtcOffset *int16
	var creditCardIsEntityCard *bool
	var creditCardCurrencyLimit *string

	if a.Extend != nil {
		lastReconciledTime = a.Extend.LastReconciledTime
	}

	if a.Category == ACCOUNT_CATEGORY_CREDIT_CARD {
		// the credit limit belongs to the whole shared pool and is only exposed on the level-one account
		if a.ParentAccountId == LevelOneAccountParentId {
			if a.Extend != nil && a.Extend.CreditCardLimit != nil {
				textualLimit := utils.Int64ToString(*a.Extend.CreditCardLimit)
				creditCardLimit = &textualLimit
			} else {
				creditCardLimit = &defaultCreditCardAccountLimit
			}
		}

		if a.Extend != nil && a.Extend.CreditCardStatementDate != nil {
			creditCardStatementDate = a.Extend.CreditCardStatementDate
		} else {
			creditCardStatementDate = &defaultCreditCardAccountStatementDate
		}

		if a.Extend != nil {
			creditCardIsEntityCard = a.Extend.CreditCardIsEntityCard
			creditCardCurrencyLimit = a.Extend.CreditCardCurrencyLimit
			creditCardBankName = a.Extend.CreditCardBankName
			creditCardCardName = a.Extend.CreditCardCardName
			creditCardLastFourDigits = a.Extend.CreditCardLastFourDigits
			creditCardExpireMonth = a.Extend.CreditCardExpireMonth
			creditCardExpireYear = a.Extend.CreditCardExpireYear
			creditCardCardholderName = a.Extend.CreditCardCardholderName
			creditCardPictureId = a.Extend.CreditCardPictureId
			creditCardStatementDateIncludeTransaction = a.Extend.CreditCardStatementDateIncludeTransaction
			creditCardRepaymentDateType = a.Extend.CreditCardRepaymentDateType
			creditCardRepaymentDaysAfterStatement = a.Extend.CreditCardRepaymentDaysAfterStatement
			creditCardRepaymentDayOfMonth = a.Extend.CreditCardRepaymentDayOfMonth
			creditCardStatementReminderEnabled = a.Extend.CreditCardStatementReminderEnabled
			creditCardStatementEmailEnabled = a.Extend.CreditCardStatementEmailEnabled
			creditCardRepaymentReminderEnabled = a.Extend.CreditCardRepaymentReminderEnabled
			creditCardRepaymentReminderDaysBefore = a.Extend.CreditCardRepaymentReminderDaysBefore
			creditCardNotificationMinutesOfDay = a.Extend.CreditCardNotificationMinutesOfDay
			creditCardNotificationTimezoneName = a.Extend.CreditCardNotificationTimezoneName
			creditCardNotificationTimezoneUtcOffset = a.Extend.CreditCardNotificationTimezoneUtcOffset
		}
	}

	return &AccountInfoResponse{
		Id:                       a.AccountId,
		Name:                     a.Name,
		ParentId:                 a.ParentAccountId,
		Category:                 a.Category,
		Type:                     a.Type,
		Icon:                     a.Icon,
		IconType:                 a.IconType,
		Color:                    a.Color,
		Currency:                 a.Currency,
		Balance:                  utils.Int64ToString(a.Balance),
		Comment:                  a.Comment,
		LastReconciledTime:       lastReconciledTime,
		CreditCardStatementDate:  creditCardStatementDate,
		CreditCardLimit:          creditCardLimit,
		CreditCardBankName:       creditCardBankName,
		CreditCardCardName:       creditCardCardName,
		CreditCardLastFourDigits: creditCardLastFourDigits,
		CreditCardExpireMonth:    creditCardExpireMonth,
		CreditCardExpireYear:     creditCardExpireYear,
		CreditCardCardholderName: creditCardCardholderName,
		CreditCardPictureId:      creditCardPictureId,
		CreditCardStatementDateIncludeTransaction: creditCardStatementDateIncludeTransaction,
		CreditCardRepaymentDateType:               creditCardRepaymentDateType,
		CreditCardRepaymentDaysAfterStatement:     creditCardRepaymentDaysAfterStatement,
		CreditCardRepaymentDayOfMonth:             creditCardRepaymentDayOfMonth,
		CreditCardStatementReminderEnabled:        creditCardStatementReminderEnabled,
		CreditCardStatementEmailEnabled:           creditCardStatementEmailEnabled,
		CreditCardRepaymentReminderEnabled:        creditCardRepaymentReminderEnabled,
		CreditCardRepaymentReminderDaysBefore:     creditCardRepaymentReminderDaysBefore,
		CreditCardNotificationMinutesOfDay:        creditCardNotificationMinutesOfDay,
		CreditCardNotificationTimezoneName:        creditCardNotificationTimezoneName,
		CreditCardNotificationTimezoneUtcOffset:   creditCardNotificationTimezoneUtcOffset,
		CreditCardIsEntityCard:                    creditCardIsEntityCard,
		CreditCardCurrencyLimit:                   creditCardCurrencyLimit,
		DisplayOrder:                              a.DisplayOrder,
		IsAsset:                                   assetAccountCategory[a.Category],
		IsLiability:                               liabilityAccountCategory[a.Category],
		Hidden:                                    a.Hidden,
	}
}

// FromDB fills the fields from the data stored in database
func (a *AccountExtend) FromDB(data []byte) error {
	return json.Unmarshal(data, a)
}

// ToDB returns the actual stored data in database
func (a *AccountExtend) ToDB() ([]byte, error) {
	return json.Marshal(a)
}

// AccountInfoResponseSlice represents the slice data structure of AccountInfoResponse
type AccountInfoResponseSlice []*AccountInfoResponse

// Len returns the count of items
func (a AccountInfoResponseSlice) Len() int {
	return len(a)
}

// Swap swaps two items
func (a AccountInfoResponseSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// Less reports whether the first item is less than the second one
func (a AccountInfoResponseSlice) Less(i, j int) bool {
	if a[i].Category != a[j].Category {
		return a[i].Category < a[j].Category
	}

	return a[i].DisplayOrder < a[j].DisplayOrder
}
