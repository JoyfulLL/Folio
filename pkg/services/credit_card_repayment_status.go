package services

import (
	"math/big"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

const creditCardRepaymentTransactionPageCount = 1000

const creditCardRepaymentCancelledByPaidReason = "payment marked as paid"
const creditCardRepaymentCancelledByScheduleReason = "notification is no longer scheduled"

// creditCardRepaymentCycle represents the statement date and repayment date of one billing cycle
type creditCardRepaymentCycle struct {
	ScheduledDate int32
	StatementDate int32
}

// GetCurrentCreditCardRepaymentStatus returns the repayment status of the billing cycle which is
// currently due for the given account. It returns nil when the account has no repayment reminder.
func (s *CreditCardNotificationService) GetCurrentCreditCardRepaymentStatus(c core.Context, account *models.Account, currentUnixTime int64) (*models.CreditCardRepaymentStatusResponse, error) {
	cycle, ok := s.currentCreditCardRepaymentCycle(account, currentUnixTime)

	if !ok {
		return nil, nil
	}

	status, err := s.getCreditCardRepaymentStatus(c, s.UserDataDB(account.Uid), account.Uid, account.AccountId, cycle)

	if err != nil || status == nil || status.Paid {
		return status, err
	}

	// The current cycle is the one users look at in the account list. Detect a full repayment
	// here as well so recording the transfer immediately updates the UI instead of waiting for
	// the next notification scan to reach the cycle
	isFullyRepaid, err := s.isCreditCardCycleFullyRepaid(c, account, cycle)

	if err != nil {
		log.Warnf(c, "[credit_card_notifications.GetCurrentCreditCardRepaymentStatus] failed to check repayment of account \"id:%d\" of user \"uid:%d\" for cycle \"%d\", because %s", account.AccountId, account.Uid, cycle.ScheduledDate, err.Error())
		return status, nil
	}

	if !isFullyRepaid {
		return status, nil
	}

	return s.MarkCreditCardRepaymentPaid(c, account, cycle.ScheduledDate, models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_AUTO, time.Now().Unix())
}

// GetCreditCardRepaymentStatus returns the repayment status of the given billing cycle
func (s *CreditCardNotificationService) GetCreditCardRepaymentStatus(c core.Context, account *models.Account, scheduledDate int32) (*models.CreditCardRepaymentStatusResponse, error) {
	cycle, ok := s.creditCardRepaymentCycleByDate(account, scheduledDate)

	if !ok {
		return nil, nil
	}

	return s.getCreditCardRepaymentStatus(c, s.UserDataDB(account.Uid), account.Uid, account.AccountId, cycle)
}

// MarkCreditCardRepaymentPaid marks one billing cycle as repaid and stops its outstanding
// repayment reminders. A manually marked cycle is never overwritten by automatic detection.
func (s *CreditCardNotificationService) MarkCreditCardRepaymentPaid(c core.Context, account *models.Account, scheduledDate int32, source models.CreditCardRepaymentStatusSource, currentUnixTime int64) (*models.CreditCardRepaymentStatusResponse, error) {
	cycle, ok := s.creditCardRepaymentCycleByDate(account, scheduledDate)

	if !ok {
		return nil, nil
	}

	db := s.UserDataDB(account.Uid)
	status, err := s.getStoredCreditCardRepaymentStatus(c, db, account.Uid, account.AccountId, cycle.ScheduledDate)

	if err != nil {
		return nil, err
	}

	if status != nil && status.Source == models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_MANUAL && source != models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_MANUAL {
		return s.buildCreditCardRepaymentStatusResponse(cycle, status), nil
	}

	if status == nil {
		status = &models.CreditCardRepaymentStatus{
			StatusId:        s.GenerateUuid(uuid.UUID_TYPE_CREDIT_CARD_REPAYMENT),
			Uid:             account.Uid,
			AccountId:       account.AccountId,
			ScheduledDate:   cycle.ScheduledDate,
			StatementDate:   cycle.StatementDate,
			Source:          source,
			PaidUnixTime:    currentUnixTime,
			CreatedUnixTime: currentUnixTime,
		}
		_, err = db.NewSession(c).Insert(status)
	} else {
		status.Source = source
		status.StatementDate = cycle.StatementDate
		status.PaidUnixTime = currentUnixTime
		status.UpdatedUnixTime = currentUnixTime
		_, err = db.NewSession(c).Cols("source", "statement_date", "paid_unix_time", "updated_unix_time").ID(status.StatusId).Update(status)
	}

	if err != nil {
		return nil, err
	}

	err = s.cancelRepaymentNotifications(c, db, account.Uid, account.AccountId, cycle.ScheduledDate, currentUnixTime, creditCardRepaymentCancelledByPaidReason)

	if err != nil {
		return nil, err
	}

	return s.buildCreditCardRepaymentStatusResponse(cycle, status), nil
}

// UnmarkCreditCardRepaymentPaid removes a manual repayment mark. When the repayment transaction
// still covers the whole bill, the status is immediately recreated as automatically detected.
func (s *CreditCardNotificationService) UnmarkCreditCardRepaymentPaid(c core.Context, account *models.Account, scheduledDate int32, currentUnixTime int64) (*models.CreditCardRepaymentStatusResponse, error) {
	cycle, ok := s.creditCardRepaymentCycleByDate(account, scheduledDate)

	if !ok {
		return nil, nil
	}

	db := s.UserDataDB(account.Uid)
	status, err := s.getStoredCreditCardRepaymentStatus(c, db, account.Uid, account.AccountId, cycle.ScheduledDate)

	if err != nil {
		return nil, err
	}

	if status == nil || status.Source != models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_MANUAL {
		return s.getCreditCardRepaymentStatus(c, db, account.Uid, account.AccountId, cycle)
	}

	_, err = db.NewSession(c).ID(status.StatusId).Delete(&models.CreditCardRepaymentStatus{})

	if err != nil {
		return nil, err
	}

	isFullyRepaid, err := s.isCreditCardCycleFullyRepaid(c, account, cycle)

	if err != nil {
		return nil, err
	}

	if isFullyRepaid {
		_, err = s.MarkCreditCardRepaymentPaid(c, account, cycle.ScheduledDate, models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_AUTO, currentUnixTime)

		if err != nil {
			return nil, err
		}

		return s.getCreditCardRepaymentStatus(c, db, account.Uid, account.AccountId, cycle)
	}

	err = s.reviveRepaymentNotifications(c, db, account.Uid, account.AccountId, cycle.ScheduledDate, currentUnixTime)

	if err != nil {
		return nil, err
	}

	return s.buildCreditCardRepaymentStatusResponse(cycle, nil), nil
}

// syncAutoRepaymentStatuses reconciles the automatic repayment marks of all cycles in the current
// plan window. A mark is only kept while the recorded transfer into the card still covers the
// whole bill of that cycle.
func (s *CreditCardNotificationService) syncAutoRepaymentStatuses(c core.Context, db datastore.Database, account *models.Account, planItems []*creditCardNotificationPlanItem, currentUnixTime int64) error {
	seenCycles := make(map[int32]bool)

	for i := 0; i < len(planItems); i++ {
		planItem := planItems[i]

		if !isCreditCardRepaymentNotificationType(planItem.NotificationType) || seenCycles[planItem.ScheduledDate] {
			continue
		}

		seenCycles[planItem.ScheduledDate] = true
		cycle := creditCardRepaymentCycle{ScheduledDate: planItem.ScheduledDate, StatementDate: planItem.StatementDate}
		status, err := s.getStoredCreditCardRepaymentStatus(c, db, account.Uid, account.AccountId, cycle.ScheduledDate)

		if err != nil {
			return err
		}

		if status != nil && status.Source == models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_MANUAL {
			continue
		}

		isFullyRepaid, err := s.isCreditCardCycleFullyRepaid(c, account, cycle)

		if err != nil {
			log.Errorf(c, "[credit_card_notifications.syncAutoRepaymentStatuses] failed to check repayment of account \"id:%d\" of user \"uid:%d\" for cycle \"%d\", because %s", account.AccountId, account.Uid, cycle.ScheduledDate, err.Error())
			continue
		}

		if isFullyRepaid {
			_, err = s.MarkCreditCardRepaymentPaid(c, account, cycle.ScheduledDate, models.CREDIT_CARD_REPAYMENT_STATUS_SOURCE_AUTO, currentUnixTime)
		} else if status != nil {
			_, err = db.NewSession(c).ID(status.StatusId).Delete(&models.CreditCardRepaymentStatus{})

			if err == nil {
				err = s.reviveRepaymentNotifications(c, db, account.Uid, account.AccountId, cycle.ScheduledDate, currentUnixTime)
			}
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *CreditCardNotificationService) isCreditCardCyclePaid(c core.Context, db datastore.Database, uid int64, accountId int64, scheduledDate int32) (bool, error) {
	status, err := s.getStoredCreditCardRepaymentStatus(c, db, uid, accountId, scheduledDate)

	if err != nil {
		return false, err
	}

	return status != nil, nil
}

func (s *CreditCardNotificationService) getCreditCardRepaymentStatus(c core.Context, db datastore.Database, uid int64, accountId int64, cycle creditCardRepaymentCycle) (*models.CreditCardRepaymentStatusResponse, error) {
	status, err := s.getStoredCreditCardRepaymentStatus(c, db, uid, accountId, cycle.ScheduledDate)

	if err != nil {
		return nil, err
	}

	return s.buildCreditCardRepaymentStatusResponse(cycle, status), nil
}

func (s *CreditCardNotificationService) getStoredCreditCardRepaymentStatus(c core.Context, db datastore.Database, uid int64, accountId int64, scheduledDate int32) (*models.CreditCardRepaymentStatus, error) {
	status := &models.CreditCardRepaymentStatus{}
	has, err := db.NewSession(c).Where("uid=? AND account_id=? AND scheduled_date=? AND deleted=?", uid, accountId, scheduledDate, false).Get(status)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, nil
	}

	return status, nil
}

func (s *CreditCardNotificationService) buildCreditCardRepaymentStatusResponse(cycle creditCardRepaymentCycle, status *models.CreditCardRepaymentStatus) *models.CreditCardRepaymentStatusResponse {
	response := &models.CreditCardRepaymentStatusResponse{
		ScheduledDate: cycle.ScheduledDate,
		StatementDate: cycle.StatementDate,
	}

	if status != nil {
		response.Paid = true
		response.Source = status.Source
	}

	return response
}

// isCreditCardCycleFullyRepaid compares the bill of the cycle with all transfers into the card
// from the day after the statement date until the end of the repayment date. A partially repaid
// bill is deliberately not treated as repaid, so the remaining reminders are still delivered.
func (s *CreditCardNotificationService) isCreditCardCycleFullyRepaid(c core.Context, account *models.Account, cycle creditCardRepaymentCycle) (bool, error) {
	extend := account.Extend

	if extend == nil || extend.CreditCardStatementDate == nil {
		return false, nil
	}

	location := creditCardNotificationLocation(extend)
	statementTime, ok := creditCardRepaymentNumericDateToTime(cycle.StatementDate, location)

	if !ok {
		return false, nil
	}

	includeStatementDate := creditCardNotificationBoolPtrValue(extend.CreditCardStatementDateIncludeTransaction)
	cycleMinUnixTime, cycleMaxUnixTime := utils.GetStatementBillingCycleRangeForTime(statementTime.Unix(), *extend.CreditCardStatementDate, includeStatementDate, location)
	statement, err := s.transactions.GetAccountStatementByTimeRange(c, account.Uid, account.AccountId, account.Category, cycleMinUnixTime, cycleMaxUnixTime)

	if err != nil {
		return false, err
	}

	amountDue := new(big.Int).Set(statement.ClosingBalance)

	if amountDue.Sign() >= 0 {
		return false, nil
	}

	amountDue.Neg(amountDue)
	repaymentTime, ok := creditCardRepaymentNumericDateToTime(cycle.ScheduledDate, location)

	if !ok {
		return false, nil
	}

	endOfRepaymentTime := time.Date(repaymentTime.Year(), repaymentTime.Month(), repaymentTime.Day(), 23, 59, 59, 0, location)
	maxTransactionTime := utils.GetMaxTransactionTimeFromUnixTime(endOfRepaymentTime.Unix())
	minTransactionTime := utils.GetMinTransactionTimeFromUnixTime(cycleMaxUnixTime + 1)
	totalRepaid := big.NewInt(0)

	for maxTransactionTime > 0 {
		transactions, err := s.transactions.GetTransactionsByMaxTime(c, account.Uid, maxTransactionTime, minTransactionTime, models.TRANSACTION_TYPE_TRANSFER, nil, []int64{account.AccountId}, nil, false, "", "", core.MATCH_MODE_DEFAULT, false, 1, creditCardRepaymentTransactionPageCount, false, true)

		if err != nil {
			return false, err
		}

		for i := 0; i < len(transactions); i++ {
			if transactions[i].Type != models.TRANSACTION_DB_TYPE_TRANSFER_IN {
				continue
			}

			totalRepaid.Add(totalRepaid, big.NewInt(transactions[i].Amount))
		}

		if len(transactions) < creditCardRepaymentTransactionPageCount {
			break
		}

		maxTransactionTime = transactions[len(transactions)-1].TransactionTime - 1
	}

	return totalRepaid.Cmp(amountDue) >= 0, nil
}

func (s *CreditCardNotificationService) currentCreditCardRepaymentCycle(account *models.Account, currentUnixTime int64) (creditCardRepaymentCycle, bool) {
	if !creditCardNotificationCanHaveRepaymentCycle(account) {
		return creditCardRepaymentCycle{}, false
	}

	extend := account.Extend
	location := creditCardNotificationLocation(extend)
	currentLocalTime := time.Unix(currentUnixTime, 0).In(location)
	today := time.Date(currentLocalTime.Year(), currentLocalTime.Month(), currentLocalTime.Day(), 0, 0, 0, 0, location)
	statementDate := *extend.CreditCardStatementDate
	firstStatementTime := time.Date(currentLocalTime.Year(), currentLocalTime.Month(), statementDate, 0, 0, 0, 0, location)

	for monthOffset := -2; monthOffset <= 2; monthOffset++ {
		candidateStatementTime := firstStatementTime.AddDate(0, monthOffset, 0)
		repaymentTime, ok := utils.GetCreditCardRepaymentTimeByStatementTime(candidateStatementTime, creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDateType), creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDaysAfterStatement), creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDayOfMonth))

		if !ok || repaymentTime.Before(today) {
			continue
		}

		return creditCardRepaymentCycle{
			ScheduledDate: utils.FormatUnixTimeToNumericYearMonthDay(repaymentTime.Unix(), location),
			StatementDate: utils.FormatUnixTimeToNumericYearMonthDay(candidateStatementTime.Unix(), location),
		}, true
	}

	return creditCardRepaymentCycle{}, false
}

