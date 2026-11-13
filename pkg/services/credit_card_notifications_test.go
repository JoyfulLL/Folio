package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

var testNotificationZone = time.FixedZone("Test Timezone", 480*60)
var testNotificationUtcOffset = int16(480)

func testNotificationTime(year int, month time.Month, day int, hour int, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, testNotificationZone)
}

func testNotificationPlanAccount(extend *models.AccountExtend) *models.Account {
	return &models.Account{
		AccountId:       10001,
		Uid:             10000,
		Category:        models.ACCOUNT_CATEGORY_CREDIT_CARD,
		ParentAccountId: models.LevelOneAccountParentId,
		Extend:          extend,
	}
}

func testNotificationAccountExtend(fields *models.AccountExtend) *models.AccountExtend {
	extend := *fields
	extend.CreditCardNotificationTimezoneUtcOffset = &testNotificationUtcOffset

	if extend.CreditCardNotificationMinutesOfDay == nil {
		minutesOfDay := 9 * 60
		extend.CreditCardNotificationMinutesOfDay = &minutesOfDay
	}

	return &extend
}

func testNotificationBool(value bool) *bool {
	return &value
}

func testNotificationInt(value int) *int {
	return &value
}

func testNotificationString(value string) *string {
	return &value
}

func testNotificationPlanWindow(currentUnixTime int64) (int64, int64) {
	// These unit tests deliberately exercise a narrow window. The production reconciliation
	// window is much wider (see creditCardNotificationBackfillDuration).
	return currentUnixTime - int64((24 * time.Hour).Seconds()), currentUnixTime + int64((26 * time.Hour).Seconds())
}

func TestBuildNotificationPlansStatementNotifications(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
		CreditCardStatementEmailEnabled:    testNotificationBool(true),
	}))

	// transactions on the statement date belong to the next billing cycle, so both the reminder
	// and the statement email are sent on the statement date
	currentUnixTime := testNotificationTime(2026, time.November, 10, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 2)

	for i := 0; i < len(planItems); i++ {
		assert.Equal(t, account.Uid, planItems[i].Uid)
		assert.Equal(t, account.AccountId, planItems[i].AccountId)
		assert.Equal(t, int32(20261110), planItems[i].ScheduledDate)
		assert.Equal(t, testNotificationTime(2026, time.November, 10, 9, 0).Unix(), planItems[i].SendUnixTime)
	}

	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER, planItems[0].NotificationType)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL, planItems[1].NotificationType)
}

func TestBuildNotificationPlansStatementEmailWithStatementDateTransactions(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:                   testNotificationInt(10),
		CreditCardStatementEmailEnabled:           testNotificationBool(true),
		CreditCardStatementDateIncludeTransaction: testNotificationBool(true),
	}))

	// transactions on the statement date belong to the current billing cycle, which is not
	// finished until the end of the statement date, so the statement email is sent one day later
	currentUnixTime := testNotificationTime(2026, time.November, 11, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL, planItems[0].NotificationType)
	assert.Equal(t, int32(20261110), planItems[0].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.November, 11, 9, 0).Unix(), planItems[0].SendUnixTime)

	statementTime := time.Date(2026, time.November, 10, 0, 0, 0, 0, testNotificationZone)
	minUnixTime, maxUnixTime := utils.GetStatementBillingCycleRangeForTime(statementTime.Unix(), 10, true, testNotificationZone)
	assert.Equal(t, testNotificationTime(2026, time.October, 11, 0, 0).Unix(), minUnixTime)
	assert.Equal(t, testNotificationTime(2026, time.November, 11, 0, 0).Unix()-1, maxUnixTime)
}

func TestBuildNotificationPlansRepaymentNotifications(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(10),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentReminderDaysBefore: testNotificationInt(3),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(20),
	}))

	// the repayment date is 20 days after the statement date, so it is 2026-11-30 and only the
	// advance reminder of it is in the plan window
	currentUnixTime := testNotificationTime(2026, time.November, 27, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, planItems[0].NotificationType)
	assert.Equal(t, int32(20261130), planItems[0].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.November, 27, 9, 0).Unix(), planItems[0].SendUnixTime)

	// on the repayment date only the final repayment reminder is in the plan window
	currentUnixTime = testNotificationTime(2026, time.November, 30, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime = testNotificationPlanWindow(currentUnixTime)
	planItems = CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE, planItems[0].NotificationType)
	assert.Equal(t, int32(20261130), planItems[0].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.November, 30, 9, 0).Unix(), planItems[0].SendUnixTime)
}

func TestBuildNotificationPlansRepaymentDayOfMonth(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardRepaymentReminderEnabled: testNotificationBool(true),
		CreditCardRepaymentDateType:        testNotificationInt(2),
		CreditCardRepaymentDayOfMonth:      testNotificationInt(5),
	}))

	// the repayment day is earlier than the statement day, so the repayment date of the November
	// billing cycle is in the next month
	currentUnixTime := testNotificationTime(2026, time.November, 10, 10, 0).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)

	currentUnixTime = testNotificationTime(2026, time.December, 5, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime = testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE, planItems[0].NotificationType)
	assert.Equal(t, int32(20261205), planItems[0].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.December, 5, 9, 0).Unix(), planItems[0].SendUnixTime)
}

