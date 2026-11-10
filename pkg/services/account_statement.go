package services

import (
	"math/big"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const pageCountForAccountStatement = 1000

// AccountStatementTransaction represents a transaction line of an account statement
type AccountStatementTransaction struct {
	TransactionTime      int64
	TimezoneUtcOffset    int16
	Type                 models.TransactionDbType
	CategoryId           int64
	AccountId            int64
	RelatedAccountId     int64
	Amount               int64
	RelatedAccountAmount int64
	HideAmount           bool
	Comment              string
	OpeningBalance       *big.Int
	ClosingBalance       *big.Int
}

// AccountStatement represents the statement data of a single account within a time range
type AccountStatement struct {
	Transactions   []*AccountStatementTransaction
	TotalInflows   *big.Int
	TotalOutflows  *big.Int
	OpeningBalance *big.Int
	ClosingBalance *big.Int
}

// GetAccountStatementByTimeRange returns the statement of a single account within the given unix
// time range (in seconds, zero means unlimited). It reuses the transaction calculating logic of
// the account statement feature, so all the amounts are the original amounts of the account
// currency without any currency exchange
func (s *TransactionService) GetAccountStatementByTimeRange(c core.Context, uid int64, accountId int64, accountCategory models.AccountCategory, minUnixTime int64, maxUnixTime int64) (*AccountStatement, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if accountId <= 0 {
		return nil, errs.ErrAccountIdInvalid
	}

	minTransactionTime := int64(0)
	maxTransactionTime := int64(0)

	if minUnixTime > 0 {
		minTransactionTime = utils.GetMinTransactionTimeFromUnixTime(minUnixTime)
	}

	if maxUnixTime > 0 {
		maxTransactionTime = utils.GetMaxTransactionTimeFromUnixTime(maxUnixTime)
	}

	transactionsWithAccountBalance, totalInflows, totalOutflows, openingBalance, closingBalance, err := s.GetAllTransactionsInOneAccountWithAccountBalanceByMaxTime(c, uid, pageCountForAccountStatement, maxTransactionTime, minTransactionTime, accountId, accountCategory)

	if err != nil {
		return nil, err
	}

	statement := &AccountStatement{
		Transactions:   make([]*AccountStatementTransaction, len(transactionsWithAccountBalance)),
		TotalInflows:   totalInflows,
		TotalOutflows:  totalOutflows,
		OpeningBalance: openingBalance,
		ClosingBalance: closingBalance,
	}

	for i := 0; i < len(transactionsWithAccountBalance); i++ {
		transactionWithAccountBalance := transactionsWithAccountBalance[i]
		statement.Transactions[i] = &AccountStatementTransaction{
			TransactionTime:      transactionWithAccountBalance.TransactionTime,
			TimezoneUtcOffset:    transactionWithAccountBalance.TimezoneUtcOffset,
			Type:                 transactionWithAccountBalance.Type,
			CategoryId:           transactionWithAccountBalance.CategoryId,
			AccountId:            transactionWithAccountBalance.AccountId,
			RelatedAccountId:     transactionWithAccountBalance.RelatedAccountId,
			Amount:               transactionWithAccountBalance.Amount,
			RelatedAccountAmount: transactionWithAccountBalance.RelatedAccountAmount,
			HideAmount:           transactionWithAccountBalance.HideAmount,
			Comment:              transactionWithAccountBalance.Comment,
			OpeningBalance:       transactionWithAccountBalance.AccountOpeningBalance,
			ClosingBalance:       transactionWithAccountBalance.AccountClosingBalance,
		}
	}

	return statement, nil
}
