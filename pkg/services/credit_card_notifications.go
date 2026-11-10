package services

import (
	"bytes"
	"fmt"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/locales"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/templates"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// the catch up duration matches the send window, so correcting a statement date or a reminder
// time later in the day still delivers the notification of that day
const creditCardNotificationCatchUpDuration = 24 * time.Hour
const creditCardNotificationLookAheadDuration = 26 * time.Hour
const creditCardNotificationSendWindowDuration = 24 * time.Hour
const creditCardNotificationMaxRetryCount = int16(6)
const creditCardNotificationPageCount = 100
const creditCardNotificationDefaultMinutesOfDay = 9 * 60
const creditCardNotificationMaxMinutesOfDay = 24*60 - 1
const creditCardNotificationMinStatementDate = 1
const creditCardNotificationMaxStatementDate = 28
const creditCardNotificationPlanMonthsBefore = 5
const creditCardNotificationPlanMonthsAfter = 1
const creditCardNotificationMaxLastErrorLength = 200
const creditCardAccountListUrlFormat = "%sdesktop#/account/list"

// CreditCardNotificationService represents credit card notification service
type CreditCardNotificationService struct {
	ServiceUsingDB
	ServiceUsingConfig
	ServiceUsingMailer
	ServiceUsingUuid
	users        *UserService
	accounts     *AccountService
	categories   *TransactionCategoryService
	transactions *TransactionService
}

// creditCardNotificationPlanItem represents one notification planned for one account
type creditCardNotificationPlanItem struct {
	Uid              int64
	AccountId        int64
	NotificationType models.CreditCardNotificationType
	ScheduledDate    int32
	SendUnixTime     int64
}

// Initialize a credit card notification service singleton instance
var (
	CreditCardNotifications = &CreditCardNotificationService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
		ServiceUsingMailer: ServiceUsingMailer{
			container: mail.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
		users:        Users,
		accounts:     Accounts,
		categories:   TransactionCategories,
		transactions: Transactions,
	}
)

// RunCreditCardNotifications plans and sends the credit card notifications which belong to the
// current time. Plans are recomputed on every scan of the accounts of a user, so correcting a
// statement date or a reminder time still produces the notification of the current day. The unique
// index of uid, account id, notification type and scheduled date keeps every notification sent only
// once, so restarting the service or running the job more than once never sends the same
// notification to the same account twice
func (s *CreditCardNotificationService) RunCreditCardNotifications(c core.Context, currentUnixTime int64) error {
	if !s.CurrentConfig().EnableSMTP {
		log.Warnf(c, "[credit_card_notifications.RunCreditCardNotifications] credit card notifications are skipped, because smtp server is not enabled")
		return nil
	}

	var firstError error

	// the job ticks every 15 minutes so an already planned notification is always sent on time,
	// but the accounts of a user are only scanned once the interval they chose has elapsed
	scanUids, err := s.users.GetUserIdsNeedCreditCardNotificationScan(c, currentUnixTime)

	if err != nil {
		log.Errorf(c, "[credit_card_notifications.RunCreditCardNotifications] failed to get the users whose credit card accounts should be scanned, because %s", err.Error())
		firstError = err
	}

	for i := 0; i < s.UserDataDBCount(); i++ {
		db := s.UserDataDBByIndex(i)

		if len(scanUids) > 0 {
			if err := s.planNotifications(c, db, currentUnixTime, scanUids); err != nil {
				log.Errorf(c, "[credit_card_notifications.RunCreditCardNotifications] failed to plan credit card notifications for data store %d, because %s", i, err.Error())

				if firstError == nil {
					firstError = err
				}
			}
		}

		if err := s.sendNotifications(c, db, currentUnixTime); err != nil {
			log.Errorf(c, "[credit_card_notifications.RunCreditCardNotifications] failed to send credit card notifications for data store %d, because %s", i, err.Error())

			if firstError == nil {
				firstError = err
			}
		}
	}

	// a failed run is not recorded, so the accounts are scanned again on the next tick
	if len(scanUids) > 0 && firstError == nil {
		if err := s.users.UpdateUsersCreditCardNotificationLastScanTime(c, scanUids, currentUnixTime); err != nil {
			log.Errorf(c, "[credit_card_notifications.RunCreditCardNotifications] failed to record the credit card notification scan time, because %s", err.Error())
			firstError = err
		}
	}

	return firstError
}

func (s *CreditCardNotificationService) planNotifications(c core.Context, db datastore.Database, currentUnixTime int64, uids []int64) error {
	var accounts []*models.Account

	// every physical credit card account is planned independently, including sub-accounts, so
	// cards sharing one limit pool but with different statement dates are notified separately.
	// Pure containers (main accounts explicitly marked as non-entity cards) are skipped here
	err := db.NewSession(c).Where("deleted=? AND category=?", false, models.ACCOUNT_CATEGORY_CREDIT_CARD).In("uid", uids).Find(&accounts)

	if err != nil {
		return err
	}

	minSendUnixTime := currentUnixTime - int64(creditCardNotificationCatchUpDuration.Seconds())
	maxSendUnixTime := currentUnixTime + int64(creditCardNotificationLookAheadDuration.Seconds())
	createdCount := 0

	for i := 0; i < len(accounts); i++ {
		planItems := s.buildNotificationPlans(accounts[i], currentUnixTime, minSendUnixTime, maxSendUnixTime)

		for j := 0; j < len(planItems); j++ {
			added, err := s.addNotificationPlan(c, db, planItems[j], currentUnixTime)

			if err != nil {
				log.Errorf(c, "[credit_card_notifications.planNotifications] failed to plan credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\", because %s", planItems[j].NotificationType, planItems[j].AccountId, planItems[j].Uid, err.Error())
				continue
			}

			if added {
				createdCount++
			}
		}
	}

	log.Infof(c, "[credit_card_notifications.planNotifications] %d credit card notifications have been planned for %d credit card accounts in data store", createdCount, len(accounts))

	return nil
}

// buildNotificationPlans returns all the notifications which should be sent between minSendUnixTime
// and maxSendUnixTime for the given account. Statement and repayment dates are checked from the
// statement dates of the last months, because the repayment date may be several months after the
// statement date
func (s *CreditCardNotificationService) buildNotificationPlans(account *models.Account, currentUnixTime int64, minSendUnixTime int64, maxSendUnixTime int64) []*creditCardNotificationPlanItem {
	planItems := make([]*creditCardNotificationPlanItem, 0)

	if account == nil || account.Extend == nil {
		return planItems
	}

	// pure containers only hold the shared limit of their sub-accounts and have no billing
	// cycle of their own, so they never generate notifications
	if !creditCardAccountIsEntity(account) {
		return planItems
	}

	extend := account.Extend
	statementReminderEnabled := creditCardNotificationBoolPtrValue(extend.CreditCardStatementReminderEnabled)
	statementEmailEnabled := creditCardNotificationBoolPtrValue(extend.CreditCardStatementEmailEnabled)
	repaymentReminderEnabled := creditCardNotificationBoolPtrValue(extend.CreditCardRepaymentReminderEnabled)

	if !statementReminderEnabled && !statementEmailEnabled && !repaymentReminderEnabled {
		return planItems
	}

	statementDate := creditCardNotificationIntPtrValue(extend.CreditCardStatementDate)

	if statementDate < creditCardNotificationMinStatementDate || statementDate > creditCardNotificationMaxStatementDate {
		return planItems
	}

	location := creditCardNotificationLocation(extend)
	minutesOfDay := creditCardNotificationIntPtrValue(extend.CreditCardNotificationMinutesOfDay)

	if minutesOfDay < 0 || minutesOfDay > creditCardNotificationMaxMinutesOfDay {
		minutesOfDay = creditCardNotificationDefaultMinutesOfDay
	}

	includeStatementDate := creditCardNotificationBoolPtrValue(extend.CreditCardStatementDateIncludeTransaction)
	repaymentDateType := creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDateType)
	daysAfterStatement := creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDaysAfterStatement)
	repaymentDayOfMonth := creditCardNotificationIntPtrValue(extend.CreditCardRepaymentDayOfMonth)
	daysBeforeRepayment := creditCardNotificationIntPtrValue(extend.CreditCardRepaymentReminderDaysBefore)
	hours := minutesOfDay / 60
	minutes := minutesOfDay % 60
	currentYear, currentMonth, _ := time.Unix(currentUnixTime, 0).In(location).Date()
	firstStatementTime := time.Date(currentYear, currentMonth, statementDate, 0, 0, 0, 0, location)

	addPlanItem := func(notificationType models.CreditCardNotificationType, eventDate time.Time, sendDate time.Time) {
		sendTime := time.Date(sendDate.Year(), sendDate.Month(), sendDate.Day(), hours, minutes, 0, 0, location)
		sendUnixTime := sendTime.Unix()

		if sendUnixTime < minSendUnixTime || sendUnixTime > maxSendUnixTime {
			return
		}

		planItems = append(planItems, &creditCardNotificationPlanItem{
			Uid:              account.Uid,
			AccountId:        account.AccountId,
			NotificationType: notificationType,
			ScheduledDate:    utils.FormatUnixTimeToNumericYearMonthDay(eventDate.Unix(), location),
			SendUnixTime:     sendUnixTime,
		})
	}

	for monthOffset := -creditCardNotificationPlanMonthsBefore; monthOffset <= creditCardNotificationPlanMonthsAfter; monthOffset++ {
		statementTime := firstStatementTime.AddDate(0, monthOffset, 0)

		if statementReminderEnabled {
			addPlanItem(models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER, statementTime, statementTime)
		}

		if statementEmailEnabled {
			statementEmailTime := statementTime

			// transactions on the statement date belong to the current billing cycle, so the
			// billing cycle is not finished until the end of the statement date
			if includeStatementDate {
				statementEmailTime = statementTime.AddDate(0, 0, 1)
			}

			addPlanItem(models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL, statementTime, statementEmailTime)
		}

		if !repaymentReminderEnabled {
			continue
		}

		repaymentTime, hasRepaymentDate := utils.GetCreditCardRepaymentTimeByStatementTime(statementTime, repaymentDateType, daysAfterStatement, repaymentDayOfMonth)

		if !hasRepaymentDate {
			continue
		}

		addPlanItem(models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE, repaymentTime, repaymentTime)

		if daysBeforeRepayment > 0 {
			addPlanItem(models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, repaymentTime, repaymentTime.AddDate(0, 0, -daysBeforeRepayment))
		}
	}

	return planItems
}

