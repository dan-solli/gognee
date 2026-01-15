package store

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCGODriver(t *testing.T) {
	EnableSQLiteVec()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var sqliteVersion string
	var vecVersion string
	err = db.QueryRow("select sqlite_version(), vec_version()").Scan(&sqliteVersion, &vecVersion)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("sqlite_version=%s, vec_version=%s", sqliteVersion, vecVersion)

	if vecVersion == "" {
		t.Fatal("vec_version() returned empty")
	}
}
