package dbmigrate

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

type migrationSuite struct {
	suite.Suite
}

func TestMigrate(t *testing.T) {
	suite.Run(t, &migrationSuite{})
}

func (s *migrationSuite) TestMigrationDetection() {
	tests := []struct {
		name     string
		mm       []Migration
		applied  []string
		expOrder []string
		target   string
		expError bool
	}{
		{
			name: "ok, basic case, initial migration with empty ID",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "4"},
			},
			applied:  nil,
			expError: false,
			expOrder: []string{"1", "2", "3", "4"},
		},
		{
			name: "ok, arbitrary order, but still valid",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "3", ID: "4"},
				{Previous: "2", ID: "3"},
			},
			applied:  nil,
			expError: false,
			expOrder: []string{"1", "2", "3", "4"},
		},
		{
			name: "ok, with some migrations applied",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "4"},
				{Previous: "4", ID: "5"},
			},
			applied:  []string{"2"},
			expError: false,
			expOrder: []string{"3", "4", "5"},
		},
		{
			name: "ok, schema is up to date, nothing to apply",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "4"},
				{Previous: "4", ID: "5"},
			},
			applied:  []string{"1", "2", "3", "4", "5"},
			expError: false,
			expOrder: []string{},
		},
		{
			name: "ok, initial migration has some previous ID, it is possibly consolidated from previous ones",
			mm: []Migration{
				{Previous: "some earlier migration ID", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
			},
			applied:  nil,
			expError: false,
			expOrder: []string{"1", "2", "3"},
		},
		{
			name: "ok, some migrations applied, target specified",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "4"},
				{Previous: "4", ID: "5"},
				{Previous: "5", ID: "6"},
			},
			applied:  []string{"1", "2", "3"},
			target:   "5",
			expError: false,
			expOrder: []string{"4", "5"},
		},
		{
			name: "ok, some migrations applied, target specified as last",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "4"},
				{Previous: "4", ID: "5"},
				{Previous: "5", ID: "6"},
			},
			applied:  []string{"1", "2", "3"},
			target:   "6",
			expError: false,
			expOrder: []string{"4", "5", "6"},
		},
		{
			name:     "ok, single applied",
			mm:       []Migration{{ID: "1"}},
			applied:  []string{"1"},
			expOrder: []string{},
			expError: false,
		},
		{
			name: "ok, target already applied",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
			},
			applied:  []string{"1", "2"},
			target:   "2",
			expError: false,
			expOrder: []string{},
		},
		{
			name:     "error, empty ID",
			mm:       []Migration{{}},
			expError: true,
		},
		{
			name: "error, duplicate ID",
			mm: []Migration{
				{ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "1"},
			},
			expError: true,
		},
		{
			name: "error, duplicate prev migration ID",
			mm: []Migration{
				{ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "1", ID: "3"},
			},
			expError: true,
		},
		{
			name: "error, cycle",
			mm: []Migration{
				{ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
				{Previous: "3", ID: "1"},
			},
			expError: true,
		},
		{
			name: "error, multiple not existing initial migrations",
			mm: []Migration{
				{Previous: "initial 1", ID: "1"},
				{Previous: "initial 2", ID: "2"},
			},
			expError: true,
		},
		{
			name: "error, no initial",
			mm: []Migration{
				{Previous: "2", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "1"},
			},
			expError: true,
		},
		{
			name: "error, no corresponding applied migration detected",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
			},
			applied:  []string{"4", "5"},
			expError: true,
		},
		{
			name: "error, invalid target",
			mm: []Migration{
				{Previous: "", ID: "1"},
				{Previous: "1", ID: "2"},
				{Previous: "2", ID: "3"},
			},
			target:   "4",
			expError: true,
		},
	}
	for _, t := range tests {
		s.Run(t.name, func() {
			m, err := findMigrationsToApply(t.mm, t.applied, t.target)
			if t.expError {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				ids := make([]string, 0, len(m))
				for _, mi := range m {
					ids = append(ids, mi.ID)
				}
				s.Require().ElementsMatch(t.expOrder, ids)
			}
		})
	}
}

func (s *migrationSuite) TestActualMigration() {
	db, err := sql.Open("sqlite", ":memory:")
	s.Require().NoError(err)

	mm := []Migration{
		{
			ID:   "1",
			Desc: "initial",
			Stmt: `
				CREATE TABLE test
				(
					id INTEGER PRIMARY KEY AUTOINCREMENT
				);
				CREATE TABLE test2
				(
					id INTEGER PRIMARY KEY AUTOINCREMENT
				);
				`,
		},
		{
			Previous: "1",
			ID:       "2",
			Desc:     "migration #2",
			Stmt: `
				ALTER TABLE test ADD id2 INTEGER DEFAULT 0;
				`,
			DisableFK: true,
		},
	}

	s.Require().NoError(Upgrade(db, DialectSQLite, "1", mm...))
	s.Require().NoError(UpgradeToLatest(db, DialectSQLite, mm...))
}