func (s *CreditCardNotificationService) addNotificationPlan(c core.Context, db datastore.Database, planItem *creditCardNotificationPlanItem, currentUnixTime int64) (bool, error) {
	notification := &models.CreditCardNotificationLog{}
	has, err := db.NewSession(c).Where("uid=? AND account_id=? AND notification_type=? AND scheduled_date=? AND deleted=?", planItem.Uid, planItem.AccountId, planItem.NotificationType, planItem.ScheduledDate, false).Get(notification)

	if err != nil {
		return false, err
	}

	if has {
		// a corrected reminder time still moves the notification of the planned day, but a
		// notification which has already been sent or cancelled is never revived
		if notification.Status != models.CREDIT_CARD_NOTIFICATION_STATUS_PENDING && notification.Status != models.CREDIT_CARD_NOTIFICATION_STATUS_FAILED {
			return false, nil
		}

		if notification.SendUnixTime == planItem.SendUnixTime {
			return false, nil
		}

		updateModel := &models.CreditCardNotificationLog{
			SendUnixTime:    planItem.SendUnixTime,
			UpdatedUnixTime: currentUnixTime,
		}

		_, err = db.NewSession(c).Cols("send_unix_time", "updated_unix_time").ID(notification.NotificationLogId).Update(updateModel)

		return false, err
	}

	notificationModel := &models.CreditCardNotificationLog{
		NotificationLogId: s.GenerateUuid(uuid.UUID_TYPE_CREDIT_CARD_NOTIFICATION),
		Uid:               planItem.Uid,
		Deleted:           false,
		AccountId:         planItem.AccountId,
		NotificationType:  planItem.NotificationType,
		ScheduledDate:     planItem.ScheduledDate,
		SendUnixTime:      planItem.SendUnixTime,
		Status:            models.CREDIT_CARD_NOTIFICATION_STATUS_PENDING,
		RetryCount:        0,
		CreatedUnixTime:   currentUnixTime,
		UpdatedUnixTime:   currentUnixTime,
	}

	// the unique index of uid, account id, notification type and scheduled date also prevents
	// duplicated notifications when multiple instances are planning at the same time
	if _, err := db.NewSession(c).Insert(notificationModel); err != nil {
		return false, err
	}

	return true, nil
}

