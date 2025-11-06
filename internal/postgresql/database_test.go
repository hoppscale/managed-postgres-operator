package postgresql

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

var _ = Describe("PostgreSQL Database", func() {
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

	Context("Calling GetDatabase", func() {
		It("should return a Database pre-filled struct if the database already exists", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetDatabaseSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"datname",
						"owner",
					}).
						AddRow(
							"foo",
							"foo_owner",
						),
				)

			database, err := GetDatabase(pgpool, "foo")

			Expect(database.Name).To(Equal("foo"))
			Expect(database.Owner).To(Equal("foo_owner"))
			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return nil if the database doesn't exist", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetDatabaseSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"datname",
					}),
				)

			database, err := GetDatabase(pgpool, "foo")

			Expect(database).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return nil and an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetDatabaseSQLStatement))).
				WithArgs("foo").
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			database, err := GetDatabase(pgpool, "foo")

			Expect(database).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return nil and an error if PostgreSQL successfully responsed with too many results", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetDatabaseSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"datname",
					}).
						AddRow(
							"foo",
						).
						AddRow(
							"foo1",
						),
				)

			database, err := GetDatabase(pgpool, "foo")

			Expect(database).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return nil and an error if the result doesn't match the model", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetDatabaseSQLStatement))).
				WithArgs("foo").
				WillReturnRows(
					pgxmock.NewRows([]string{
						"datname",
						"fake",
					}).
						AddRow(
							"foo",
							"fake",
						),
				)

			database, err := GetDatabase(pgpool, "foo")

			Expect(database).To(BeNil())
			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling CreateDatabase", func() {
		It("should create a database and return no error", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`CREATE DATABASE "foo"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := CreateDatabase(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`CREATE DATABASE "foo"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := CreateDatabase(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling DropDatabase", func() {
		It("should drop a database and return no error", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`DROP DATABASE "foo"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := DropDatabase(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`DROP DATABASE "foo"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := DropDatabase(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling AlterDatabaseOwner", func() {
		It("should drop a database and return no error", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`ALTER DATABASE "foo" OWNER TO "foo_owner"`)).
				WillReturnResult(pgxmock.NewResult("foo_owner", 1))

			err := AlterDatabaseOwner(pgpool, "foo", "foo_owner")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`ALTER DATABASE "foo" OWNER TO "foo_owner"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := AlterDatabaseOwner(pgpool, "foo", "foo_owner")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling GetExtensions", func() {
		It("should return the list of installed extensions in a database", func() {
			pgpoolMock.ExpectQuery(regexp.QuoteMeta(`SELECT extname FROM pg_extension`)).
				WillReturnRows(
					pgxmock.NewRows([]string{
						"extname",
					}).
						AddRow(
							"plpgsql",
						),
				)

			extensions, err := GetExtensions(pgpool)

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}

			Expect(extensions).To(Equal([]string{"plpgsql"}))
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectQuery(regexp.QuoteMeta(`SELECT extname FROM pg_extension`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			extensions, err := GetExtensions(pgpool)

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
			Expect(extensions).To(BeNil())
		})
		It("should return an error if the row cannot be scanned", func() {
			pgpoolMock.ExpectQuery(regexp.QuoteMeta(`SELECT extname FROM pg_extension`)).
				WillReturnRows(
					pgxmock.NewRows([]string{
						"extname",
					}).
						AddRow(
							"plpgsql",
						).
						RowError(0, fmt.Errorf("row error")),
				)

			extensions, err := GetExtensions(pgpool)

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
			Expect(extensions).To(BeNil())
		})

	})

	Context("Calling CreateExtension", func() {
		It("should return the list of installed extensions in a database", func() {
			pgpoolMock.ExpectQuery(regexp.QuoteMeta(`SELECT extname FROM pg_extension`)).
				WillReturnRows(
					pgxmock.NewRows([]string{
						"extname",
					}).
						AddRow(
							"plpgsql",
						),
				)

			extensions, err := GetExtensions(pgpool)

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}

			Expect(extensions).To(Equal([]string{"plpgsql"}))
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectQuery(regexp.QuoteMeta(`SELECT extname FROM pg_extension`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			extensions, err := GetExtensions(pgpool)

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
			Expect(extensions).To(BeNil())
		})
	})

	Context("Calling CreateExtension", func() {
		It("should create an extension and return no error", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`CREATE EXTENSION "foo"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := CreateExtension(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`CREATE EXTENSION "foo"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := CreateExtension(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling DropExtension", func() {
		It("should drop an extension and return no error", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`DROP EXTENSION "foo"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := DropExtension(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`DROP EXTENSION "foo"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := DropExtension(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling DropDatabaseConnections", func() {
		It("should drop connections to the database and return no error", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := DropDatabaseConnections(pgpool, "foo")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := DropDatabaseConnections(pgpool, "foo")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling GetDatabaseRolePrivileges", func() {
		It("should returns the list of privileges for a role", func() {
			// Loop over all privileges
			existingPrivileges := map[string]bool{
				"CREATE":    false,
				"CONNECT":   true,
				"TEMPORARY": true,
			}
			for privName, privEnabled := range existingPrivileges {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT has_database_privilege($1, $2, $3)`))).
					WithArgs("myrole", "mydb", privName).
					WillReturnRows(
						pgxmock.NewRows([]string{
							"changeme",
						}).
							AddRow(
								privEnabled,
							),
					)
			}

			privs, err := GetDatabaseRolePrivileges(pgpool, "mydb", "myrole")

			Expect(privs).To(Equal([]string{"CONNECT", "TEMPORARY"}))

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT has_database_privilege($1, $2, $3)`))).
				WithArgs("myrole", "mydb", "CREATE").
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			privs, err := GetDatabaseRolePrivileges(pgpool, "mydb", "myrole")

			Expect(privs).To(Equal([]string{}))

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling GrantDatabaseRolePrivilege", func() {
		It("should grant a privilege", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`GRANT CREATE ON DATABASE "mydb" TO "myrole"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := GrantDatabaseRolePrivilege(pgpool, "mydb", "myrole", "CREATE")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`GRANT CREATE ON DATABASE "mydb" TO "myrole"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := GrantDatabaseRolePrivilege(pgpool, "mydb", "myrole", "CREATE")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})
	})

	Context("Calling RevokeDatabaseRolePrivilege", func() {
		It("should revoke a privilege", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`REVOKE CREATE ON DATABASE "mydb" FROM "myrole"`)).
				WillReturnResult(pgxmock.NewResult("foo", 1))

			err := RevokeDatabaseRolePrivilege(pgpool, "mydb", "myrole", "CREATE")

			Expect(err).NotTo(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

		It("should return an error if the PostgreSQL request failed", func() {
			pgpoolMock.ExpectExec(regexp.QuoteMeta(`REVOKE CREATE ON DATABASE "mydb" FROM "myrole"`)).
				WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

			err := RevokeDatabaseRolePrivilege(pgpool, "mydb", "myrole", "CREATE")

			Expect(err).To(HaveOccurred())
			if err := pgpoolMock.ExpectationsWereMet(); err != nil {
				Fail(err.Error())
			}
		})

	})

})
