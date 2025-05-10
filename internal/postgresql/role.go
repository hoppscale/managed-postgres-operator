package postgresql

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Role struct {
	Name        string `db:"rolname"`
	SuperUser   bool   `db:"rolsuper"`
	Inherit     bool   `db:"rolinherit"`
	CreateRole  bool   `db:"rolcreaterole"`
	CreateDB    bool   `db:"rolcreatedb"`
	Login       bool   `db:"rolcanlogin"`
	Replication bool   `db:"rolreplication"`
	BypassRLS   bool   `db:"rolbypassrls"`

	Password string `db:"-"`
}

const GetRoleSQLStatement = "SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = $1"

func GetRole(pgpool PGPoolInterface, name string) (role *Role, err error) {
	rows, err := pgpool.Query(context.Background(), GetRoleSQLStatement, name)
	if err != nil {
		err = fmt.Errorf("pg query failed: %s", err)
		return
	}
	defer rows.Close()

	roles, err := pgx.CollectRows(rows, pgx.RowToStructByName[Role])
	if err != nil {
		err = fmt.Errorf("failed to collect rows: %s", err)
		return
	}

	if len(roles) > 1 {
		err = fmt.Errorf("wrong number of rows returned, expected 1, got %d", len(roles))
		return
	}

	if len(roles) == 0 {
		return
	}

	role = &roles[0]
	return
}

func CreateRole(pgpool PGPoolInterface, role *Role) (err error) {
	sanitizedName := pgx.Identifier{role.Name}.Sanitize()
	options := generateRoleOptionsString(role)
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("CREATE ROLE %s %s", sanitizedName, options))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}

func generateRoleOptionsString(role *Role) string {
	s := "WITH "

	if role.SuperUser {
		s += "SUPERUSER "
	} else {
		s += "NOSUPERUSER "
	}

	if role.Inherit {
		s += "INHERIT "
	} else {
		s += "NOINHERIT "
	}

	if role.CreateRole {
		s += "CREATEROLE "
	} else {
		s += "NOCREATEROLE "
	}

	if role.CreateDB {
		s += "CREATEDB "
	} else {
		s += "NOCREATEDB "
	}

	if role.Login {
		s += "LOGIN "
	} else {
		s += "NOLOGIN "
	}

	if role.Replication {
		s += "REPLICATION "
	} else {
		s += "NOREPLICATION "
	}

	if role.BypassRLS {
		s += "BYPASSRLS "
	} else {
		s += "NOBYPASSRLS "
	}

	if role.Password != "" {
		s += fmt.Sprintf("PASSWORD '%s' ", strings.Replace(role.Password, "'", "''", -1))
	}

	return s
}

func DropRole(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("DROP ROLE %s", sanitizedName))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}

func AlterRole(pgpool PGPoolInterface, role *Role) (err error) {
	sanitizedName := pgx.Identifier{role.Name}.Sanitize()
	options := generateRoleOptionsString(role)
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("ALTER ROLE %s %s", sanitizedName, options))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}
