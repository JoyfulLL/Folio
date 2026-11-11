package models

// DailyReportLogStatus represents the sending status of a daily report
type DailyReportLogStatus byte

// Daily report log statuses
const (
	// DAILY_REPORT_LOG_STATUS_PENDING means the report has been planned but has not been sent
	// successfully yet; failed reports stay pending for the next retry
	DAILY_REPORT_LOG_STATUS_PENDING DailyReportLogStatus = 0
	DAILY_REPORT_LOG_STATUS_SUCCESS DailyReportLogStatus = 1
	DAILY_REPORT_LOG_STATUS_FAILED  DailyReportLogStatus = 2
)

// DailyReportLog represents one daily report of one user, stored in database. One row stands
// for one report of one user on one local reporting date, so the unique index of uid and
// report date keeps every report sent only once even if the job runs on multiple instances
type DailyReportLog struct {
	ReportLogId     int64                `xorm:"PK"`
	Uid             int64                `xorm:"UNIQUE(UQE_daily_report_log_uid_report_date) NOT NULL"`
	ReportDate      int32                `xorm:"UNIQUE(UQE_daily_report_log_uid_report_date) NOT NULL"`
	Status          DailyReportLogStatus `xorm:"NOT NULL"`
	RetryCount      int16                `xorm:"NOT NULL"`
	LastError       string               `xorm:"VARCHAR(255) NOT NULL"`
	SentUnixTime    int64
	CreatedUnixTime int64
	UpdatedUnixTime int64
}
