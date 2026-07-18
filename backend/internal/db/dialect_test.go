package db

import (
	"context"
	"testing"
)

func TestDialectBindPostgres(t *testing.T) {
	query := DialectPostgres.Bind("SELECT * FROM incidents WHERE id = ? AND status = ?")
	want := "SELECT * FROM incidents WHERE id = $1 AND status = $2"
	if query != want {
		t.Fatalf("bound query = %q, want %q", query, want)
	}
}

func TestOpenSQLiteDatabaseWithMigrations(t *testing.T) {
	handle, dialect, err := OpenDatabase(context.Background(), Config{
		Driver:         "sqlite",
		SQLitePath:     ":memory:",
		MigrationsAuto: true,
	})
	if err != nil {
		t.Fatalf("OpenDatabase() error = %v", err)
	}
	defer handle.Close()
	if dialect != DialectSQLite {
		t.Fatalf("dialect = %q, want sqlite", dialect)
	}

	var count int
	if err := handle.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM incidents").Scan(&count); err != nil {
		t.Fatalf("query migrated incidents table: %v", err)
	}
}
