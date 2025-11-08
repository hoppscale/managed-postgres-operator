package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Schema struct {
	Database string `db:"-"`
	Name     string `db:"name"`
	Owner    string `db:"owner"`
}

const GetSchemaSQLStatement = "SELECT schema_name as name, schema_owner as owner FROM information_schema.schemata WHERE schema_name = $1"

func GetSchema(pgpool PGPoolInterface, name string) (schema *Schema, err error) {
	rows, err := pgpool.Query(context.Background(), GetSchemaSQLStatement, name)
	if err != nil {
		err = fmt.Errorf("pg query failed: %s", err)
		return
	}
	defer rows.Close()

	schemas, err := pgx.CollectRows(rows, pgx.RowToStructByName[Schema])
	if err != nil {
		err = fmt.Errorf("failed to collect rows: %s", err)
		return
	}

	if len(schemas) > 1 {
		err = fmt.Errorf("wrong number of rows returned, expected 1, got %d", len(schemas))
		return
	}

	if len(schemas) == 0 {
		return
	}

	schema = &schemas[0]

	return schema, err
}

func CreateSchema(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to create schema: %s", err)
	}

	return err
}

func DropSchema(pgpool PGPoolInterface, name string) (err error) {
	sanitizedName := pgx.Identifier{name}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s", sanitizedName))
	if err != nil {
		return fmt.Errorf("failed to drop schema: %s", err)
	}

	return err
}

func AlterSchemaOwner(pgpool PGPoolInterface, schema, owner string) (err error) {
	sanitizedSchemaName := pgx.Identifier{schema}.Sanitize()
	sanitizedOwnerName := pgx.Identifier{owner}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("ALTER SCHEMA %s OWNER TO %s", sanitizedSchemaName, sanitizedOwnerName))
	if err != nil {
		return fmt.Errorf("failed to alter schema owner: %s", err)
	}
	return err
}

func ListSchemaAvailablePrivileges() []string {
	return []string{
		"CREATE",
		"USAGE",
	}
}

func GetSchemaRolePrivileges(pgpool PGPoolInterface, schema, role string) (existingPrivileges []string, err error) {
	existingPrivileges = []string{}
	var hasPrivilege bool
	for _, privilege := range ListSchemaAvailablePrivileges() {
		rows, err := pgpool.Query(context.Background(), "SELECT has_schema_privilege($1, $2, $3)", role, schema, privilege)
		if err != nil {
			err = fmt.Errorf("pg query failed: %s", err)
			return []string{}, err
		}
		defer rows.Close()

		hasPrivilege, err = pgx.CollectOneRow(rows, pgx.RowTo[bool])
		if err != nil {
			err = fmt.Errorf("failed to collect rows: %s", err)
			return []string{}, err
		}

		if hasPrivilege {
			existingPrivileges = append(existingPrivileges, privilege)
		}
	}
	return
}

func GrantSchemaRolePrivilege(pgpool PGPoolInterface, schema, role, privilege string) (err error) {
	sanitizedSchema := pgx.Identifier{schema}.Sanitize()
	sanitizedRole := pgx.Identifier{role}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("GRANT %s ON SCHEMA %s TO %s", privilege, sanitizedSchema, sanitizedRole))
	if err != nil {
		return fmt.Errorf("failed to grant privilege \"%s\" on schema %s to role %s: %s", privilege, sanitizedSchema, sanitizedRole, err)
	}

	return
}

func RevokeSchemaRolePrivilege(pgpool PGPoolInterface, schema, role, privilege string) (err error) {
	sanitizedSchema := pgx.Identifier{schema}.Sanitize()
	sanitizedRole := pgx.Identifier{role}.Sanitize()

	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("REVOKE %s ON SCHEMA %s FROM %s", privilege, sanitizedSchema, sanitizedRole))
	if err != nil {
		return fmt.Errorf("failed to revoke privilege \"%s\" on schema %s from role %s: %s", privilege, sanitizedSchema, sanitizedRole, err)
	}

	return
}
