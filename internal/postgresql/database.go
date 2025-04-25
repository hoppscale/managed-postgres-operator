package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Database struct {
	Name  string `db:"datname"`
	Owner string `db:"owner"`

	Extensions []string `db:"-"`
}

const GetDatabaseSQLStatement = "SELECT d.datname, pg_catalog.pg_get_userbyid(d.datdba) as owner FROM pg_catalog.pg_database d WHERE d.datname = $1"

func GetDatabase(pgpool PGPoolInterface, name string) (database *Database, err error) {
	rows, err := pgpool.Query(context.Background(), GetDatabaseSQLStatement, name)
	if err != nil {
		err = fmt.Errorf("pg query failed: %s", err)
		return
	}
	defer rows.Close()

	databases, err := pgx.CollectRows(rows, pgx.RowToStructByName[Database])
	if err != nil {
		err = fmt.Errorf("failed to collect rows: %s", err)
		return
	}

	if len(databases) > 1 {
		err = fmt.Errorf("wrong number of rows returned, expected 1, got %d", len(databases))
		return
	}

	if len(databases) == 0 {
		return
	}

	database = &databases[0]
	return
}

func CreateDatabase(pgpool PGPoolInterface, database string) (err error) {
	sanitizedName := pgx.Identifier{database}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to create database: %s", err)
	}
	return
}

func DropDatabase(pgpool PGPoolInterface, database string) (err error) {
	sanitizedName := pgx.Identifier{database}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("DROP DATABASE %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to drop database: %s", err)
	}
	return
}

func AlterDatabaseOwner(pgpool PGPoolInterface, database, owner string) (err error) {
	sanitizedDatabaseName := pgx.Identifier{database}.Sanitize()
	sanitizedOwnerName := pgx.Identifier{owner}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("ALTER DATABASE %s OWNER TO %s", sanitizedDatabaseName, sanitizedOwnerName))
	if err != nil {
		return fmt.Errorf("failed to alter database owner: %s", err)
	}
	return
}

func GetExtensions(pgpool PGPoolInterface) (extensions []string, err error) {
	rows, err := pgpool.Query(context.Background(), "SELECT extname FROM pg_extension")
	if err != nil {
		err = fmt.Errorf("pg query failed: %s", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var extension string
		err := rows.Scan(&extension)
		if err != nil {
			return nil, fmt.Errorf("failed to read rows: %s", err)
		}
		extensions = append(extensions, extension)
	}
	return
}

func CreateExtension(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("CREATE EXTENSION %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to create extension: %s", err)
	}
	return
}

func DropExtension(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("DROP EXTENSION %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to drop extension: %s", err)
	}
	return
}

func DropDatabaseConnections(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to drop database connections: %s", err)
	}
	return
}
