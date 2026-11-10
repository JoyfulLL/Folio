package models

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccountToAccountInfoResponseBalance(t *testing.T) {
	account := &Account{Balance: math.MaxInt64}
	response := account.ToAccountInfoResponse()
	assert.Equal(t, "9223372036854775807", response.Balance)

	account.Balance = math.MinInt64
	response = account.ToAccountInfoResponse()
	assert.Equal(t, "-9223372036854775808", response.Balance)
}

func testCreditCardPoolAccount() *Account {
	entityCard := false
	currencyLimit := "129,000"
	minutesOfDay := 9 * 60

	return &Account{
		AccountId:       100,
		Uid:             10000,
		Name:            "Global Payment",
		Category:        ACCOUNT_CATEGORY_CREDIT_CARD,
		Type:            ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS,
		Currency:        "CNY",
		ParentAccountId: LevelOneAccountParentId,
		Extend: &AccountExtend{
			CreditCardLimit:                         func() *int64 { v := int64(3000000); return &v }(),
			CreditCardStatementDate:                 func() *int { v := 5; return &v }(),
			CreditCardIsEntityCard:                  &entityCard,
			CreditCardCurrencyLimit:                 &currencyLimit,
			CreditCardNotificationMinutesOfDay:      &minutesOfDay,
			CreditCardNotificationTimezoneUtcOffset: func() *int16 { v := int16(480); return &v }(),
		},
	}
}

func testCreditCardSubAccount() *Account {
	entityCard := true
	currencyLimit := "30,000"
	statementDate := 15

	return &Account{
		AccountId:       101,
		Uid:             10000,
		Name:            "Mastercard 5083",
		Category:        ACCOUNT_CATEGORY_CREDIT_CARD,
		Type:            ACCOUNT_TYPE_SINGLE_ACCOUNT,
		Currency:        "CNY",
		ParentAccountId: 100,
		Extend: &AccountExtend{
			CreditCardStatementDate:  &statementDate,
			CreditCardIsEntityCard:   &entityCard,
			CreditCardCurrencyLimit:  &currencyLimit,
			CreditCardBankName:       func() *string { v := "ABC"; return &v }(),
			CreditCardLastFourDigits: func() *string { v := "5083"; return &v }(),
		},
	}
}

func TestAccountExtendCreditCardPoolFieldsRoundTrip(t *testing.T) {
	// serializing and deserializing the extend keeps all pool fields, so no data is lost
	account := testCreditCardPoolAccount()
	data, err := account.Extend.ToDB()
	assert.Nil(t, err)

	extend := &AccountExtend{}
	err = extend.FromDB(data)
	assert.Nil(t, err)

	assert.NotNil(t, extend.CreditCardLimit)
	assert.Equal(t, int64(3000000), *extend.CreditCardLimit)
	assert.NotNil(t, extend.CreditCardStatementDate)
	assert.Equal(t, 5, *extend.CreditCardStatementDate)
	assert.NotNil(t, extend.CreditCardIsEntityCard)
	assert.False(t, *extend.CreditCardIsEntityCard)
	assert.NotNil(t, extend.CreditCardCurrencyLimit)
	assert.Equal(t, "129,000", *extend.CreditCardCurrencyLimit)
	assert.NotNil(t, extend.CreditCardNotificationMinutesOfDay)
	assert.Equal(t, 9*60, *extend.CreditCardNotificationMinutesOfDay)
}

func TestAccountExtendBackwardCompatibleWithOldStoredData(t *testing.T) {
	// extends stored before the shared-limit feature have neither of the new fields, and they
	// must be loaded without errors as accounts without an explicit entity flag
	extend := &AccountExtend{}
	err := extend.FromDB([]byte(`{"creditCardStatementDate":10}`))
	assert.Nil(t, err)

	assert.NotNil(t, extend.CreditCardStatementDate)
	assert.Equal(t, 10, *extend.CreditCardStatementDate)
	assert.Nil(t, extend.CreditCardIsEntityCard)
	assert.Nil(t, extend.CreditCardCurrencyLimit)
}

