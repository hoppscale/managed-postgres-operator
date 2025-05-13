package postgresql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const GetRoleMembershipStatement = "SELECT roleid::regrole::text AS group_role FROM pg_auth_members WHERE member::regrole::text = $1"

func GetRoleMembership(pgpool PGPoolInterface, role string) (membership []string, err error) {
	rows, err := pgpool.Query(context.Background(), GetRoleMembershipStatement, role)
	if err != nil {
		err = fmt.Errorf("pg query failed: %s", err)
		return
	}
	defer rows.Close()

	membership, err = pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		err = fmt.Errorf("failed to collect rows: %s", err)
		return
	}

	return
}

func GrantRoleMembership(pgpool PGPoolInterface, groupRole, role string) (err error) {
	sanitizedGroupRole := pgx.Identifier{groupRole}.Sanitize()
	sanitizedRole := pgx.Identifier{role}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("GRANT %s TO %s", sanitizedGroupRole, sanitizedRole))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}

func RevokeRoleMembership(pgpool PGPoolInterface, groupRole, role string) (err error) {
	sanitizedGroupRole := pgx.Identifier{groupRole}.Sanitize()
	sanitizedRole := pgx.Identifier{role}.Sanitize()
	_, err = pgpool.Exec(context.Background(), fmt.Sprintf("REVOKE %s FROM %s", sanitizedGroupRole, sanitizedRole))
	if err != nil {
		err = fmt.Errorf("pg exec failed: %s", err)
		return
	}
	return
}