func (s *CreditCardNotificationService) sendNotifications(c core.Context, db datastore.Database, currentUnixTime int64) error {
	minSendUnixTime := currentUnixTime - int64(creditCardNotificationSendWindowDuration.Seconds())
	var notificationLogs []*models.CreditCardNotificationLog

	err := db.NewSession(c).Where("deleted=? AND notification_type IN (?, ?, ?, ?) AND status IN (?, ?) AND send_unix_time>=? AND send_unix_time<=? AND retry_count<?", false,
		models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER,
		models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL,
		models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE,
		models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE,
		models.CREDIT_CARD_NOTIFICATION_STATUS_PENDING,
		models.CREDIT_CARD_NOTIFICATION_STATUS_FAILED,
		minSendUnixTime,
		currentUnixTime,
		creditCardNotificationMaxRetryCount).OrderBy("send_unix_time asc").Limit(creditCardNotificationPageCount).Find(&notificationLogs)

	if err != nil {
		return err
	}

	if len(notificationLogs) < 1 {
		return nil
	}

	log.Infof(c, "[credit_card_notifications.sendNotifications] should send %d credit card notifications now", len(notificationLogs))

	for i := 0; i < len(notificationLogs); i++ {
		s.sendNotification(c, notificationLogs[i], currentUnixTime)
	}

	return nil
}

