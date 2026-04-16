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

func CreateRole(pgpool PGPoolInterface, operatorRole, role *Role) (err error) {
	sanitizedName := pgx.Identifier{role.Name}.Sanitize()

	options, err := generateRoleOptionsString(operatorRole, &Role{}, role)
	if err != nil {
		return err
	}

	options += fmt.Sprintf("ADMIN %s", pgx.Identifier{operatorRole.Name}.Sanitize())

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("CREATE ROLE %s %s", sanitizedName, options))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}

func generateRoleOptionsString(operatorRole, existingRole, desiredRole *Role) (options string, err error) {
	rawOptions := "WITH "

	if existingRole.SuperUser != desiredRole.SuperUser {
		if !operatorRole.SuperUser {
			err = fmt.Errorf("cannot set SUPERUSER option: the operator's role must have SUPERUSER option")
			return "", err
		}

		if desiredRole.SuperUser {
			rawOptions += "SUPERUSER "
		} else {
			rawOptions += "NOSUPERUSER "
		}
	}

	if existingRole.Inherit != desiredRole.Inherit {
		if !operatorRole.Inherit {
			err = fmt.Errorf("cannot set INHERIT option: the operator's role must have INHERIT option")
			return "", err
		}

		if desiredRole.Inherit {
			rawOptions += "INHERIT "
		} else {
			rawOptions += "NOINHERIT "
		}
	}

	if existingRole.CreateRole != desiredRole.CreateRole {
		if !operatorRole.CreateRole {
			err = fmt.Errorf("cannot set CREATEROLE option: the operator's role must have CREATEROLE option")
			return "", err
		}

		if desiredRole.CreateRole {
			rawOptions += "CREATEROLE "
		} else {
			rawOptions += "NOCREATEROLE "
		}
	}

	if existingRole.CreateDB != desiredRole.CreateDB {
		if !operatorRole.CreateDB {
			err = fmt.Errorf("cannot set CREATEDB option: the operator's role must have CREATEDB option")
			return "", err
		}

		if desiredRole.CreateDB {
			rawOptions += "CREATEDB "
		} else {
			rawOptions += "NOCREATEDB "
		}
	}

	if existingRole.Login != desiredRole.Login {
		if !operatorRole.Login {
			err = fmt.Errorf("cannot set LOGIN option: the operator's role must have LOGIN option")
			return "", err
		}

		if desiredRole.Login {
			rawOptions += "LOGIN "
		} else {
			rawOptions += "NOLOGIN "
		}
	}

	if existingRole.Replication != desiredRole.Replication {
		if !operatorRole.Replication {
			err = fmt.Errorf("cannot set REPLICATION option: the operator's role must have REPLICATION option")
			return "", err
		}

		if desiredRole.Replication {
			rawOptions += "REPLICATION "
		} else {
			rawOptions += "NOREPLICATION "
		}
	}

	if existingRole.BypassRLS != desiredRole.BypassRLS {
		if !operatorRole.BypassRLS {
			err = fmt.Errorf("cannot set BYPASSRLS option: the operator's role must have BYPASSRLS option")
			return "", err
		}

		if desiredRole.BypassRLS {
			rawOptions += "BYPASSRLS "
		} else {
			rawOptions += "NOBYPASSRLS "
		}
	}

	if desiredRole.Password != "" {
		rawOptions += fmt.Sprintf("PASSWORD '%s' ", strings.ReplaceAll(desiredRole.Password, "'", "''"))
	}

	options = rawOptions
	return options, nil
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

func AlterRole(pgpool PGPoolInterface, operatorRole, existingRole, desiredRole *Role) (err error) {
	sanitizedName := pgx.Identifier{desiredRole.Name}.Sanitize()

	options, err := generateRoleOptionsString(operatorRole, existingRole, desiredRole)
	if err != nil {
		return err
	}

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("ALTER ROLE %s %s", sanitizedName, options))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}

func ReassignOwnedToRole(pgpool PGPoolInterface, oldRole, newRole string) (err error) {
	sanitizedOldRoleName := pgx.Identifier{oldRole}.Sanitize()
	sanitizedNewRoleName := pgx.Identifier{newRole}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("REASSIGN OWNED BY %s TO %s", sanitizedOldRoleName, sanitizedNewRoleName))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}
