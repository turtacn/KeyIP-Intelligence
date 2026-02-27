package repositories

import (
	"context"
	"database/sql"
)

// queryExecutor abstracts sql.DB and sql.Tx
type queryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// scanner abstracts sql.Row and sql.Rows
type scanner interface {
	Scan(dest ...interface{}) error
}
