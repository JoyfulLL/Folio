package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidCreditCardNotificationInterval(t *testing.T) {
	for _, interval := range []int16{15, 30, 60, 180, 360, 720} {
		assert.True(t, IsValidCreditCardNotificationInterval(interval))
	}

	for _, interval := range []int16{-15, 0, 1, 14, 16, 45, 90, 120, 721, 1440} {
		assert.False(t, IsValidCreditCardNotificationInterval(interval))
	}
}

func TestGetValidCreditCardNotificationInterval(t *testing.T) {
	assert.Equal(t, CREDIT_CARD_NOTIFICATION_DEFAULT_INTERVAL, GetValidCreditCardNotificationInterval(0))
	assert.Equal(t, CREDIT_CARD_NOTIFICATION_DEFAULT_INTERVAL, GetValidCreditCardNotificationInterval(45))
	assert.Equal(t, int16(15), GetValidCreditCardNotificationInterval(15))
	assert.Equal(t, int16(720), GetValidCreditCardNotificationInterval(720))
}

func TestUserNeedCreditCardNotificationScan(t *testing.T) {
	const currentUnixTime = int64(1800000000)

	// a user who has never been scanned is due, whatever interval is saved
	assert.True(t, (&User{}).NeedCreditCardNotificationScan(currentUnixTime))
	assert.True(t, (&User{CreditCardNotificationInterval: 720}).NeedCreditCardNotificationScan(currentUnixTime))

	user := &User{
		CreditCardNotificationInterval:         15,
		CreditCardNotificationLastScanUnixTime: currentUnixTime - 14*60,
	}
	assert.False(t, user.NeedCreditCardNotificationScan(currentUnixTime))

	user.CreditCardNotificationLastScanUnixTime = currentUnixTime - 15*60
	assert.True(t, user.NeedCreditCardNotificationScan(currentUnixTime))

	// an interval which was never saved falls back to the shortest one
	user = &User{
		CreditCardNotificationInterval:         0,
		CreditCardNotificationLastScanUnixTime: currentUnixTime - 15*60,
	}
	assert.True(t, user.NeedCreditCardNotificationScan(currentUnixTime))

	user = &User{
		CreditCardNotificationInterval:         720,
		CreditCardNotificationLastScanUnixTime: currentUnixTime - 11*3600,
	}
	assert.False(t, user.NeedCreditCardNotificationScan(currentUnixTime))

	user.CreditCardNotificationLastScanUnixTime = currentUnixTime - 12*3600
	assert.True(t, user.NeedCreditCardNotificationScan(currentUnixTime))

	// saving a credit card account resets the scan time, so the user is due on the next tick
	user.CreditCardNotificationLastScanUnixTime = 0
	assert.True(t, user.NeedCreditCardNotificationScan(currentUnixTime))
}
