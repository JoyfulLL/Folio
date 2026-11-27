package cmd

import (
	"sort"

	"github.com/urfave/cli/v3"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// databaseTableRegistration binds a model to the store that maintains its table
type databaseTableRegistration struct {
	store *datastore.DataStore
	bean  any
	label string
}

// getAllDatabaseTableRegistrations returns every table maintained by this build
func getAllDatabaseTableRegistrations() []databaseTableRegistration {
	return []databaseTableRegistration{
		{datastore.Container.UserStore, new(models.User), "user"},
		{datastore.Container.UserStore, new(models.TwoFactor), "two-factor"},
		{datastore.Container.UserStore, new(models.TwoFactorRecoveryCode), "two-factor recovery code"},
		{datastore.Container.TokenStore, new(models.TokenRecord), "token record"},
		{datastore.Container.UserDataStore, new(models.Account), "account"},
		{datastore.Container.UserDataStore, new(models.Transaction), "transaction"},
		{datastore.Container.UserDataStore, new(models.TransactionCategory), "transaction category"},
		{datastore.Container.UserDataStore, new(models.TransactionTagGroup), "transaction tag group"},
		{datastore.Container.UserDataStore, new(models.TransactionTag), "transaction tag"},
		{datastore.Container.UserDataStore, new(models.TransactionTagIndex), "transaction tag index"},
		{datastore.Container.UserDataStore, new(models.TransactionTemplate), "transaction template"},
		{datastore.Container.UserDataStore, new(models.TransactionPictureInfo), "transaction picture"},
		{datastore.Container.UserDataStore, new(models.UserCustomIcon), "user custom icon"},
		{datastore.Container.UserDataStore, new(models.ImportPreset), "user import preset"},
		{datastore.Container.UserDataStore, new(models.UserCustomExchangeRate), "user custom exchange rate"},
		{datastore.Container.UserDataStore, new(models.UserApplicationCloudSetting), "user application cloud settings"},
		{datastore.Container.UserDataStore, new(models.UserExternalAuth), "user external auth"},
		{datastore.Container.UserDataStore, new(models.InsightsExplorer), "insights explorer"},
		{datastore.Container.UserDataStore, new(models.CreditCardNotificationLog), "credit card notification log"},
		{datastore.Container.UserDataStore, new(models.CreditCardRepaymentStatus), "credit card repayment status"},
		{datastore.Container.UserDataStore, new(models.DailyReportLog), "daily report log"},
		{datastore.Container.UserDataStore, new(models.UserDailyReportSetting), "user daily report setting"},
	}
}

// Database represents the database command
var Database = &cli.Command{
	Name:  "database",
	Usage: "Folio database maintenance",
	Commands: []*cli.Command{
		{
			Name:   "update",
			Usage:  "Update database structure",
			Action: bindAction(updateDatabaseStructure),
		},
		{
			Name:   "check",
			Usage:  "Report drift between code models and physical tables (read-only)",
			Action: bindAction(checkDatabaseStructure),
		},
	},
}

func updateDatabaseStructure(c *core.CliContext) error {
	_, err := initializeSystem(c)

	if err != nil {
		return err
	}

	log.CliInfof(c, "[database.updateDatabaseStructure] starting maintaining")

	err = updateAllDatabaseTablesStructure(c)

	if err != nil {
		log.CliErrorf(c, "[database.updateDatabaseStructure] update database table structure failed, because %s", err.Error())
		return err
	}

	log.CliInfof(c, "[database.updateDatabaseStructure] all tables maintained successfully")
	return nil
}

func checkDatabaseStructure(c *core.CliContext) error {
	_, err := initializeSystem(c)

	if err != nil {
		return err
	}

	log.CliInfof(c, "[database.checkDatabaseStructure] starting read-only structure check")

	registrations := getAllDatabaseTableRegistrations()

	// The user, token and user-data stores may share one physical database, so
	// compare the union of all physical tables against all registered models.
	unmanaged := make(map[string]bool)
	checkedStores := make(map[*datastore.DataStore]bool)

	for _, registration := range registrations {
		if checkedStores[registration.store] {
			continue
		}

		checkedStores[registration.store] = true

		physicalNames, err := registration.store.GetTableNames()
		if err != nil {
			log.CliErrorf(c, "[database.checkDatabaseStructure] failed to read physical table names, because %s", err.Error())
			return err
		}

		for _, name := range physicalNames {
			unmanaged[name] = true
		}
	}

	for _, item := range registrations {
		name, err := item.store.GetTableName(item.bean)
		if err != nil {
			log.CliErrorf(c, "[database.checkDatabaseStructure] failed to resolve table name of %s, because %s", item.label, err.Error())
			return err
		}

		if !unmanaged[name] {
			log.CliWarnf(c, "[database.checkDatabaseStructure] table %s is declared in code but missing in database; run \"folio database update\"", name)
			continue
		}

		delete(unmanaged, name)
	}

	remaining := make([]string, 0, len(unmanaged))
	for name := range unmanaged {
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)

	for _, name := range remaining {
		log.CliWarnf(c, "[database.checkDatabaseStructure] table %s exists in database but is no longer managed by code; review before removing it", name)
	}

	log.CliInfof(c, "[database.checkDatabaseStructure] structure check finished")
	return nil
}

func updateAllDatabaseTablesStructure(c *core.CliContext) error {
	for _, registration := range getAllDatabaseTableRegistrations() {
		if err := registration.store.SyncStructs(registration.bean); err != nil {
			return err
		}

		log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] %s table maintained successfully", registration.label)
	}

	return nil
}
