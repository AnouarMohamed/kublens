package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Config struct {
	Driver         string
	URL            string
	SQLitePath     string
	MigrationsAuto bool
}

func Open(ctx context.Context, path string) (*sql.DB, error) {
	handle, _, err := OpenDatabase(ctx, Config{
		Driver:         string(DialectSQLite),
		SQLitePath:     path,
		MigrationsAuto: true,
	})
	return handle, err
}

func OpenDatabase(ctx context.Context, cfg Config) (*sql.DB, Dialect, error) {
	dialect, err := NormalizeDialect(cfg.Driver)
	if err != nil {
		return nil, "", err
	}
	if dialect == DialectPostgres {
		handle, err := openPostgres(ctx, cfg.URL, cfg.MigrationsAuto)
		return handle, dialect, err
	}
	handle, err := openSQLite(ctx, cfg.SQLitePath, cfg.MigrationsAuto)
	return handle, dialect, err
}

func openSQLite(ctx context.Context, path string, migrate bool) (*sql.DB, error) {
	dbPath := strings.TrimSpace(path)
	if dbPath == "" {
		dbPath = "data/kubelens.db"
	}

	if err := ensureParentDir(dbPath); err != nil {
		return nil, err
	}

	handle, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if _, err := handle.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := handle.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if migrate {
		if err := Migrate(ctx, handle); err != nil {
			_ = handle.Close()
			return nil, err
		}
	}
	if err := handle.PingContext(ctx); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return handle, nil
}

func openPostgres(ctx context.Context, databaseURL string, migrate bool) (*sql.DB, error) {
	url := strings.TrimSpace(databaseURL)
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL is required when DATABASE_DRIVER=postgres")
	}

	handle, err := sql.Open(DialectPostgres.DriverName(), url)
	if err != nil {
		return nil, fmt.Errorf("open postgres database: %w", err)
	}
	if migrate {
		if err := Migrate(ctx, handle); err != nil {
			_ = handle.Close()
			return nil, err
		}
	}
	if err := handle.PingContext(ctx); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping postgres database: %w", err)
	}

	return handle, nil
}

func Migrate(ctx context.Context, handle *sql.DB) error {
	if handle == nil {
		return fmt.Errorf("database handle is nil")
	}

	for _, stmt := range schemaStatements {
		if _, err := handle.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply database schema: %w", err)
		}
	}

	return nil
}

func ensureParentDir(path string) error {
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") {
		return nil
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory %s: %w", dir, err)
	}
	return nil
}