func TestBuildNotificationPlansSkipsInvalidSettings(t *testing.T) {
	// no notification is enabled
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate: testNotificationInt(10),
	}))

	currentUnixTime := testNotificationTime(2026, time.November, 10, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)

	// the statement date is not set, so the statement and repayment dates cannot be calculated
	account = testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(20),
	}))
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)

	// the repayment date is not set, so only the statement reminder is planned
	account = testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
		CreditCardRepaymentReminderEnabled: testNotificationBool(true),
		CreditCardRepaymentDateType:        testNotificationInt(0),
	}))
	currentUnixTime = testNotificationTime(2026, time.November, 10, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime = testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER, planItems[0].NotificationType)
	assert.Equal(t, testNotificationTime(2026, time.November, 10, 9, 0).Unix(), planItems[0].SendUnixTime)
}

func TestBuildNotificationPlansSkipsNotificationsOutsideWindow(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(10),
		CreditCardStatementReminderEnabled:    testNotificationBool(true),
		CreditCardStatementEmailEnabled:       testNotificationBool(true),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentReminderDaysBefore: testNotificationInt(1),
		CreditCardRepaymentDateType:           testNotificationInt(2),
		CreditCardRepaymentDayOfMonth:         testNotificationInt(1),
	}))

	// both the statement date and the repayment date are out of the plan window
	currentUnixTime := testNotificationTime(2026, time.November, 20, 9, 0).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)

	// the advance reminder and the final reminder of the same repayment date are both planned
	currentUnixTime = testNotificationTime(2026, time.November, 30, 9, 30).Unix()
	minSendUnixTime, maxSendUnixTime = testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 2)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE, planItems[0].NotificationType)
	assert.Equal(t, int32(20261201), planItems[0].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.December, 1, 9, 0).Unix(), planItems[0].SendUnixTime)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE, planItems[1].NotificationType)
	assert.Equal(t, int32(20261201), planItems[1].ScheduledDate)
	assert.Equal(t, testNotificationTime(2026, time.November, 30, 9, 0).Unix(), planItems[1].SendUnixTime)
}

func TestBuildNotificationPlansSkipsPureContainerAccounts(t *testing.T) {
	// a main account explicitly marked as a pure container (non-entity card) plans nothing,
	// even when it still has billing settings saved
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(10),
		CreditCardStatementReminderEnabled:    testNotificationBool(true),
		CreditCardStatementEmailEnabled:       testNotificationBool(true),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(20),
		CreditCardIsEntityCard:                testNotificationBool(false),
	}))

	currentUnixTime := testNotificationTime(2026, time.November, 10, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)

	// the send side also rejects pure containers as a second line of defense
	assert.False(t, creditCardAccountIsEntity(account))

	// an account explicitly marked as an entity card plans notifications as usual
	account = testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
		CreditCardIsEntityCard:             testNotificationBool(true),
	}))
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER, planItems[0].NotificationType)
}

func TestBuildNotificationPlansSubAccountCards(t *testing.T) {
	// sub-account cards are planned independently with their own statement dates, so cards
	// sharing one limit pool but with different statement dates are notified separately
	subAccount := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(15),
		CreditCardStatementReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(20),
	}))
	subAccount.ParentAccountId = 10000

	assert.True(t, creditCardAccountIsEntity(subAccount))

	// the statement reminder of the sub-account card is planned on its own statement date
	currentUnixTime := testNotificationTime(2026, time.November, 15, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(subAccount, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, subAccount.AccountId, planItems[0].AccountId)
	assert.Equal(t, models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER, planItems[0].NotificationType)
	assert.Equal(t, int32(20261115), planItems[0].ScheduledDate)

	// a second sub-account card of the same pool with a different statement date is planned
	// on its own date and never inherits the other card's schedule
	otherSubAccount := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(25),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
	}))
	otherSubAccount.AccountId = 10002
	otherSubAccount.ParentAccountId = 10000

	currentUnixTime = testNotificationTime(2026, time.November, 25, 8, 30).Unix()
	minSendUnixTime, maxSendUnixTime = testNotificationPlanWindow(currentUnixTime)
	planItems = CreditCardNotifications.buildNotificationPlans(otherSubAccount, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 1)
	assert.Equal(t, otherSubAccount.AccountId, planItems[0].AccountId)
	assert.Equal(t, int32(20261125), planItems[0].ScheduledDate)
}