func (s *CreditCardNotificationService) sendNotification(c core.Context, notification *models.CreditCardNotificationLog, currentUnixTime int64) {
	account, err := s.accounts.GetAccountByAccountId(c, notification.Uid, notification.AccountId)

	if err != nil {
		log.Warnf(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" is not sent, because the account cannot be found", notification.NotificationType, notification.AccountId, notification.Uid)
		s.cancelNotification(c, notification, currentUnixTime, "account not found")
		return
	}

	if account.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD {
		log.Warnf(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" is not sent, because the account is not a credit card", notification.NotificationType, notification.AccountId, notification.Uid)
		s.cancelNotification(c, notification, currentUnixTime, "account is not a credit card")
		return
	}

	// sub-account cards are notified independently; pure containers are rejected as a second
	// line of defense in case a stale plan was created before the account was marked
	if !creditCardAccountIsEntity(account) {
		log.Warnf(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" is not sent, because the account is not a physical credit card", notification.NotificationType, notification.AccountId, notification.Uid)
		s.cancelNotification(c, notification, currentUnixTime, "account is not a physical credit card")
		return
	}

	if !s.notificationStillScheduled(account, notification) {
		log.Infof(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" is not sent, because the notification is no longer scheduled", notification.NotificationType, notification.AccountId, notification.Uid)
		s.cancelNotification(c, notification, currentUnixTime, "notification is no longer scheduled")
		return
	}

	user, err := s.users.GetUserById(c, notification.Uid)

	if err != nil || user.Email == "" || !user.EmailVerified || user.Disabled {
		reason := "user not found"

		if err == nil {
			reason = "user cannot receive emails"
		}

		log.Warnf(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" is not sent, because %s", notification.NotificationType, notification.AccountId, notification.Uid, reason)
		s.cancelNotification(c, notification, currentUnixTime, reason)
		return
	}

	message, err := s.buildEmailMessage(c, user, account, notification)

	if err != nil {
		log.Errorf(c, "[credit_card_notifications.sendNotification] failed to build credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\", because %s", notification.NotificationType, notification.AccountId, notification.Uid, err.Error())
		s.updateNotificationResult(c, notification, models.CREDIT_CARD_NOTIFICATION_STATUS_FAILED, err.Error(), currentUnixTime)
		return
	}

	err = s.SendMail(message)

	if err != nil {
		log.Errorf(c, "[credit_card_notifications.sendNotification] failed to send credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" to \"%s\", because %s", notification.NotificationType, notification.AccountId, notification.Uid, user.Email, err.Error())
		s.updateNotificationResult(c, notification, models.CREDIT_CARD_NOTIFICATION_STATUS_FAILED, err.Error(), currentUnixTime)
		return
	}

	log.Infof(c, "[credit_card_notifications.sendNotification] credit card notification \"type:%s\" for account \"id:%d\" of user \"uid:%d\" has been sent to \"%s\"", notification.NotificationType, notification.AccountId, notification.Uid, user.Email)
	s.updateNotificationResult(c, notification, models.CREDIT_CARD_NOTIFICATION_STATUS_SUCCESS, "", currentUnixTime)
}

