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
						Name:       "foo",
						CreateRole: true,
						CreateDB:   true,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				resourceSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "db-foo",
					},
					Type: "Opaque",
					Data: map[string][]byte{
						"password": []byte("password"),
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

			// Delete Secret
			resourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "db-foo",
				},
			}
			Expect(k8sClient.Delete(ctx, resourceSecret)).To(Succeed())

			// Delete CustomResource
			resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PostgresRole")
			controllerutil.RemoveFinalizer(resource, PostgresRoleFinalizer)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		When("the resource is managed by the operator's instance", func() {
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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))
				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
					WithArgs("foo").
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

		When("the resource is created without password and no role exists", func() {
			It("should reconcile the resource and create the role", func() {
				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: make(map[string]string),
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))
				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
					WithArgs("foo").
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

		When("the resource is created but the role already exists", func() {
			It("should not try to create role and match existing role", func() {
				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: make(map[string]string),
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
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
				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
					WithArgs("foo").
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

		When("the resource is created with a password and no role exists", func() {
			It("should reconcile the resource and create the role", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.Spec.PasswordSecretName = "db-foo"
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: make(map[string]string),
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS PASSWORD 'password'`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))
				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
					WithArgs("foo").
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

		When("the resource is created without a password but a secret and no role exists", func() {
			It("should reconcile the resource, create a secret, generate a password and create the role", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.Spec.PasswordSecretName = "db-config"
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: make(map[string]string),
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s[a-zA-Z0-9]{30}'$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER NOINHERIT CREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS PASSWORD '`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))
				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleMembershipStatement))).
					WithArgs("foo").
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
				secretPassword := &corev1.Secret{}
				secretPasswordName := types.NamespacedName{
					Namespace: "default",
					Name:      "db-config",
				}
				Expect(k8sClient.Get(ctx, secretPasswordName, secretPassword)).To(Succeed())
				Expect(k8sClient.Delete(ctx, secretPassword)).To(Succeed())
			})
		})

		When("the resource is deleted", func() {
			It("should successfully reconcile the resource on deletion", func() {
				By("Reconciling the deleted resource")

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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "foo"`))).
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

		When("the resource is deleted and an associated secret exists", func() {
			It("should successfully reconcile the resource on deletion and delete the associated secret", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				controllerutil.AddFinalizer(resource, PostgresRoleFinalizer)
				resource.Spec.PasswordSecretName = "db-config"
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

				resourceSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Name:      "db-config",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by": "managed-postgres-operator.hoppscale.com",
						},
					},
					Type: "Opaque",
					Data: map[string][]byte{
						"password": []byte("password"),
					},
				}
				Expect(k8sClient.Create(ctx, resourceSecret)).To(Succeed())

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: make(map[string]string),
				}

				pgpoolsMock["default"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetRoleSQLStatement))).
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
				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolsMock["default"].ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
				secretNamespacedName := types.NamespacedName{
					Namespace: "default",
					Name:      "db-config",
				}
				err = k8sClient.Get(ctx, secretNamespacedName, &corev1.Secret{})
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			})
		})

		When("reconciling on creation", func() {
			It("should not create role if already exists", func() {
				existingRole := &postgresql.Role{
					Name: "foo",
				}
				desiredRole := &postgresql.Role{
					Name: "foo",
				}

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: map[string]string{},
				}

				err := controllerReconciler.reconcileOnCreation(existingRole, desiredRole)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should create role if it doesn't exist", func() {
				var existingRole *postgresql.Role = nil
				desiredRole := &postgresql.Role{
					Name: "foo",
				}

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: map[string]string{},
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`CREATE ROLE "foo" WITH NOSUPERUSER NOINHERIT NOCREATEROLE NOCREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := controllerReconciler.reconcileOnCreation(existingRole, desiredRole)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should update role if the desired role's password is different than the cached password", func() {
				existingRole := &postgresql.Role{
					Name: "foo",
				}
				desiredRole := &postgresql.Role{
					Name:     "foo",
					Password: "fake",
				}

				controllerReconciler := &PostgresRoleReconciler{
					Client:  k8sClient,
					Scheme:  k8sClient.Scheme(),
					PGPools: pgpools,
					CacheRolePasswords: map[string]string{
						"foo": "foo",
					},
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`ALTER ROLE "foo" WITH NOSUPERUSER NOINHERIT NOCREATEROLE NOCREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS PASSWORD 'fake'`))).
					WillReturnResult(pgxmock.NewResult("foo", 1))

				err := controllerReconciler.reconcileOnCreation(existingRole, desiredRole)
				Expect(err).NotTo(HaveOccurred())
				for _, poolMock := range pgpoolsMock {
					if err := poolMock.ExpectationsWereMet(); err != nil {
						Fail(err.Error())
					}
				}
			})

			It("should update role if the desired role is different than the existing role", func() {
				existingRole := &postgresql.Role{
					Name: "foo",
				}
				desiredRole := &postgresql.Role{
					Name:     "foo",
					CreateDB: true,
				}

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: map[string]string{},
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`ALTER ROLE "foo" WITH NOSUPERUSER NOINHERIT NOCREATEROLE CREATEDB NOLOGIN NOREPLICATION NOBYPASSRLS`))).
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

		When("reconciling on deletion", func() {
			It("should return immediately if no role exists", func() {
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

			It("should drop database successfully", func() {
				existingRole := &postgresql.Role{
					Name: "foo",
				}

				controllerReconciler := &PostgresRoleReconciler{
					Client:             k8sClient,
					Scheme:             k8sClient.Scheme(),
					PGPools:            pgpools,
					CacheRolePasswords: map[string]string{},
				}

				pgpoolsMock["default"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP ROLE "foo"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

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