func (s *CreditCardNotificationService) creditCardRepaymentCycleByDate(account *models.Account, scheduledDate int32) (creditCardRepaymentCycle, bool) {
	if !creditCardNotificationCanHaveRepaymentCycle(account) {
		return creditCardRepaymentCycle{}, false
	}

	extend := account.Extend
	location := creditCardNotificationLocation(extend)
	targetRepaymentTime, ok := creditCardRepaymentNumericDateToTime(scheduledDate, location)

	if !ok {
		return creditCardRepaymentCycle{}, false
	}

	statementDate := *extend.CreditCardStatementDate
	targetMonthStart := time.Date(targetRepaymentTime.Year(), targetRepaymentTime.Month(), 1, 0, 0, 0, 0, location)

	for monthOffset := -2; monthOffset <= 2; monthOffset++ {
		candidateStatementTime := targetMonthStart.AddDate(0, monthOffset, 0).AddDate(0, 0, statementDate-1)
		repaymentTime, ok := utils.GetCreditCardRepaymentTimeByStatementTime(candidateStatementTime, creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDateType), creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDaysAfterStatement), creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDayOfMonth))

		if !ok || utils.FormatUnixTimeToNumericYearMonthDay(repaymentTime.Unix(), location) != scheduledDate {
			continue
		}

		return creditCardRepaymentCycle{
			ScheduledDate: scheduledDate,
			StatementDate: utils.FormatUnixTimeToNumericYearMonthDay(candidateStatementTime.Unix(), location),
		}, true
	}

	return creditCardRepaymentCycle{}, false
}

