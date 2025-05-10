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

	Context("Calling GetRole", func() {
		It("should return the role if the role already exists", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"rolname",
						"rolsuper",
						"rolinherit",
						"rolcreaterole",
						"rolcreatedb",
						"rolcanlogin",
						"rolreplication",
						"rolbypassrls",
					}).
						AddRow(
							"foo",
							false,
							false,
							true,
							true,
							false,
							false,
							false,
						),
				)

			role, err := GetRole(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}

			Expect(role.Name).To(Equal("foo"))
			Expect(role.SuperUser).To(BeFalse())
			Expect(role.Inherit).To(BeFalse())
			Expect(role.CreateRole).To(BeTrue())
			Expect(role.CreateDB).To(BeTrue())
			Expect(role.Login).To(BeFalse())
			Expect(role.Replication).To(BeFalse())
			Expect(role.Password).To(Equal(""))
			Expect(role.BypassRLS).To(BeFalse())
		})

		It("should return an empty role if the role doesn't exist", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"rolname",
						"rolsuper",
						"rolinherit",
						"rolcreaterole",
						"rolcreatedb",
						"rolcanlogin",
						"rolreplication",
						"rolbypassrls",
					}),
				)

			role, err := GetRole(pgpool, "foo")

			Expect(role).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleSQLStatement))).
				WithArgs("foo").
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			role, err := GetRole(pgpool, "foo")

			Expect(role).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request returns more than one row", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"rolname",
						"rolsuper",
						"rolinherit",
						"rolcreaterole",
						"rolcreatedb",
						"rolcanlogin",
						"rolreplication",
						"rolbypassrls",
					}).
						AddRow(
							"foo",
							false,
							false,
							true,
							true,
							false,
							false,
							false,
						).
						AddRow(
							"foo2",
							false,
							false,
							true,
							true,
							false,
							false,
							false,
						),
				)

			role, err := GetRole(pgpool, "foo")

			Expect(role).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the result do not match the model", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetRoleSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"rolname",
						"rolsuper",
						"rolinherit",
						"rolcreaterole",
						"rolcreatedb",
						"rolcanlogin",
						"rolreplication",
						"rolbypassrls",
						"fake",
					}).
						AddRow(
							"foo",
							false,
							false,
							true,
							true,
							false,
							false,
							false,
							"fake",
						).
						AddRow(
							"foo2",
							false,
							false,
							true,
							true,
							false,
							false,
							false,
							"fake",
						),
				)

			role, err := GetRole(pgpool, "foo")

			Expect(role).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling CreateRole", func() {
		It("should create a role with the defined options and return no error", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH SUPERUSER NOINHERIT CREATEROLE NOCREATEDB NOLOGIN NOREPLICATION BYPASSRLS PASSWORD 'password'`))).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			role := Role{
				Name:       "foo",
				SuperUser:  true,
				Inherit:    false,
				CreateRole: true,
				BypassRLS:  true,
				Password:   "password",
			}
			err := CreateRole(pgpool, &role)

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER INHERIT NOCREATEROLE CREATEDB LOGIN REPLICATION NOBYPASSRLS`))).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			role := Role{
				Name:        "foo",
				Inherit:     true,
				CreateDB:    true,
				Login:       true,
				Replication: true,
			}
			err := CreateRole(pgpool, &role)

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling DropRole", func() {
		It("should drop a role and return no error", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "foo"`))).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := DropRole(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "foo"`))).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := DropRole(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling AlterRole", func() {
		It("should update a role with the defined options and return no error", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`ALTER ROLE "foo" WITH SUPERUSER NOINHERIT CREATEROLE NOCREATEDB NOLOGIN NOREPLICATION BYPASSRLS`))).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			role := Role{
				Name:       "foo",
				SuperUser:  true,
				Inherit:    false,
				CreateRole: true,
				BypassRLS:  true,
			}
			err := AlterRole(pgpool, &role)

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`ALTER ROLE "foo" WITH NOSUPERUSER INHERIT NOCREATEROLE CREATEDB LOGIN REPLICATION NOBYPASSRLS`))).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			role := Role{
				Name:        "foo",
				Inherit:     true,
				CreateDB:    true,
				Login:       true,
				Replication: true,
			}
			err := AlterRole(pgpool, &role)

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})
})
