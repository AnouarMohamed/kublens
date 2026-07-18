package db

import (
	"fmt"
	"strings"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

func NormalizeDialect(driver string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", string(DialectSQLite):
		return DialectSQLite, nil
	case string(DialectPostgres), "pgx":
		return DialectPostgres, nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", strings.TrimSpace(driver))
	}
}

func (d Dialect) Bind(query string) string {
	if d != DialectPostgres {
		return query
	}

	var builder strings.Builder
	builder.Grow(len(query) + 8)
	index := 1
	for _, char := range query {
		if char == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

func (d Dialect) DriverName() string {
	if d == DialectPostgres {
		return "pgx"
	}
	return "sqlite"
}

func (d Dialect) RuntimeName() string {
	if d == DialectPostgres {
		return string(DialectPostgres)
	}
	return string(DialectSQLite)
}