func creditCardNotificationCanHaveRepaymentCycle(account *models.Account) bool {
	if !creditCardAccountIsEntity(account) || account.Extend == nil {
		return false
	}

	extend := account.Extend

	if !creditCardNotificationBoolPtrValue(extend.CreditCardRepaymentReminderEnabled) || extend.CreditCardStatementDate == nil {
		return false
	}

	statementDate := *extend.CreditCardStatementDate

	return statementDate >= creditCardNotificationMinStatementDate && statementDate <= creditCardNotificationMaxStatementDate
}

func (s *CreditCardNotificationService) cancelRepaymentNotifications(c core.Context, db datastore.Database, uid int64, accountId int64, scheduledDate int32, currentUnixTime int64, reason string) error {
	updateModel := &models.CreditCardNotificationLog{
		Status:          models.CREDIT_CARD_NOTIFICATION_STATUS_CANCELLED,
		LastError:       reason,
		UpdatedUnixTime: currentUnixTime,
	}
	_, err := db.NewSession(c).Cols("status", "last_error", "updated_unix_time").
		Where("uid=? AND account_id=? AND scheduled_date=? AND deleted=? AND notification_type IN (?, ?) AND status IN (?, ?)", uid, accountId, scheduledDate, false,
			models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE,
			models.CREDIT_CARD_NOTIFICATION_STATUS_PENDING, models.CREDIT_CARD_NOTIFICATION_STATUS_FAILED).
		Update(updateModel)

	return err
}

