package postgresql

import (
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

var _ = Describe("PostgreSQL Schema", func() {
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

	Context("Calling GetSchema", func() {
		When("the schema already exists", func() {
			It("should return a Schema pre-filled struct if the schema already exists", func() {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetSchemaSQLStatement))).
					WithArgs("myschema").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"name",
							"owner",
						}).
							AddRow(
								"myschema",
								"myrole",
							),
					)

				schema, err := GetSchema(pgpool, "myschema")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(schema.Name).To(Equal("myschema"))
				Expect(schema.Owner).To(Equal("myrole"))
			})
		})
		When("the schema doesn't exist", func() {
			It("should return a nil Schema and no error", func() {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(GetSchemaSQLStatement))).
					WithArgs("myschema").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"name",
							"owner",
						}),
					)

				schema, err := GetSchema(pgpool, "myschema")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(schema).To(BeNil())
			})
		})
	})

	Context("Calling CreateSchema", func() {
		When("the schema doesn't exist", func() {
			It("should create the schema in database and return no error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("CREATE SCHEMA \"myschema\""))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := CreateSchema(pgpool, "myschema")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("CREATE SCHEMA \"myschema\""))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := CreateSchema(pgpool, "myschema")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})

	Context("Calling DropSchema", func() {
		When("the schema exists", func() {
			It("should drop the schema from database and return no error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("DROP SCHEMA \"myschema\""))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := DropSchema(pgpool, "myschema")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("DROP SCHEMA \"myschema\""))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := DropSchema(pgpool, "myschema")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})

	Context("Calling AlterSchemaOwner", func() {
		When("the schema exists and the role exists", func() {
			It("should successfully alter owner of the schema and return no error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("ALTER SCHEMA \"myschema\" OWNER TO \"myrole\""))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := AlterSchemaOwner(pgpool, "myschema", "myrole")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("ALTER SCHEMA \"myschema\" OWNER TO \"myrole\""))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := AlterSchemaOwner(pgpool, "myschema", "myrole")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})

	Context("Calling GetSchemaRolePrivileges", func() {
		When("the schema and role exist", func() {
			It("should retrieve all privileges associated to the role and return no error", func() {
				// Loop over all privileges
				existingPrivileges := map[string]bool{
					"CREATE": false,
					"USAGE":  true,
				}
				for _, privilege := range ListSchemaAvailablePrivileges() {
					pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta("SELECT has_schema_privilege($1, $2, $3)"))).
						WithArgs(
							"myrole",
							"myschema",
							privilege,
						).
						WillReturnRows(
							pgxmock.NewRows([]string{
								"has_schema_privilege",
							}).
								AddRow(
									existingPrivileges[privilege],
								),
						)
				}

				privs, err := GetSchemaRolePrivileges(pgpool, "myschema", "myrole")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(privs).To(Equal([]string{"USAGE"}))
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error and an empty list of privileges", func() {
				pgpoolMock.ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT has_schema_privilege($1, $2, $3)`))).
					WithArgs("myrole", "mydb", "CREATE").
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				privs, err := GetSchemaRolePrivileges(pgpool, "mydb", "myrole")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}

				Expect(privs).To(Equal([]string{}))
			})
		})
	})

	Context("Calling GrantSchemaRolePrivilege", func() {
		When("the schema and role exist", func() {
			It("should grant privilege to the role and return no error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("GRANT CREATE ON SCHEMA \"myschema\" TO \"myrole\""))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := GrantSchemaRolePrivilege(pgpool, "myschema", "myrole", "CREATE")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("GRANT CREATE ON SCHEMA \"myschema\" TO \"myrole\""))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := GrantSchemaRolePrivilege(pgpool, "myschema", "myrole", "CREATE")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})

	Context("Calling RevokeSchemaRolePrivilege", func() {
		When("the schema and role exist", func() {
			It("should revoke privilege to the role and return no error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("REVOKE CREATE ON SCHEMA \"myschema\" FROM \"myrole\""))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := RevokeSchemaRolePrivilege(pgpool, "myschema", "myrole", "CREATE")

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("PostgreSQL returns an error", func() {
			It("should return an error", func() {
				pgpoolMock.ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta("REVOKE CREATE ON SCHEMA \"myschema\" FROM \"myrole\""))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := RevokeSchemaRolePrivilege(pgpool, "myschema", "myrole", "CREATE")

				Expect(err).To(HaveOccurred())
				if err := pgpoolMock.ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})
	})
})
