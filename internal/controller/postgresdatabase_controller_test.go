package controller

import (
	"context"
	"fmt"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pgxmock "github.com/pashagolub/pgxmock/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

var _ = Describe("PostgresDatabase Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		postgresdatabase := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}

		var pgpoolsMock map[string]pgxmock.PgxPoolIface
		var pgpools *postgresql.PGPools

		BeforeEach(func() {
			By("creating the custom resource for the Kind PostgresDatabase")
			err := k8sClient.Get(ctx, typeNamespacedName, postgresdatabase)
			if err != nil && errors.IsNotFound(err) {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabaseSpec{
						Name:  "foo",
						Owner: "foo_owner",
						Extensions: []string{
							"plpgsql",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			mock, err := pgxmock.NewPool()
			if err != nil {
				Fail(err.Error())
			}
			pgpoolsMock = map[string]pgxmock.PgxPoolIface{
				"default": mock,
				"foo":     mock,
			}
			pgpools = &postgresql.PGPools{
				Default: mock,
				Databases: map[string]postgresql.PGPoolInterface{
					"foo": mock,
				},
			}
		})

		AfterEach(func() {
			resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PostgresDatabase")
			controllerutil.RemoveFinalizer(resource, PostgresDatabaseFinalizer)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			for _, pool := range pgpoolsMock {
				pool.Close()
			}
		})

		When("the resource is managed by the operator's instance", func() {
			It("should continue to reconcile the resource and create the database", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresDatabaseReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetDatabaseSQLStatement))).
					WithArgs("foo").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"datname",
							"owner",
						}),
					)
				pgpoolsMock["default"].ExpectExec(`CREATE DATABASE "foo"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["default"].ExpectExec(`ALTER DATABASE "foo" OWNER TO "foo_owner"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["foo"].ExpectQuery(`SELECT extname FROM pg_extension`).
					WillReturnRows(
						pgxmock.NewRows([]string{
							"extname",
						}).
							AddRow(
								"plpgsql",
							),
					)

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("the resource is not managed by the operator's instance", func() {
			It("should skip reconciliation", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "bar",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresDatabaseReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("the resource is created and no database exists", func() {
			It("should reconcile the resource and create the database", func() {
				By("Reconciling the created resource")
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetDatabaseSQLStatement))).
					WithArgs("foo").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"datname",
							"owner",
						}),
					)
				pgpoolsMock["default"].ExpectExec(`CREATE DATABASE "foo"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["default"].ExpectExec(`ALTER DATABASE "foo" OWNER TO "foo_owner"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["foo"].ExpectQuery(`SELECT extname FROM pg_extension`).
					WillReturnRows(
						pgxmock.NewRows([]string{
							"extname",
						}).
							AddRow(
								"plpgsql",
							),
					)

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("the resource is deleted", func() {
			It("should successfully reconcile the resource on deletion", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				controllerutil.AddFinalizer(resource, PostgresDatabaseFinalizer)
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetDatabaseSQLStatement))).
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

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP DATABASE "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("reconciling on creation", func() {
			It("should not create database if already exists", func() {
				existingDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
				}
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				err := controllerReconciler.reconcileOnCreation(existingDatabase, desiredDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should create database if not exists", func() {
				var existingDatabase *postgresql.Database
				desiredDatabase := &postgresql.Database{
					Name: "foo",
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(`CREATE DATABASE "foo"`).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileOnCreation(existingDatabase, desiredDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return an error if database creation failed", func() {
				var existingDatabase *postgresql.Database
				desiredDatabase := &postgresql.Database{
					Name: "foo",
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(`CREATE DATABASE "foo"`).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := controllerReconciler.reconcileOnCreation(existingDatabase, desiredDatabase)
				Expect(err).To(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should alter database owner if it has changed", func() {
				existingDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo",
				}
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(`ALTER DATABASE "foo" OWNER TO "foo_owner"`).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileOnCreation(existingDatabase, desiredDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return an error if the alter database owner has failed", func() {
				existingDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo",
				}
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(`ALTER DATABASE "foo" OWNER TO "foo_owner"`).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := controllerReconciler.reconcileOnCreation(existingDatabase, desiredDatabase)
				Expect(err).To(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("reconciling on deletion", func() {
			It("should return immediately if no database exists", func() {
				var existingDatabase *postgresql.Database
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return immediately if the option KeepDatabaseOnDelete is set", func() {
				existingDatabase := &postgresql.Database{
					Name: "foo",
				}
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{
					Spec: managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabaseSpec{
						KeepDatabaseOnDelete: true,
					},
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should drop database successfully", func() {
				existingDatabase := &postgresql.Database{
					Name: "foo",
				}
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP DATABASE "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return an error if dropping database failed", func() {
				existingDatabase := &postgresql.Database{
					Name: "foo",
				}
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}

				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP DATABASE "foo"`))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).To(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return an error if dropping connections failed", func() {
				existingDatabase := &postgresql.Database{
					Name: "foo",
				}
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}

				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = "foo"`))).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).To(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should not drop connections if option PreserveConnectionsOnDelete is set", func() {
				existingDatabase := &postgresql.Database{
					Name: "foo",
				}
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{
					Spec: managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabaseSpec{
						PreserveConnectionsOnDelete: true,
					},
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP DATABASE "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileOnDeletion(resource, existingDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("reconciling extensions", func() {
			It("should create the missing extensions", func() {
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
					Extensions: []string{
						"plpgsql",
						"postgis",
					},
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["foo"].ExpectQuery(`SELECT extname FROM pg_extension`).
					WillReturnRows(
						pgxmock.NewRows([]string{
							"extname",
						}).
							AddRow(
								"plpgsql",
							),
					)
				pgpoolsMock["foo"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE EXTENSION "postgis"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileExtensions(desiredDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should drop the extensions that are not defined", func() {
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
					Extensions: []string{
						"plpgsql",
					},
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["foo"].ExpectQuery(`SELECT extname FROM pg_extension`).
					WillReturnRows(
						pgxmock.NewRows([]string{
							"extname",
						}).
							AddRow(
								"plpgsql",
							).
							AddRow(
								"postgis",
							),
					)
				pgpoolsMock["foo"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP EXTENSION "postgis"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcileExtensions(desiredDatabase)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should return an error if listing extensions failed", func() {
				desiredDatabase := &postgresql.Database{
					Name:  "foo",
					Owner: "foo_owner",
					Extensions: []string{
						"plpgsql",
					},
				}
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				pgpoolsMock["foo"].ExpectQuery(`SELECT extname FROM pg_extension`).
					WillReturnError(fmt.Errorf("fake error from PostgreSQL"))

				err := controllerReconciler.reconcileExtensions(desiredDatabase)
				Expect(err).To(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})
		})

		When("reconciling privileges", func() {
			It("should grant privileges to role 'myrole'", func() {
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				// Loop over all privileges
				for _, privilege := range postgresql.ListDatabaseAvailablePrivileges() {
					pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT has_database_privilege($1, $2, $3)`))).
						WithArgs("myrole", "mydb", privilege).
						WillReturnRows(
							pgxmock.NewRows([]string{
								"changeme",
							}).
								AddRow(
									false,
								),
						)
				}
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`GRANT CREATE ON DATABASE "mydb" TO "myrole"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcilePrivileges(
					"mydb",
					"myrole",
					[]string{
						"CREATE",
					},
				)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should revoke privileges to role 'myrole'", func() {
				controllerReconciler := &PostgresDatabaseReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
				}

				// Loop over all privileges
				for _, privilege := range postgresql.ListDatabaseAvailablePrivileges() {
					pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(`SELECT has_database_privilege($1, $2, $3)`))).
						WithArgs("myrole", "mydb", privilege).
						WillReturnRows(
							pgxmock.NewRows([]string{
								"changeme",
							}).
								AddRow(
									true,
								),
						)
				}
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`REVOKE CREATE ON DATABASE "mydb" FROM "myrole"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				err := controllerReconciler.reconcilePrivileges(
					"mydb",
					"myrole",
					[]string{
						"CONNECT",
						"TEMPORARY",
					},
				)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

		})
	})
})