func TestCreditCardNotificationLocation(t *testing.T) {
	// a saved timezone name is used when it can be loaded
	location := creditCardNotificationLocation(&models.AccountExtend{
		CreditCardNotificationTimezoneName: testNotificationString("UTC"),
	})
	assert.Equal(t, time.UTC, location)

	// an unknown timezone name falls back to the saved utc offset
	location = creditCardNotificationLocation(&models.AccountExtend{
		CreditCardNotificationTimezoneName:      testNotificationString("Unknown/Timezone"),
		CreditCardNotificationTimezoneUtcOffset: &testNotificationUtcOffset,
	})
	_, utcOffset := time.Unix(testNotificationTime(2026, time.November, 10, 9, 0).Unix(), 0).In(location).Zone()
	assert.Equal(t, 480*60, utcOffset)

	// the server timezone is used when nothing is saved
	assert.Equal(t, time.Local, creditCardNotificationLocation(&models.AccountExtend{}))
	assert.Equal(t, time.Local, creditCardNotificationLocation(nil))
}

func TestBuildNotificationPlansCorrectedStatementDateSameDay(t *testing.T) {
	// the statement date was wrongly saved as the 12th and corrected to the 10th in the evening,
	// so the notifications of the current day are still planned although the reminder time has
	// already passed
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
		CreditCardStatementEmailEnabled:    testNotificationBool(true),
		CreditCardNotificationMinutesOfDay: testNotificationInt(11 * 60),
	}))

	currentUnixTime := testNotificationTime(2026, time.November, 10, 20, 0).Unix()
	minSendUnixTime, maxSendUnixTime := testNotificationPlanWindow(currentUnixTime)
	planItems := CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime)
	assert.Len(t, planItems, 2)

	for i := 0; i < len(planItems); i++ {
		assert.Equal(t, int32(20261110), planItems[i].ScheduledDate)
		assert.Equal(t, testNotificationTime(2026, time.November, 10, 11, 0).Unix(), planItems[i].SendUnixTime)
	}

	// the wrongly saved 12th never produces a notification on the 10th
	account.Extend.CreditCardStatementDate = testNotificationInt(12)
	assert.Len(t, CreditCardNotifications.buildNotificationPlans(account, currentUnixTime, minSendUnixTime, maxSendUnixTime), 0)
}

func TestNotificationStillScheduledStatementNotifications(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:            testNotificationInt(10),
		CreditCardStatementReminderEnabled: testNotificationBool(true),
		CreditCardStatementEmailEnabled:    testNotificationBool(true),
	}))

	notification := &models.CreditCardNotificationLog{
		NotificationType: models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER,
		ScheduledDate:    int32(20261110),
		SendUnixTime:     testNotificationTime(2026, time.November, 10, 9, 0).Unix(),
	}

	assert.True(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	// turning the reminder off invalidates the notification which was already planned
	account.Extend.CreditCardStatementReminderEnabled = testNotificationBool(false)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	// correcting the statement date invalidates the notification of the wrong day
	account.Extend.CreditCardStatementReminderEnabled = testNotificationBool(true)
	account.Extend.CreditCardStatementDate = testNotificationInt(12)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	// changing the reminder time invalidates the old send time, and the statement email of the
	// same day is scheduled at the new time independently of the reminder
	account.Extend.CreditCardStatementDate = testNotificationInt(10)
	account.Extend.CreditCardNotificationMinutesOfDay = testNotificationInt(11 * 60)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	emailNotification := &models.CreditCardNotificationLog{
		NotificationType: models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL,
		ScheduledDate:    int32(20261110),
		SendUnixTime:     testNotificationTime(2026, time.November, 10, 11, 0).Unix(),
	}
	assert.True(t, CreditCardNotifications.notificationStillScheduled(account, emailNotification))

	// a pure container never has a notification of its own
	account.Extend.CreditCardIsEntityCard = testNotificationBool(false)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, emailNotification))
}

func TestNotificationStillScheduledStatementEmailAcrossMonthBoundary(t *testing.T) {
	// the statement email of february 28th is sent on march 1st when the transactions of the
	// statement date belong to the billed cycle, and recomputing the plans from that send time
	// still finds the statement date of the previous month
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:                   testNotificationInt(28),
		CreditCardStatementEmailEnabled:           testNotificationBool(true),
		CreditCardStatementDateIncludeTransaction: testNotificationBool(true),
	}))

	notification := &models.CreditCardNotificationLog{
		NotificationType: models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL,
		ScheduledDate:    int32(20270228),
		SendUnixTime:     testNotificationTime(2027, time.March, 1, 9, 0).Unix(),
	}

	assert.True(t, CreditCardNotifications.notificationStillScheduled(account, notification))
}

func TestNotificationStillScheduledRepaymentNotification(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(10),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(18),
	}))

	notification := &models.CreditCardNotificationLog{
		NotificationType: models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE,
		ScheduledDate:    int32(20261128),
		SendUnixTime:     testNotificationTime(2026, time.November, 28, 9, 0).Unix(),
	}

	assert.True(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	// moving the repayment date invalidates the notification of the old repayment day
	account.Extend.CreditCardRepaymentDaysAfterStatement = testNotificationInt(20)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, notification))

	// turning the repayment reminder off invalidates it as well
	account.Extend.CreditCardRepaymentDaysAfterStatement = testNotificationInt(18)
	account.Extend.CreditCardRepaymentReminderEnabled = testNotificationBool(false)
	assert.False(t, CreditCardNotifications.notificationStillScheduled(account, notification))
}
