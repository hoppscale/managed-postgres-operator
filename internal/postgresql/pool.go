package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGPools struct {
	Default   PGPoolInterface
	Databases map[string]PGPoolInterface
}

type PGPoolInterface interface {
	Acquire(ctx context.Context) (c *pgxpool.Conn, err error)
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
	Close()
	Config() *pgxpool.Config
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func EnsurePGPoolExists(pgpools *PGPools, database string) (err error) {
	// Check if pool already exists
	if _, ok := pgpools.Databases[database]; ok {
		return
	}

	// Create new pool
	config, err := pgxpool.ParseConfig("")
	if err != nil {
		err = fmt.Errorf("failed to initialize empty config: %s", err)
		return
	}

	if pgpools.Default == nil {
		err = fmt.Errorf("failed to retrieve default database connection config")
		return
	}

	config.ConnConfig = pgpools.Default.Config().ConnConfig
	config.ConnConfig.Database = database

	pgpools.Databases[database], err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		err = fmt.Errorf("failed to open pool with config: %s", err)
		return
	}

	return
}
