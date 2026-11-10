package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testBillingZone = time.FixedZone("Test Timezone", 28800) // UTC+8

func testBillingTime(year int, month int, day int, hour int, minute int, second int) time.Time {
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, testBillingZone)
}

func TestAddMonthsClamped(t *testing.T) {
	// 2026 is not a leap year
	assert.Equal(t, testBillingTime(2026, 2, 28, 0, 0, 0), AddMonthsClamped(testBillingTime(2026, 1, 29, 0, 0, 0), 1))
	assert.Equal(t, testBillingTime(2026, 2, 28, 12, 30, 0), AddMonthsClamped(testBillingTime(2026, 1, 31, 12, 30, 0), 1))
	assert.Equal(t, testBillingTime(2028, 2, 29, 0, 0, 0), AddMonthsClamped(testBillingTime(2028, 1, 31, 0, 0, 0), 1))
	assert.Equal(t, testBillingTime(2025, 12, 10, 0, 0, 0), AddMonthsClamped(testBillingTime(2026, 1, 10, 0, 0, 0), -1))
	assert.Equal(t, testBillingTime(2026, 1, 28, 8, 0, 0), AddMonthsClamped(testBillingTime(2026, 2, 28, 8, 0, 0), -1))
}

func TestGetBillingCycleFirstTimeByUnixTimeExcludeStatementDate(t *testing.T) {
	// statement date is 10th, transactions on the statement date belong to the next bill
	unixTime := testBillingTime(2026, 11, 9, 12, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 10, 10, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, false, testBillingZone))

	unixTime = testBillingTime(2026, 11, 10, 0, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 10, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, false, testBillingZone))

	unixTime = testBillingTime(2026, 11, 10, 23, 59, 59).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 10, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, false, testBillingZone))

	// Mar 5th with statement date 28th belongs to the cycle started on Feb 28th
	unixTime = testBillingTime(2026, 3, 5, 8, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 2, 28, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 28, false, testBillingZone))
}

func TestGetBillingCycleLastTimeByUnixTimeExcludeStatementDate(t *testing.T) {
	unixTime := testBillingTime(2026, 11, 9, 12, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 9, 23, 59, 59), GetBillingCycleLastTimeByUnixTime(unixTime, 10, false, testBillingZone))

	unixTime = testBillingTime(2026, 11, 10, 15, 2, 29).Unix()
	assert.Equal(t, testBillingTime(2026, 12, 9, 23, 59, 59), GetBillingCycleLastTimeByUnixTime(unixTime, 10, false, testBillingZone))

	// cycle [Jan 28, Feb 27] for statement date 28th in a non-leap year
	unixTime = testBillingTime(2026, 2, 10, 12, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 2, 27, 23, 59, 59), GetBillingCycleLastTimeByUnixTime(unixTime, 28, false, testBillingZone))
}

func TestGetBillingCycleFirstTimeByUnixTimeIncludeStatementDate(t *testing.T) {
	// statement date is 10th, transactions on the statement date belong to the current bill
	unixTime := testBillingTime(2026, 11, 9, 12, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 10, 11, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, true, testBillingZone))

	unixTime = testBillingTime(2026, 11, 10, 9, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 10, 11, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, true, testBillingZone))

	unixTime = testBillingTime(2026, 11, 11, 0, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 11, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 10, true, testBillingZone))

	// Feb 28th statement in a non-leap year, same semantics with the frontend moment.js calculation
	unixTime = testBillingTime(2026, 2, 28, 9, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 1, 29, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 28, true, testBillingZone))

	// statement date 28th in March, one month back is Feb 28th in a non-leap year, so the cycle
	// starts from Mar 1st
	unixTime = testBillingTime(2026, 3, 5, 8, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 3, 1, 0, 0, 0), GetBillingCycleFirstTimeByUnixTime(unixTime, 28, true, testBillingZone))
}

