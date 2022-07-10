package dbmigrate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type dialectSupp interface {
	ensureMigrationLog(q Querier) error
	ensureFKChecksEnabled(q Querier) error
	enableFKs(q Querier) error
	disableFKs(q Querier) error
	saveMigrationToLog(q Querier, migrationID, desc string) error
}

func newDialectSupp(dialect string) dialectSupp {
	switch dialect {
	case DialectSQLite:
		return &sqliteSupp{}
	case DialectMySQL:
		return &mysqlSupp{}
	}
	panic(fmt.Errorf("unsupported dialect: %q", dialect))
}

func ensureMigrationsHistoryTableExists(q Querier) error {
	_, err := q.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id TEXT NOT NULL,
			desc TEXT NOT NULL,
			at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("creating migration table: %w", err)
	}
	return nil
}

func getAppliedMigrations(q Querier) ([]string, error) {
	rows, err := q.Query(`SELECT id FROM migrations`)
	if err != nil {
		return nil, err
	}
	res := make([]string, 0)
	_, err = scanEachRow(rows, func(s scanner) error {
		id := ""
		if scanErr := s.Scan(&id); scanErr != nil {
			return scanErr
		}
		res = append(res, id)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func wrapTx(tx *sql.Tx, fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = rollbackErr
			}
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				err = commitErr
			}
		}
	}()
	return fn()
}

func withTx(dbConn *sql.DB, fn func(q Querier) error) error {
	tx, err := dbConn.Begin()
	if err != nil {
		return err
	}
	return wrapTx(tx, func() error { return fn(tx) })
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanEachRow(rows *sql.Rows, scanRow func(s scanner) error) (rowsProcessed int, err error) {
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		err = scanRow(rows)
		if err != nil {
			return 0, fmt.Errorf("scan row: %w", err)
		}
		count++
	}
	if err = rows.Err(); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("rows scan: %w", err)
	}
	return count, nil
}

func findMigrationsToApply(migrations []Migration, applied []string, targetID string) ([]*Migration, error) {
	mMap, rmMap := make(map[string]*Migration, len(migrations)), make(map[string]string, len(migrations))
	aMap := make(map[string]struct{}, len(applied))

	for _, a := range applied {
		aMap[a] = struct{}{}
	}
	appliedFound := len(aMap) == 0

	var initialID *string
	for i := range migrations {
		m := migrations[i]

		if m.ID == "" {
			return nil, errors.New("migration ID cannot be empty")
		}

		if !appliedFound {
			if _, found := aMap[m.Previous]; found {
				appliedFound = true
			}
		}

		if prev, found := mMap[m.ID]; found {
			return nil, fmt.Errorf("migration with ID %q already exists, parent migration %q", m.ID, prev.Previous)
		}

		if curID, found := rmMap[m.Previous]; found {
			return nil, fmt.Errorf("migration with ID %q already exists, next migration %q", m.Previous, curID)
		}
		mMap[m.ID] = &m
		rmMap[m.Previous] = m.ID
	}

	for i := range migrations {
		m := migrations[i]
		if _, found := mMap[m.Previous]; !found {
			if initialID != nil {
				return nil, fmt.Errorf("migration %q has invalid previous migration ID %q", m.ID, m.Previous)
			}
			s := m.Previous
			initialID = &s
		}
	}

	if !appliedFound {
		return nil, errors.New("provided migration does not correlate with already applied migrations")
	}
	if targetID != "" {
		if _, found := mMap[targetID]; !found {
			return nil, fmt.Errorf("target migration with ID %q not found", targetID)
		}
	}

	ordered := make([]*Migration, 0, len(migrations))
	for {
		mID, found := rmMap[*initialID]
		if !found {
			break
		}
		m := mMap[mID]
		ordered = append(ordered, m)

		initialID = &mID
	}

	latestApplied := -1
	target := len(ordered)
	for i, m := range ordered {
		if _, found := aMap[m.ID]; found {
			latestApplied = i
		}
		if m.ID == targetID {
			target = i + 1
		}
	}
	if target-1 <= latestApplied {
		return nil, nil
	}
	return ordered[latestApplied+1 : target], nil
}

func applyMigration(dbConn *sql.DB, ds dialectSupp, m Migration) error {
	if m.DisableFK {
		if err := ds.disableFKs(dbConn); err != nil {
			return err
		}
	}

	err := withTx(dbConn, func(q Querier) error {
		if m.PreUpgrade != nil {
			if err := m.PreUpgrade(q); err != nil {
				return fmt.Errorf("migration %q, pre-upgrade method failed: %v", m.ID, err)
			}
		}

		statements := strings.Split(m.Stmt, ";")
		for _, stmt := range statements {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := q.Exec(stmt); err != nil {
				return fmt.Errorf("migration %q failed: %w", m.ID, err)
			}
		}

		if m.PostUpgrade != nil {
			if err := m.PostUpgrade(q); err != nil {
				return fmt.Errorf("migration %q, post-upgrade method failed: %v", m.ID, err)
			}
		}
		if err := ds.saveMigrationToLog(q, m.ID, m.Desc); err != nil {
			return err
		}
		return nil
	})

	if m.DisableFK {
		if fkErr := ds.enableFKs(dbConn); fkErr != nil {
			return fkErr
		}
	}
	return err
}
