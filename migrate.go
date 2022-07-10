package dbmigrate

import (
	"database/sql"
	"fmt"
)

const (
	// DialectSQLite is constant for SQLite dialect name
	DialectSQLite string = "sqlite3"
	// DialectMySQL is constant for MySQL dialect name
	DialectMySQL string = "mysql"
)

// Querier is interface for making database queries
type Querier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// MigrationFuncType is a type of function that does "functional" changes in every migration
type MigrationFuncType func(c Querier) error

// Migration represents database migration meta information
type Migration struct {
	Previous string
	ID       string

	Desc string

	Stmt        string
	PreUpgrade  MigrationFuncType
	PostUpgrade MigrationFuncType
	DisableFK   bool
}

// UpgradeToLatest upgrades database to latest migration
func UpgradeToLatest(db *sql.DB, dialect string, migrations ...Migration) error {
	return Upgrade(db, dialect, "", migrations...)
}

// Upgrade upgrades database to given target migration
func Upgrade(db *sql.DB, dialect string, targetMigration string, migrations ...Migration) error {
	ds := newDialectSupp(dialect)

	if err := ds.ensureMigrationLog(db); err != nil {
		return err
	}
	if err := ds.ensureFKChecksEnabled(db); err != nil {
		return err
	}
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	mm, err := findMigrationsToApply(migrations, applied, targetMigration)
	if err != nil {
		return err
	}

	if len(mm) == 0 {
		return nil
	}

	for _, m := range mm {
		if mErr := applyMigration(db, ds, *m); mErr != nil {
			return fmt.Errorf("migration %q failed: %w", m.ID, mErr)
		}
	}
	return nil
}
