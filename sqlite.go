package dbmigrate

import (
	"fmt"
)

type sqliteSupp struct {
}

func (s *sqliteSupp) ensureMigrationLog(q Querier) error {
	return ensureMigrationsHistoryTableExists(q)
}

func (s *sqliteSupp) ensureFKChecksEnabled(q Querier) error {
	_, err := q.Exec(`PRAGMA foreign_keys = ON;`)
	if err != nil {
		return fmt.Errorf("sqlite: error enabling foreign keys: %w", err)
	}
	return nil
}

func (s *sqliteSupp) enableFKs(q Querier) error {
	_, err := q.Exec(`PRAGMA foreign_keys = ON; PRAGMA legacy_alter_table = OFF;`)
	if err != nil {
		return fmt.Errorf("sqlite: disable FK constraints: %w", err)
	}
	return nil
}

func (s *sqliteSupp) disableFKs(q Querier) error {
	_, err := q.Exec(`PRAGMA legacy_alter_table = ON; PRAGMA foreign_keys = OFF;`)
	if err != nil {
		return fmt.Errorf("sqlite: disable FK constraints: %w", err)
	}
	return nil
}

func (s *sqliteSupp) saveMigrationToLog(q Querier, migrationID, desc string) error {
	if _, err := q.Exec(
		`INSERT INTO migrations (desc, id, at) VALUES (?, ?, DATE('now'));`, desc, migrationID,
	); err != nil {
		return fmt.Errorf("sqlite: save migration info to db: %w", err)
	}
	return nil
}
