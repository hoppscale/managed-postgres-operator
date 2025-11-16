/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pgxmock "github.com/pashagolub/pgxmock/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

var _ = Describe("PostgresRole Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		postgresrole := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}

		var pgpoolsMock map[string]pgxmock.PgxPoolIface
		var pgpools *postgresql.PGPools

		BeforeEach(func() {
			By("creating the custom resource for the Kind PostgresRole")
			err := k8sClient.Get(ctx, typeNamespacedName, postgresrole)
			if err != nil && errors.IsNotFound(err) {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: managedpostgresoperatorhoppscalecomv1alpha1.PostgresRoleSpec{
						Name:       "myrole",
						CreateRole: true,
						CreateDB:   true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				resourceSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "myrole-password",
					},
					Type: "Opaque",
					Data: map[string][]byte{
						"password": []byte("mypassword"),
					},
				}
				Expect(k8sClient.Create(ctx, resourceSecret)).To(Succeed())

				mock, err := pgxmock.NewPool()
				if err != nil {
					Fail(err.Error())
				}
				pgpoolsMock = map[string]pgxmock.PgxPoolIface{
					"default": mock,
				}
				pgpools = &postgresql.PGPools{
					Default:   mock,
					Databases: map[string]postgresql.PGPoolInterface{},
				}
			}
		})

		AfterEach(func() {
			// Delete PGPools
			for _, pool := range pgpoolsMock {
				pool.Close()
			}

			// Delete Secret (passwordFromSecret)
			resourceSecretInput := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "myrole-password",
				},
			}
			typeSecretInputNamespacedName := types.NamespacedName{
				Namespace: "default",
				Name:      "myrole-password",
			}
			err := k8sClient.Get(ctx, typeSecretInputNamespacedName, resourceSecretInput)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resourceSecretInput)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				Fail(err.Error())
			}

			// Delete Secret (secretName)
			resourceSecretOutput := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "db-config-myrole",
				},
			}
			typeSecretOutputNamespacedName := types.NamespacedName{
				Namespace: "default",
				Name:      "db-config-myrole",
			}
			err = k8sClient.Get(ctx, typeSecretOutputNamespacedName, resourceSecretOutput)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resourceSecretOutput)).To(Succeed())
			} else if !errors.IsNotFound(err) {
				Fail(err.Error())
			}

			// Delete CustomResource
			resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PostgresRole")
			controllerutil.RemoveFinalizer(resource, PostgresRoleFinalizer)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		When("the resource is created and managed by the operator's instance", func() {
			When("the role doesn't exist", func() {
				When("no password is provided", func() {
					It("should continue to reconcile the resource and create the role", func() {
						resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
						Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
						resource.ObjectMeta.Annotations = map[string]string{
							utils.OperatorInstanceAnnotationName: "foo",
						}
						Expect(k8sClient.Update(ctx, resource)).To(Succeed())
						controllerReconciler := &PostgresRoleReconciler{
							Client:               k8sClient,
							Scheme:               k8sClient.Scheme(),
							PGPools:              pgpools,
							OperatorInstanceName: "foo",
							CacheRolePasswords:   make(map[string]string),
						}

						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
							WithArgs("myrole").
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
						pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s '.*'$", regexp.QuoteMeta(`CREATE ROLE "myrole" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS PASSWORD`))).
							WillReturnResult(pgxmock.NewResult("CREATE ROLE", 1))
						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
							WithArgs("myrole").
							WillReturnRows(
								pgxmock.NewRows([]string{
									"group_role",
								}),
							)

						_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
							NamespacedName: typeNamespacedName,
						})

						Expect(err).NotTo(HaveOccurred())
						if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}
					})
				})

				When("a secretName is provided", func() {
					It("should create a Secret with PostgreSQL connection information", func() {
						controllerReconciler := &PostgresRoleReconciler{
							Client:               k8sClient,
							Scheme:               k8sClient.Scheme(),
							PGPools:              pgpools,
							OperatorInstanceName: "foo",
							CacheRolePasswords:   make(map[string]string),
						}

						role := postgresql.Role{
							Name:     "myrole",
							Password: "mypassword",
						}

						pgConfig, err := pgx.ParseConfig("postgres://localhost:5432/mydatabase")
						Expect(err).NotTo(HaveOccurred())

						err = controllerReconciler.reconcileRoleSecret(
							"default",
							"db-config-myrole",
							make(map[string]string),
							&role,
							pgConfig,
						)

						Expect(err).NotTo(HaveOccurred())

						outputSecretNamespacedName := types.NamespacedName{
							Namespace: "default",
							Name:      "db-config-myrole",
						}
						outputSecret := &corev1.Secret{}
						Expect(k8sClient.Get(ctx, outputSecretNamespacedName, outputSecret)).To(Succeed())
						Expect(outputSecret.Data["PGUSER"]).To(Equal([]byte("myrole")))
						Expect(outputSecret.Data["PGPASSWORD"]).To(Equal([]byte("mypassword")))
						Expect(outputSecret.Data["PGHOST"]).To(Equal([]byte("localhost")))
						Expect(outputSecret.Data["PGPORT"]).To(Equal([]byte("5432")))
						Expect(outputSecret.Data["PGDATABASE"]).To(Equal([]byte("mydatabase")))
						Expect(outputSecret.Data).To(HaveLen(5))
					})
				})

				When("a secretName and a secretTemplate are provided", func() {
					It("should create a Secret with PostgreSQL connection information with the defined templating", func() {
						controllerReconciler := &PostgresRoleReconciler{
							Client:               k8sClient,
							Scheme:               k8sClient.Scheme(),
							PGPools:              pgpools,
							OperatorInstanceName: "foo",
							CacheRolePasswords:   make(map[string]string),
						}

						role := postgresql.Role{
							Name:     "myrole",
							Password: "mypassword",
						}

						pgConfig, err := pgx.ParseConfig("postgres://localhost:5432/mydatabase")
						Expect(err).NotTo(HaveOccurred())

						secretTemplate := map[string]string{
							"PGDATABASE": "fake",
							"JDBC_URL":   "jdbc:postgresql://{{ .Host }}:{{ .Port }}/fake?user={{ .Role }}&password={{ .Password }}",
						}
						err = controllerReconciler.reconcileRoleSecret(
							"default",
							"db-config-myrole",
							secretTemplate,
							&role,
							pgConfig,
						)

						Expect(err).NotTo(HaveOccurred())
						if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}

						outputSecretNamespacedName := types.NamespacedName{
							Namespace: "default",
							Name:      "db-config-myrole",
						}
						outputSecret := &corev1.Secret{}
						Expect(k8sClient.Get(ctx, outputSecretNamespacedName, outputSecret)).To(Succeed())
						Expect(outputSecret.Data["PGUSER"]).To(Equal([]byte("myrole")))
						Expect(outputSecret.Data["PGPASSWORD"]).To(Equal([]byte("mypassword")))
						Expect(outputSecret.Data["PGHOST"]).To(Equal([]byte("localhost")))
						Expect(outputSecret.Data["PGPORT"]).To(Equal([]byte("5432")))
						Expect(outputSecret.Data["PGDATABASE"]).To(Equal([]byte("fake")))
						Expect(outputSecret.Data["JDBC_URL"]).To(Equal([]byte("jdbc:postgresql://localhost:5432/fake?user=myrole&password=mypassword")))
						Expect(outputSecret.Data).To(HaveLen(6))
					})
				})

				When("a password is provided", func() {
					It("should retrieve the password from the secret and create the role", func() {
						resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
						Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
						resource.ObjectMeta.Annotations = map[string]string{
							utils.OperatorInstanceAnnotationName: "foo",
						}
						resource.Spec.PasswordFromSecret = &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRolePasswordFromSecret{
							Name: "myrole-password",
							Key:  "password",
						}
						Expect(k8sClient.Update(ctx, resource)).To(Succeed())
						controllerReconciler := &PostgresRoleReconciler{
							Client:               k8sClient,
							Scheme:               k8sClient.Scheme(),
							PGPools:              pgpools,
							OperatorInstanceName: "foo",
							CacheRolePasswords:   make(map[string]string),
						}

						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
							WithArgs("myrole").
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
						pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "myrole" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS PASSWORD 'mypassword'`))).
							WillReturnResult(pgxmock.NewResult("CREATE ROLE", 1))
						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
							WithArgs("myrole").
							WillReturnRows(
								pgxmock.NewRows([]string{
									"group_role",
								}),
							)

						_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
							NamespacedName: typeNamespacedName,
						})

						Expect(err).NotTo(HaveOccurred())
						if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}
					})
				})
			})

			When("the role already exists", func() {
				When("there is no difference between the existing role and the resource", func() {
					It("should retrieve the role and do nothing", func() {
						controllerReconciler := &PostgresRoleReconciler{
							Client:  k8sClient,
							Scheme:  k8sClient.Scheme(),
							PGPools: pgpools,
							CacheRolePasswords: map[string]string{
								"myrole": "mypassword",
							},
						}

						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
							WithArgs("myrole").
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
										"myrole",
										false,
										false,
										true,
										true,
										false,
										false,
										false,
									),
							)
						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
							WithArgs("myrole").
							WillReturnRows(
								pgxmock.NewRows([]string{
									"group_role",
								}),
							)

						_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
							NamespacedName: typeNamespacedName,
						})

						Expect(err).NotTo(HaveOccurred())
						if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}
					})
				})

				When("the role is member of a role which has been changed", func() {
					It("should revoke membership on the old role and grant membership on the new one", func() {
						resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
						Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
						resource.Spec.MemberOfRoles = []string{
							"role_to_add",
						}
						Expect(k8sClient.Update(ctx, resource)).To(Succeed())

						controllerReconciler := &PostgresRoleReconciler{
							Client:  k8sClient,
							Scheme:  k8sClient.Scheme(),
							PGPools: pgpools,
							CacheRolePasswords: map[string]string{
								"myrole": "mypassword",
							},
						}

						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
							WithArgs("myrole").
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
										"myrole",
										false,
										false,
										true,
										true,
										false,
										false,
										false,
									),
							)
						pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
							WithArgs("myrole").
							WillReturnRows(
								pgxmock.NewRows([]string{
									"group_role",
								}).
									AddRow(
										"role_to_remove",
									),
							)

						pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`REVOKE "role_to_remove" FROM "myrole"`))).
							WillReturnResult(pgxmock.NewResult("REVOKE", 1))

						pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`GRANT "role_to_add" TO "myrole"`))).
							WillReturnResult(pgxmock.NewResult("GRANT", 1))

						_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
							NamespacedName: typeNamespacedName,
						})

						Expect(err).NotTo(HaveOccurred())
						if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}
					})
				})

				When("the output secret exists", func() {
					When("the output secret's label has been removed and PGUSER is missing", func() {
						It("should update the output secret to add the 'managed-by' label and add PGUSER field", func() {
							existingOutputSecret := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: "default",
									Name:      "db-config-myrole",
								},
								Type: "Opaque",
								Data: map[string][]byte{
									"PGPASSWORD": []byte("mypassword"),
									"PGHOST":     []byte("localhost"),
									"PGPORT":     []byte("5432"),
									"PGDATABASE": []byte("mydatabase"),
								},
							}
							Expect(k8sClient.Create(ctx, existingOutputSecret)).To(Succeed())

							controllerReconciler := &PostgresRoleReconciler{
								Client:               k8sClient,
								Scheme:               k8sClient.Scheme(),
								PGPools:              pgpools,
								OperatorInstanceName: "foo",
								CacheRolePasswords:   make(map[string]string),
							}

							role := postgresql.Role{
								Name:     "myrole",
								Password: "mypassword",
							}

							pgConfig, err := pgx.ParseConfig("postgres://localhost:5432/mydatabase")
							Expect(err).NotTo(HaveOccurred())

							err = controllerReconciler.reconcileRoleSecret(
								"default",
								"db-config-myrole",
								make(map[string]string),
								&role,
								pgConfig,
							)

							Expect(err).NotTo(HaveOccurred())

							outputSecretNamespacedName := types.NamespacedName{
								Namespace: "default",
								Name:      "db-config-myrole",
							}
							outputSecret := &corev1.Secret{}
							Expect(k8sClient.Get(ctx, outputSecretNamespacedName, outputSecret)).To(Succeed())
							Expect(outputSecret.Data["PGUSER"]).To(Equal([]byte("myrole")))
							Expect(outputSecret.Data["PGPASSWORD"]).To(Equal([]byte("mypassword")))
							Expect(outputSecret.Data["PGHOST"]).To(Equal([]byte("localhost")))
							Expect(outputSecret.Data["PGPORT"]).To(Equal([]byte("5432")))
							Expect(outputSecret.Data["PGDATABASE"]).To(Equal([]byte("mydatabase")))
							Expect(outputSecret.Data).To(HaveLen(5))
						})
					})
				})

			})

			When("the resource has been changed and the role needs to be updated", func() {
				It("should alter role to apply the changes", func() {
					existingRole := &postgresql.Role{
						Name: "myrole",
					}
					desiredRole := &postgresql.Role{
						Name:     "myrole",
						CreateDB: true,
					}

					controllerReconciler := &PostgresRoleReconciler{
						Client:             k8sClient,
						Scheme:             k8sClient.Scheme(),
						PGPools:            pgpools,
						CacheRolePasswords: map[string]string{},
					}

					pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`ALTER ROLE "myrole" WITH NOSUPERUSER NOINHERIT NOCREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS`))).
						WillReturnResult(pgxmock.NewResult("foo", 1))

					err := controllerReconciler.reconcileOnCreation(existingRole, desiredRole)
					Expect(err).NotTo(HaveOccurred())
					for _, poolMock := range pgpoolsMock {
						if err := poolMock.ExpectationsWereMet(); err != nil {
							Fail(err.Error())
						}
					}
				})
			})
		})

		When("the resource is not managed by the operator's instance", func() {
			It("should skip reconciliation", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "bar",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresRoleReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
					CacheRolePasswords:   make(map[string]string),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("the resource is deleted", func() {
			When("the role exists", func() {
				It("should successfully drop the role", func() {
					resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
					Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
					controllerutil.AddFinalizer(resource, PostgresRoleFinalizer)
					Expect(k8sClient.Update(ctx, resource)).To(Succeed())
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

					controllerReconciler := &PostgresRoleReconciler{
						Client:             k8sClient,
						Scheme:             k8sClient.Scheme(),
						PGPools:            pgpools,
						CacheRolePasswords: make(map[string]string),
					}

					pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
						WithArgs("myrole").
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
									"myrole",
									false,
									false,
									true,
									true,
									false,
									false,
									false,
								),
						)
					pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "myrole"`))).
						WillReturnResult(pgxmock.NewResult("DROP ROLE", 1))

					_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})

					Expect(err).NotTo(HaveOccurred())
					if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				})
			})
			When("no role exists", func() {
				It("should return immediately", func() {
					var existingRole *postgresql.Role

					controllerReconciler := &PostgresRoleReconciler{
						Client:             k8sClient,
						Scheme:             k8sClient.Scheme(),
						PGPools:            pgpools,
						CacheRolePasswords: map[string]string{},
					}

					err := controllerReconciler.reconcileOnDeletion(existingRole)
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
})
