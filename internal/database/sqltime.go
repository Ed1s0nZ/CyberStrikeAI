package database

import (
	"errors"
	"strings"
	"time"
)

// formatSQLiteUTC stores instants as UTC RFC3339 for consistent SQLite reads/writes.
func formatSQLiteUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// sqliteEpochGE returns SQL comparing column to param as Unix seconds (timezone-safe).
func sqliteEpochGE(column, op string) string {
	return "strftime('%s', " + column + ") " + op + " strftime('%s', ?)"
}

// ParseRFC3339Time parses API/query timestamps (RFC3339 or RFC3339Nano).
func ParseRFC3339Time(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty time value")
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
