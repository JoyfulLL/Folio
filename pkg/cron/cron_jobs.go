package cron

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// RemoveExpiredTokensJob represents the cron job which periodically remove expired user tokens from the database
var RemoveExpiredTokensJob = &CronJob{
	Name:        "RemoveExpiredTokens",
	Description: "Periodically remove expired user tokens from the database.",
	Period: CronJobFixedHourPeriod{
		Hour: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Tokens.DeleteAllExpiredTokens(c)
	},
}

// CreateScheduledTransactionJob represents the cron job which periodically create transaction by scheduled transaction template
var CreateScheduledTransactionJob = &CronJob{
	Name:        "CreateScheduledTransaction",
	Description: "Periodically create transaction by scheduled transaction template.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Transactions.CreateScheduledTransactions(c, time.Now().Unix(), c.GetInterval())
	},
}

// CreditCardNotificationJob represents the cron job which periodically plans and sends credit card notifications by email
var CreditCardNotificationJob = &CronJob{
	Name:        "CreditCardNotification",
	Description: "Periodically plan and send credit card statement and repayment date notifications by email.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.CreditCardNotifications.RunCreditCardNotifications(c, time.Now().Unix())
	},
}

// DailyReportJob represents the cron job which periodically sends the daily income and expense report to the Feishu webhook
var DailyReportJob = &CronJob{
	Name:        "DailyReport",
	Description: "Periodically send the daily income and expense report to the Feishu webhook.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.DailyReports.RunDailyReports(c, time.Now().Unix())
	},
}
