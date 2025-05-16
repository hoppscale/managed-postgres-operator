package postgresql

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

var _ = Describe("PostgreSQL Role", func() {
	var pgpoolMock pgxmock.PgxPoolIface
	var pgpool PGPoolInterface

	BeforeEach(func() {
		mock, err := pgxmock.NewPool()
		if err != nil {
			Fail(err.Error())
		}
		pgpoolMock = mock
		pgpool = mock
	})
	AfterEach(func() {
		pgpoolMock.Close()
	})

	Context("Calling GetRoleMemberships", func() {
		When("the role has no membership", func() {
			It("should an empty list of roles and no error", func() {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleMembershipStatement))).
					WithArgs("foo").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"group_role",
						}),
					)

				result, err := GetRoleMembership(pgpool, "foo")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(result).To(BeEmpty())
			})
		})

		When("the role is a member of multiple roles", func() {
			It("should return the list of group roles and no error", func() {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleMembershipStatement))).
					WithArgs("foo").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"group_role",
						}).
							AddRow(
								"alpha",
							).
							AddRow(
								"beta",
							),
					)

				result, err := GetRoleMembership(pgpool, "foo")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(result).To(Equal([]string{"alpha", "beta"}))
			})
		})
	})

	Context("Calling GrantRoleMembership", func() {
		It("should successfully grant the role to the group role", func() {

			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`GRANT "foo" TO "bar"`))).
				WillReturnResult(pgxmock.NewResult("", 1))

			err := GrantRoleMembership(pgpool, "foo", "bar")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling RevokeRoleMembership", func() {
		It("should successfully revoke the role from the group role", func() {

			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`REVOKE "foo" FROM "bar"`))).
				WillReturnResult(pgxmock.NewResult("", 1))

			err := RevokeRoleMembership(pgpool, "foo", "bar")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

})
