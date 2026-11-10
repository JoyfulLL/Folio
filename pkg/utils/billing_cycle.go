package utils

import (
	"time"
)

// daysInMonth returns the count of days of the specified year and month (month is 1-12)
func daysInMonth(year int, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
			return 29
		}

		return 28
	}

	return 30
}

// AddMonthsClamped returns the time shifted by the specified month count, clamping the day-of-month
// to the target month length, keeping the wall-clock time and location. It keeps the same semantics
// with moment.js `add(n, 'months')`, which is different with `time.Time.AddDate`
func AddMonthsClamped(t time.Time, months int) time.Time {
	year, month, day := t.Date()
	hour, minute, second := t.Clock()

	totalMonth := int(year)*12 + int(month) - 1 + months
	newYear := totalMonth / 12
	newMonth := totalMonth%12 + 1

	if newMonth < 1 { // normalize for negative remainder, never happens when totalMonth >= 0
		newMonth += 12
		newYear--
	}

	maxDay := daysInMonth(newYear, newMonth)

	if day > maxDay {
		day = maxDay
	}

	return time.Date(newYear, time.Month(newMonth), day, hour, minute, second, 0, t.Location())
}

// GetBillingCycleFirstTimeByUnixTime returns the first time of the billing cycle which contains the
// specified unix time in loc. statementDate is the day-of-month (1-28) of the billing statement date.
// If includeStatementDate is true, transactions on the statement date are included in the current
// billing cycle, so a cycle starts from the next day of the statement date; otherwise a cycle starts
// from the statement date, and transactions on the statement date belong to the next billing cycle
func GetBillingCycleFirstTimeByUnixTime(unixTime int64, statementDate int, includeStatementDate bool, loc *time.Location) time.Time {
	t := time.Unix(unixTime, 0).In(loc)
	year, month, day := t.Date()

	if includeStatementDate {
		if day > statementDate {
			return time.Date(year, month, statementDate+1, 0, 0, 0, 0, loc)
		}

		return AddMonthsClamped(time.Date(year, month, statementDate, 0, 0, 0, 0, loc), -1).AddDate(0, 0, 1)
	}

	if day >= statementDate {
		return time.Date(year, month, statementDate, 0, 0, 0, 0, loc)
	}

	return AddMonthsClamped(time.Date(year, month, statementDate, 0, 0, 0, 0, loc), -1)
}

// GetBillingCycleFirstUnixTimeByUnixTime returns the unix time of the first time of the billing
// cycle which contains the specified unix time
func GetBillingCycleFirstUnixTimeByUnixTime(unixTime int64, statementDate int, includeStatementDate bool, loc *time.Location) int64 {
	return GetBillingCycleFirstTimeByUnixTime(unixTime, statementDate, includeStatementDate, loc).Unix()
}

// GetBillingCycleLastTimeByUnixTime returns the last time (23:59:59 of the day before the first time
// of the next billing cycle) of the billing cycle which contains the specified unix time
func GetBillingCycleLastTimeByUnixTime(unixTime int64, statementDate int, includeStatementDate bool, loc *time.Location) time.Time {
	return AddMonthsClamped(GetBillingCycleFirstTimeByUnixTime(unixTime, statementDate, includeStatementDate, loc), 1).Add(-time.Second)
}

// GetBillingCycleLastUnixTimeByUnixTime returns the unix time of the last time of the billing cycle
// which contains the specified unix time
func GetBillingCycleLastUnixTimeByUnixTime(unixTime int64, statementDate int, includeStatementDate bool, loc *time.Location) int64 {
	return GetBillingCycleLastTimeByUnixTime(unixTime, statementDate, includeStatementDate, loc).Unix()
}

// GetStatementBillingCycleRangeForTime returns the unix time range of the billing cycle whose
// statement is being issued at the specified time in loc. The specified time should be on the
// statement date (day-of-month equals to statementDate).
//
// If includeStatementDate is false, transactions on the statement date belong to the next billing
// cycle, so the issued statement covers the cycle which has just finished at the end of yesterday;
// if includeStatementDate is true, the issued statement covers the cycle ending at the end of the
// statement date (which contains transactions already made before the notification time)
func GetStatementBillingCycleRangeForTime(unixTime int64, statementDate int, includeStatementDate bool, loc *time.Location) (int64, int64) {
	cycleFirstTime := GetBillingCycleFirstTimeByUnixTime(unixTime, statementDate, includeStatementDate, loc)

	if !includeStatementDate {
		cycleFirstTime = AddMonthsClamped(cycleFirstTime, -1)
	}

	cycleLastTime := AddMonthsClamped(cycleFirstTime, 1).Add(-time.Second)

	return cycleFirstTime.Unix(), cycleLastTime.Unix()
}

// GetCreditCardRepaymentTimeByStatementTime returns the final repayment time of a credit card bill
// issued at the end of the specified statement time day. repaymentDateType is 0 (not set), 1
// (fixed days after statement date) or 2 (fixed day of month); daysAfterStatement is valid when
// repaymentDateType is 1 (1-99); repaymentDayOfMonth is valid when repaymentDateType is 2 (1-28).
// It returns false when the repayment date is not set
func GetCreditCardRepaymentTimeByStatementTime(statementTime time.Time, repaymentDateType int, daysAfterStatement int, repaymentDayOfMonth int) (time.Time, bool) {
	location := statementTime.Location()
	year, month, day := statementTime.Date()
	firstDayOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, location)
	statementDayStart := time.Date(year, month, day, 0, 0, 0, 0, location)

	switch repaymentDateType {
	case 1:
		if daysAfterStatement < 1 {
			return statementDayStart, false
		}

		repaymentTime := statementDayStart.AddDate(0, 0, daysAfterStatement)
		return time.Date(repaymentTime.Year(), repaymentTime.Month(), repaymentTime.Day(), 0, 0, 0, 0, location), true
	case 2:
		if repaymentDayOfMonth < 1 {
			return statementDayStart, false
		}

		var repaymentTime time.Time

		if repaymentDayOfMonth > day {
			repaymentTime = firstDayOfMonth.AddDate(0, 0, repaymentDayOfMonth-1)
		} else {
			repaymentTime = firstDayOfMonth.AddDate(0, 1, 0).AddDate(0, 0, repaymentDayOfMonth-1)
		}

		return time.Date(repaymentTime.Year(), repaymentTime.Month(), repaymentTime.Day(), 0, 0, 0, 0, location), true
	}

	return statementDayStart, false
}