func (s *CreditCardNotificationService) reviveRepaymentNotifications(c core.Context, db datastore.Database, uid int64, accountId int64, scheduledDate int32, currentUnixTime int64) error {
	updateModel := &models.CreditCardNotificationLog{
		Status:          models.CREDIT_CARD_NOTIFICATION_STATUS_PENDING,
		RetryCount:      0,
		LastError:       "",
		UpdatedUnixTime: currentUnixTime,
	}
	_, err := db.NewSession(c).Cols("status", "retry_count", "last_error", "updated_unix_time").
		Where("uid=? AND account_id=? AND scheduled_date=? AND deleted=? AND notification_type IN (?, ?) AND status=? AND last_error IN (?, ?)", uid, accountId, scheduledDate, false,
			models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE,
			models.CREDIT_CARD_NOTIFICATION_STATUS_CANCELLED, creditCardRepaymentCancelledByPaidReason, creditCardRepaymentCancelledByScheduleReason).
		Update(updateModel)

	return err
}

func isCreditCardRepaymentNotificationType(notificationType models.CreditCardNotificationType) bool {
	return notificationType == models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE || notificationType == models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE
}

func creditCardRepaymentNumericDateToTime(yearMonthDay int32, location *time.Location) (time.Time, bool) {
	year := int(yearMonthDay / 10000)
	month := int(yearMonthDay % 10000 / 100)
	day := int(yearMonthDay % 100)

	if year < 1900 || year > 2999 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}

	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)

	if value.Year() != year || int(value.Month()) != month || value.Day() != day {
		return time.Time{}, false
	}

	return value, true
}