func TestAccountToAccountInfoResponseCreditCardFields(t *testing.T) {
	// the level-one account exposes the shared pool limit and the card fields
	poolResponse := testCreditCardPoolAccount().ToAccountInfoResponse()
	assert.NotNil(t, poolResponse.CreditCardLimit)
	assert.Equal(t, "3000000", *poolResponse.CreditCardLimit)
	assert.NotNil(t, poolResponse.CreditCardIsEntityCard)
	assert.False(t, *poolResponse.CreditCardIsEntityCard)
	assert.NotNil(t, poolResponse.CreditCardCurrencyLimit)
	assert.Equal(t, "129,000", *poolResponse.CreditCardCurrencyLimit)

	// a sub-account card exposes its own card fields, but never the shared pool limit
	subAccountResponse := testCreditCardSubAccount().ToAccountInfoResponse()
	assert.Nil(t, subAccountResponse.CreditCardLimit)
	assert.NotNil(t, subAccountResponse.CreditCardStatementDate)
	assert.Equal(t, 15, *subAccountResponse.CreditCardStatementDate)
	assert.NotNil(t, subAccountResponse.CreditCardIsEntityCard)
	assert.True(t, *subAccountResponse.CreditCardIsEntityCard)
	assert.NotNil(t, subAccountResponse.CreditCardCurrencyLimit)
	assert.Equal(t, "30,000", *subAccountResponse.CreditCardCurrencyLimit)
	assert.NotNil(t, subAccountResponse.CreditCardBankName)
	assert.Equal(t, "ABC", *subAccountResponse.CreditCardBankName)
	assert.NotNil(t, subAccountResponse.CreditCardLastFourDigits)
	assert.Equal(t, "5083", *subAccountResponse.CreditCardLastFourDigits)

	// a non-credit-card account exposes no credit card fields at all
	checkingAccount := &Account{
		AccountId:       200,
		Uid:             10000,
		Name:            "Cash",
		Category:        ACCOUNT_CATEGORY_CASH,
		Type:            ACCOUNT_TYPE_SINGLE_ACCOUNT,
		Currency:        "CNY",
		ParentAccountId: LevelOneAccountParentId,
	}
	checkingAccountResponse := checkingAccount.ToAccountInfoResponse()
	assert.Nil(t, checkingAccountResponse.CreditCardLimit)
	assert.Nil(t, checkingAccountResponse.CreditCardStatementDate)
	assert.Nil(t, checkingAccountResponse.CreditCardIsEntityCard)
	assert.Nil(t, checkingAccountResponse.CreditCardCurrencyLimit)
}

func TestAccountInfoResponseSliceLess(t *testing.T) {
	var accountRespSlice AccountInfoResponseSlice
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           1,
		Category:     ACCOUNT_CATEGORY_CHECKING_ACCOUNT,
		DisplayOrder: int32(1),
	})
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           2,
		Category:     ACCOUNT_CATEGORY_CASH,
		DisplayOrder: int32(3),
	})
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           3,
		Category:     ACCOUNT_CATEGORY_CREDIT_CARD,
		DisplayOrder: int32(2),
	})
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           4,
		Category:     ACCOUNT_CATEGORY_CASH,
		DisplayOrder: int32(2),
	})
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           5,
		Category:     ACCOUNT_CATEGORY_CREDIT_CARD,
		DisplayOrder: int32(1),
	})
	accountRespSlice = append(accountRespSlice, &AccountInfoResponse{
		Id:           6,
		Category:     ACCOUNT_CATEGORY_CASH,
		DisplayOrder: int32(1),
	})

	sort.Sort(accountRespSlice)

	assert.Equal(t, int64(6), accountRespSlice[0].Id)
	assert.Equal(t, int64(4), accountRespSlice[1].Id)
	assert.Equal(t, int64(2), accountRespSlice[2].Id)
	assert.Equal(t, int64(1), accountRespSlice[3].Id)
	assert.Equal(t, int64(5), accountRespSlice[4].Id)
	assert.Equal(t, int64(3), accountRespSlice[5].Id)
}