func TestGetBillingCycleLastTimeByUnixTimeIncludeStatementDate(t *testing.T) {
	unixTime := testBillingTime(2026, 11, 9, 12, 0, 0).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 10, 23, 59, 59), GetBillingCycleLastTimeByUnixTime(unixTime, 10, true, testBillingZone))

	unixTime = testBillingTime(2026, 11, 10, 15, 2, 29).Unix()
	assert.Equal(t, testBillingTime(2026, 11, 10, 23, 59, 59), GetBillingCycleLastTimeByUnixTime(unixTime, 10, true, testBillingZone))
}

func TestGetStatementBillingCycleRangeForTime(t *testing.T) {
	// on the statement date (Nov 10) at 9:00, statement covers the just-finished cycle
	minUnixTime, maxUnixTime := GetStatementBillingCycleRangeForTime(testBillingTime(2026, 11, 10, 9, 0, 0).Unix(), 10, false, testBillingZone)
	assert.Equal(t, testBillingTime(2026, 10, 10, 0, 0, 0).Unix(), minUnixTime)
	assert.Equal(t, testBillingTime(2026, 11, 9, 23, 59, 59).Unix(), maxUnixTime)

	// including statement date, statement covers the cycle ending at the end of the statement date
	minUnixTime, maxUnixTime = GetStatementBillingCycleRangeForTime(testBillingTime(2026, 11, 10, 9, 0, 0).Unix(), 10, true, testBillingZone)
	assert.Equal(t, testBillingTime(2026, 10, 11, 0, 0, 0).Unix(), minUnixTime)
	assert.Equal(t, testBillingTime(2026, 11, 10, 23, 59, 59).Unix(), maxUnixTime)
}

func TestGetCreditCardRepaymentTimeByStatementTime(t *testing.T) {
	// fixed days after statement date
	statementTime := testBillingTime(2026, 11, 10, 9, 0, 0)
	repaymentTime, ok := GetCreditCardRepaymentTimeByStatementTime(statementTime, 1, 20, 0)
	assert.Equal(t, true, ok)
	assert.Equal(t, testBillingTime(2026, 11, 30, 0, 0, 0), repaymentTime)

	// fixed days after statement date crossing month boundary
	statementTime = testBillingTime(2026, 1, 28, 9, 0, 0)
	repaymentTime, ok = GetCreditCardRepaymentTimeByStatementTime(statementTime, 1, 20, 0)
	assert.Equal(t, true, ok)
	assert.Equal(t, testBillingTime(2026, 2, 17, 0, 0, 0), repaymentTime)

	// fixed day of month after the statement date
	statementTime = testBillingTime(2026, 11, 10, 9, 0, 0)
	repaymentTime, ok = GetCreditCardRepaymentTimeByStatementTime(statementTime, 2, 0, 25)
	assert.Equal(t, true, ok)
	assert.Equal(t, testBillingTime(2026, 11, 25, 0, 0, 0), repaymentTime)

	// fixed day of month before (or equal to) the statement date, repayment in next month
	repaymentTime, ok = GetCreditCardRepaymentTimeByStatementTime(testBillingTime(2026, 11, 10, 9, 0, 0), 2, 0, 5)
	assert.Equal(t, true, ok)
	assert.Equal(t, testBillingTime(2026, 12, 5, 0, 0, 0), repaymentTime)

	repaymentTime, ok = GetCreditCardRepaymentTimeByStatementTime(testBillingTime(2026, 11, 10, 9, 0, 0), 2, 0, 10)
	assert.Equal(t, true, ok)
	assert.Equal(t, testBillingTime(2026, 12, 10, 0, 0, 0), repaymentTime)

	// not set or invalid parameters
	_, ok = GetCreditCardRepaymentTimeByStatementTime(testBillingTime(2026, 11, 10, 9, 0, 0), 0, 0, 0)
	assert.Equal(t, false, ok)

	_, ok = GetCreditCardRepaymentTimeByStatementTime(testBillingTime(2026, 11, 10, 9, 0, 0), 1, 0, 0)
	assert.Equal(t, false, ok)

	_, ok = GetCreditCardRepaymentTimeByStatementTime(testBillingTime(2026, 11, 10, 9, 0, 0), 2, 0, 0)
	assert.Equal(t, false, ok)
}
