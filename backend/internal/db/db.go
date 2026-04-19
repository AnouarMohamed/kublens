package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
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
	if err := Migrate(ctx, handle); err != nil {
		_ = handle.Close()
		return nil, err
	}
	if err := handle.PingContext(ctx); err != nil {
		_ = handle.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return handle, nil
}

func Migrate(ctx context.Context, handle *sql.DB) error {
	if handle == nil {
		return fmt.Errorf("sqlite database handle is nil")
	}

	for _, stmt := range schemaStatements {
		if _, err := handle.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply sqlite schema: %w", err)
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