// notificationStillScheduled returns whether the current settings of the account still produce
// this notification. The plans are recomputed for the scheduled send time itself, so a corrected
// statement date, a changed reminder time and a turned off switch all invalidate a notification
// which was planned before the change
func (s *CreditCardNotificationService) notificationStillScheduled(account *models.Account, notification *models.CreditCardNotificationLog) bool {
	planItems := s.buildNotificationPlans(account, notification.SendUnixTime, notification.SendUnixTime, notification.SendUnixTime)

	for i := 0; i < len(planItems); i++ {
		if planItems[i].NotificationType == notification.NotificationType && planItems[i].ScheduledDate == notification.ScheduledDate {
			return true
		}
	}

	return false
}

func (s *CreditCardNotificationService) buildEmailMessage(c core.Context, user *models.User, account *models.Account, notification *models.CreditCardNotificationLog) (*mail.MailMessage, error) {
	// GetLocaleTextItems returns the default locale text items for an unknown locale, so an empty
	// user language needs no special handling
	localeTextItems := locales.GetLocaleTextItems(user.Language)
	templateParams := map[string]any{
		"AppName":     localeTextItems.GlobalTextItems.AppName,
		"Salutation":  fmt.Sprintf("Hi, %s", user.Nickname),
		"AccountName": account.Name,
		"AccountUrl":  fmt.Sprintf(creditCardAccountListUrlFormat, s.CurrentConfig().RootUrl),
		"RootUrl":     s.CurrentConfig().RootUrl,
	}

	templateName := templates.TEMPLATE_CREDIT_CARD_STATEMENT_REMINDER
	subject := "Credit Card Statement Date Reminder"

	switch notification.NotificationType {
	case models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER:
		templateParams["StatementDate"] = utils.FormatNumericYearMonthDayToLongDate(notification.ScheduledDate)
	case models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL:
		templateName = templates.TEMPLATE_CREDIT_CARD_STATEMENT
		subject = fmt.Sprintf("Credit Card Statement - %s", account.Name)

		statementParams, err := s.buildStatementEmailParams(c, user, account, notification)

		if err != nil {
			return nil, err
		}

		for key, value := range statementParams {
			templateParams[key] = value
		}
	case models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE:
		templateName = templates.TEMPLATE_CREDIT_CARD_REPAYMENT_REMINDER
		subject = "Credit Card Repayment Date Reminder"

		location := creditCardNotificationLocation(account.Extend)
		repaymentTime := creditCardNotificationTimeFromNumericDate(notification.ScheduledDate, location)
		sendTime := time.Unix(notification.SendUnixTime, 0).In(location)
		daysBefore := utils.GetDaysBetweenDates(time.Date(sendTime.Year(), sendTime.Month(), sendTime.Day(), 0, 0, 0, 0, location), repaymentTime)

		if daysBefore < 0 {
			daysBefore = 0
		}

		templateParams["RepaymentDate"] = utils.FormatNumericYearMonthDayToLongDate(notification.ScheduledDate)
		templateParams["DaysBefore"] = daysBefore
	}

	templateParams["Title"] = subject
	body, err := s.renderEmail(templateName, templateParams)

	if err != nil {
		return nil, err
	}

	return &mail.MailMessage{
		To:      user.Email,
		Subject: subject,
		Body:    body,
	}, nil
}

