package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCreditCardRepaymentCycleByDate(t *testing.T) {
	tests := []struct {
		name          string
		extend        *models.AccountExtend
		scheduledDate int32
		wantStatement int32
		wantValid     bool
	}{
		{
			name: "days after statement in same month",
			extend: testNotificationAccountExtend(&models.AccountExtend{
				CreditCardStatementDate:               testNotificationInt(10),
				CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
				CreditCardRepaymentDateType:           testNotificationInt(1),
				CreditCardRepaymentDaysAfterStatement: testNotificationInt(18),
			}),
			scheduledDate: 20261128,
			wantStatement: 20261110,
			wantValid:     true,
		},
		{
			name: "fixed day of following month",
			extend: testNotificationAccountExtend(&models.AccountExtend{
				CreditCardStatementDate:            testNotificationInt(10),
				CreditCardRepaymentReminderEnabled: testNotificationBool(true),
				CreditCardRepaymentDateType:        testNotificationInt(2),
				CreditCardRepaymentDayOfMonth:      testNotificationInt(5),
			}),
			scheduledDate: 20261105,
			wantStatement: 20261010,
			wantValid:     true,
		},
		{
			name: "reminder disabled",
			extend: testNotificationAccountExtend(&models.AccountExtend{
				CreditCardStatementDate:               testNotificationInt(10),
				CreditCardRepaymentReminderEnabled:    testNotificationBool(false),
				CreditCardRepaymentDateType:           testNotificationInt(1),
				CreditCardRepaymentDaysAfterStatement: testNotificationInt(18),
			}),
			scheduledDate: 20261128,
			wantValid:     false,
		},
		{
			name: "date does not belong to any cycle",
			extend: testNotificationAccountExtend(&models.AccountExtend{
				CreditCardStatementDate:               testNotificationInt(10),
				CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
				CreditCardRepaymentDateType:           testNotificationInt(1),
				CreditCardRepaymentDaysAfterStatement: testNotificationInt(18),
			}),
			scheduledDate: 20261129,
			wantValid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := testNotificationPlanAccount(tt.extend)
			cycle, ok := CreditCardNotifications.creditCardRepaymentCycleByDate(account, tt.scheduledDate)
			assert.Equal(t, tt.wantValid, ok)

			if tt.wantValid {
				assert.Equal(t, tt.scheduledDate, cycle.ScheduledDate)
				assert.Equal(t, tt.wantStatement, cycle.StatementDate)
			}
		})
	}
}

func TestCurrentCreditCardRepaymentCycle(t *testing.T) {
	account := testNotificationPlanAccount(testNotificationAccountExtend(&models.AccountExtend{
		CreditCardStatementDate:               testNotificationInt(10),
		CreditCardRepaymentReminderEnabled:    testNotificationBool(true),
		CreditCardRepaymentDateType:           testNotificationInt(1),
		CreditCardRepaymentDaysAfterStatement: testNotificationInt(18),
	}))

	t.Run("returns upcoming repayment date", func(t *testing.T) {
		cycle, ok := CreditCardNotifications.currentCreditCardRepaymentCycle(account, testNotificationTime(2026, time.November, 25, 10, 0).Unix())
		assert.True(t, ok)
		assert.Equal(t, int32(20261128), cycle.ScheduledDate)
		assert.Equal(t, int32(20261110), cycle.StatementDate)
	})

	t.Run("moves to next cycle after repayment date", func(t *testing.T) {
		cycle, ok := CreditCardNotifications.currentCreditCardRepaymentCycle(account, testNotificationTime(2026, time.November, 29, 10, 0).Unix())
		assert.True(t, ok)
		assert.Equal(t, int32(20261228), cycle.ScheduledDate)
		assert.Equal(t, int32(20261210), cycle.StatementDate)
	})
}

func TestCreditCardRepaymentNumericDateToTime(t *testing.T) {
	tests := []struct {
		name      string
		value     int32
		wantValid bool
	}{
		{name: "valid date", value: 20261128, wantValid: true},
		{name: "invalid month", value: 20261301, wantValid: false},
		{name: "invalid day", value: 20260230, wantValid: false},
		{name: "zero value", value: 0, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := creditCardRepaymentNumericDateToTime(tt.value, testNotificationZone)
			assert.Equal(t, tt.wantValid, ok)
		})
	}
}

func TestIsCreditCardRepaymentNotificationType(t *testing.T) {
	assert.True(t, isCreditCardRepaymentNotificationType(models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_ADVANCE))
	assert.True(t, isCreditCardRepaymentNotificationType(models.CREDIT_CARD_NOTIFICATION_TYPE_REPAYMENT_DUE))
	assert.False(t, isCreditCardRepaymentNotificationType(models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_REMINDER))
	assert.False(t, isCreditCardRepaymentNotificationType(models.CREDIT_CARD_NOTIFICATION_TYPE_STATEMENT_EMAIL))
}
