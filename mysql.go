package dbmigrate

import (
	"fmt"
)

type mysqlSupp struct {
}

func (s *mysqlSupp) ensureMigrationLog(q Querier) error {
	return ensureMigrationsHistoryTableExists(q)
}

func (s *mysqlSupp) ensureFKChecksEnabled(_ Querier) error {
	return nil
}

func (s *mysqlSupp) enableFKs(_ Querier) error {
	return nil
}

func (s *mysqlSupp) disableFKs(_ Querier) error {
	return nil
}

func (s *mysqlSupp) saveMigrationToLog(q Querier, migrationID, desc string) error {
	query := `INSERT INTO migrations (desc, id, at) VALUES (?, ?, NOW());`
	if _, err := q.Exec(query, desc, migrationID); err != nil {
		return fmt.Errorf("mysql: save migration info to db: %w", err)
	}
	return nil
}
