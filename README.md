# dbmigrate

Database migrations in golang, with support for pre- and post- migration hooks

Example:
```
import "github.com/vlanse/dbmigrate"

...

mm := []dbmigrate.Migration{
    {
        ID:   "migration-uniq-id-1",
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
        Previous: "migration-uniq-id-1",
        ID:       "migration-uniq-id-2",
        Desc:     "migration #2",
        Stmt: `
            ALTER TABLE test ADD id2 INTEGER DEFAULT 0;
            `,
        DisableFK: true,
    },
}

if err := dbmigrate.UpgradeToLatest(db, DialectSQLite, mm...)); err != nil {
	...
}
```