func (s *CreditCardNotificationService) buildStatementEmailParams(c core.Context, user *models.User, account *models.Account, notification *models.CreditCardNotificationLog) (map[string]any, error) {
	extend := account.Extend
	location := creditCardNotificationLocation(extend)
	statementDate := creditCardNotificationIntPtrValue(extend.CreditCardStatementDate)
	includeStatementDate := creditCardNotificationBoolPtrValue(extend.CreditCardStatementDateIncludeTransaction)
	statementTime := creditCardNotificationTimeFromNumericDate(notification.ScheduledDate, location)
	minUnixTime, maxUnixTime := utils.GetStatementBillingCycleRangeForTime(statementTime.Unix(), statementDate, includeStatementDate, location)

	statement, err := s.transactions.GetAccountStatementByTimeRange(c, user.Uid, account.AccountId, account.Category, minUnixTime, maxUnixTime)

	if err != nil {
		return nil, err
	}

	allAccounts, err := s.accounts.GetAllAccountsByUid(c, user.Uid)

	if err != nil {
		return nil, err
	}

	allCategories, err := s.categories.GetAllCategoriesByUid(c, user.Uid, 0, -1)

	if err != nil {
		return nil, err
	}

	accountMap := s.accounts.GetAccountMapByList(allAccounts)
	categoryMap := s.categories.GetCategoryMapByList(allCategories)
	transactionItems := make([]map[string]any, len(statement.Transactions))

	for i := 0; i < len(statement.Transactions); i++ {
		statementTransaction := statement.Transactions[i]
		transactionLocation := time.FixedZone("Transaction Timezone", int(statementTransaction.TimezoneUtcOffset)*60)
		amount := "••••••"

		if !statementTransaction.HideAmount {
			amount = utils.FormatAmount(statementTransaction.Amount)
		}

		transactionItems[i] = map[string]any{
			"Time":        utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(statementTransaction.TransactionTime), transactionLocation),
			"Type":        statementTransaction.Type.String(),
			"Description": creditCardStatementTransactionDescription(statementTransaction, accountMap, categoryMap),
			"Comment":     statementTransaction.Comment,
			"Amount":      amount,
		}
	}

	return map[string]any{
		"CycleStartTime": utils.FormatUnixTimeToLongDate(minUnixTime, location),
		"CycleEndTime":   utils.FormatUnixTimeToLongDate(maxUnixTime, location),
		"Currency":       account.Currency,
		"OpeningBalance": utils.FormatBigIntAmount(statement.OpeningBalance),
		"TotalInflows":   utils.FormatBigIntAmount(statement.TotalInflows),
		"TotalOutflows":  utils.FormatBigIntAmount(statement.TotalOutflows),
		"ClosingBalance": utils.FormatBigIntAmount(statement.ClosingBalance),
		"Transactions":   transactionItems,
	}, nil
}

func (s *CreditCardNotificationService) renderEmail(templateName templates.KnownTemplate, templateParams map[string]any) (string, error) {
	tmpl, err := templates.GetTemplate(templateName)

	if err != nil {
		return "", err
	}

	var bodyBuffer bytes.Buffer

	if err := tmpl.Execute(&bodyBuffer, templateParams); err != nil {
		return "", err
	}

	return bodyBuffer.String(), nil
}

func (s *CreditCardNotificationService) cancelNotification(c core.Context, notification *models.CreditCardNotificationLog, currentUnixTime int64, reason string) {
	notification.Status = models.CREDIT_CARD_NOTIFICATION_STATUS_CANCELLED
	s.updateNotification(c, notification, reason, currentUnixTime)
}

func (s *CreditCardNotificationService) updateNotificationResult(c core.Context, notification *models.CreditCardNotificationLog, status models.CreditCardNotificationStatus, lastError string, currentUnixTime int64) {
	if status != models.CREDIT_CARD_NOTIFICATION_STATUS_SUCCESS {
		notification.RetryCount++
	}

	notification.Status = status
	s.updateNotification(c, notification, lastError, currentUnixTime)
}

