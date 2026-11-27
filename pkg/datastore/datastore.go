package datastore

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
)

// DataStore represents a data storage containing a series of database shards
type DataStore struct {
	databases []Database
}

// Count returns total count of database instances
func (s *DataStore) Count() int {
	return len(s.databases)
}

// Get returns a database instance by index
func (s *DataStore) Get(index int) Database {
	return s.databases[index]
}

// Choose returns a database instance by sharding key
func (s *DataStore) Choose(key int64) Database {
	return s.databases[0]
}

// Query returns a new database session in a specific database by sharding key
func (s *DataStore) Query(c core.Context, key int64) DatabaseSession {
	return s.Choose(key).NewSession(c)
}

// DoTransaction runs a new database transaction in a specific database by sharding key
func (s *DataStore) DoTransaction(key int64, c core.Context, fn func(sess DatabaseSession) error) (err error) {
	return s.Choose(key).DoTransaction(c, fn)
}

// SyncStructs updates database structs by database models
func (s *DataStore) SyncStructs(beans ...any) error {
	var err error

	for i := 0; i < len(s.databases); i++ {
		err = s.databases[i].SyncStructs(beans...)

		if err != nil {
			return err
		}
	}

	return err
}

// GetTableNames returns all distinct physical table names across all database shards
func (s *DataStore) GetTableNames() ([]string, error) {
	names := make([]string, 0)
	seen := make(map[string]bool)

	for _, database := range s.databases {
		xormDatabase, ok := database.(*XOrmDatabase)
		if !ok {
			return nil, errs.ErrDatabaseOperationFailed
		}

		metas, err := xormDatabase.engineGroup.DBMetas()
		if err != nil {
			return nil, err
		}

		for _, meta := range metas {
			if seen[meta.Name] {
				continue
			}

			seen[meta.Name] = true
			names = append(names, meta.Name)
		}
	}

	return names, nil
}

// GetTableName returns the physical table name mapped from the given bean
func (s *DataStore) GetTableName(bean any) (string, error) {
	if len(s.databases) < 1 {
		return "", errs.ErrDatabaseIsNull
	}

	xormDatabase, ok := s.databases[0].(*XOrmDatabase)
	if !ok {
		return "", errs.ErrDatabaseOperationFailed
	}

	return xormDatabase.engineGroup.TableName(bean), nil
}

// NewDataStore returns a new data storage by a series of database
func NewDataStore(databases ...Database) (*DataStore, error) {
	if len(databases) < 1 {
		return nil, errs.ErrDatabaseIsNull
	}

	return &DataStore{
		databases: databases,
	}, nil
}