func (s *CreditCardNotificationService) updateNotification(c core.Context, notification *models.CreditCardNotificationLog, lastError string, currentUnixTime int64) {
	db := s.UserDataDB(notification.Uid)
	updateModel := &models.CreditCardNotificationLog{
		Status:          notification.Status,
		RetryCount:      notification.RetryCount,
		LastError:       creditCardNotificationTrimLastError(lastError),
		UpdatedUnixTime: currentUnixTime,
	}

	cols := []string{"status", "retry_count", "last_error", "updated_unix_time"}

	if notification.Status == models.CREDIT_CARD_NOTIFICATION_STATUS_SUCCESS {
		cols = append(cols, "sent_unix_time")
		updateModel.SentUnixTime = currentUnixTime
	}

	_, err := db.NewSession(c).Cols(cols...).ID(notification.NotificationLogId).Update(updateModel)

	if err != nil {
		log.Errorf(c, "[credit_card_notifications.updateNotification] failed to update credit card notification \"id:%d\", because %s", notification.NotificationLogId, err.Error())
	}
}

func creditCardNotificationTrimLastError(lastError string) string {
	lastErrorRunes := []rune(lastError)

	if len(lastErrorRunes) > creditCardNotificationMaxLastErrorLength {
		return string(lastErrorRunes[:creditCardNotificationMaxLastErrorLength])
	}

	return lastError
}

func creditCardStatementTransactionDescription(statementTransaction *AccountStatementTransaction, accountMap map[int64]*models.Account, categoryMap map[int64]*models.TransactionCategory) string {
	switch statementTransaction.Type {
	case models.TRANSACTION_DB_TYPE_INCOME, models.TRANSACTION_DB_TYPE_EXPENSE:
		if category := categoryMap[statementTransaction.CategoryId]; category != nil {
			return category.Name
		}
	case models.TRANSACTION_DB_TYPE_TRANSFER_OUT, models.TRANSACTION_DB_TYPE_TRANSFER_IN:
		if relatedAccount := accountMap[statementTransaction.RelatedAccountId]; relatedAccount != nil {
			return relatedAccount.Name
		}
	}

	return ""
}

// creditCardAccountIsEntity returns whether the account itself represents a physical credit
// card. Accounts explicitly marked as non-entity cards (pure containers which only hold the
// shared limit of their sub-accounts) neither plan nor receive notifications. Legacy accounts
// without the flag are treated as entity cards to keep the previous behavior
func creditCardAccountIsEntity(account *models.Account) bool {
	if account == nil || account.Extend == nil {
		return true
	}

	if account.Extend.CreditCardIsEntityCard != nil && !*account.Extend.CreditCardIsEntityCard {
		return false
	}

	return true
}

// creditCardNotificationLocation returns the location used to calculate credit card notification
// times. The timezone name is checked when the account is saved, so the saved utc offset is only
// used as a fallback when the timezone database is not available on the current server
func creditCardNotificationLocation(extend *models.AccountExtend) *time.Location {
	if extend != nil && extend.CreditCardNotificationTimezoneName != nil && *extend.CreditCardNotificationTimezoneName != "" {
		location, err := time.LoadLocation(*extend.CreditCardNotificationTimezoneName)

		if err == nil {
			return location
		}
	}

	if extend != nil && extend.CreditCardNotificationTimezoneUtcOffset != nil {
		return time.FixedZone("Notification Timezone", int(*extend.CreditCardNotificationTimezoneUtcOffset)*60)
	}

	return time.Local
}

func creditCardNotificationTimeFromNumericDate(yearMonthDay int32, location *time.Location) time.Time {
	year := int(yearMonthDay / 10000)
	month := int(yearMonthDay % 10000 / 100)
	day := int(yearMonthDay % 100)

	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, location)
}

func creditCardNotificationBoolPtrValue(value *bool) bool {
	return value != nil && *value
}

func creditCardNotificationIntPtrValue(value *int) int {
	if value == nil {
		return 0
	}

	return *value
}
