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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

var _ = Describe("PostgresSchema Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		postgresschema := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}

		var pgpoolsMock map[string]pgxmock.PgxPoolIface
		var pgpools *postgresql.PGPools

		BeforeEach(func() {
			By("creating the custom resource for the Kind PostgresSchema")
			err := k8sClient.Get(ctx, typeNamespacedName, postgresschema)
			if err != nil && errors.IsNotFound(err) {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchemaSpec{
						Database: "mydb",
						Name:     "myschema",
						Owner:    "myrole",
						PrivilegesByRole: map[string]managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchemaPrivilegesSpec{
							"fakerole": {
								Create: false,
								Usage:  true,
							},
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
				"mydb":    mock,
			}
			pgpools = &postgresql.PGPools{
				Default: mock,
				Databases: map[string]postgresql.PGPoolInterface{
					"mydb": mock,
				},
			}
		})

		AfterEach(func() {
			resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance PostgresSchema")
			controllerutil.RemoveFinalizer(resource, PostgresSchemaFinalizer)
			Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			for _, pool := range pgpoolsMock {
				pool.Close()
			}
		})

		When("the resource is managed by the operator's instance", func() {
			It("should continue to reconcile the resource and create the schema", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetSchemaSQLStatement))).
					WithArgs("myschema").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"name",
							"owner",
						}),
					)
				pgpoolsMock["mydb"].ExpectExec(`CREATE SCHEMA "myschema"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["mydb"].ExpectExec(`ALTER SCHEMA "myschema" OWNER TO "myrole"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				for _, privilege := range postgresql.ListSchemaAvailablePrivileges() {
					pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta("SELECT has_schema_privilege($1, $2, $3)"))).
						WithArgs(
							"fakerole",
							"myschema",
							privilege,
						).
						WillReturnRows(
							pgxmock.NewRows([]string{
								"has_schema_privilege",
							}).
								AddRow(
									false,
								),
						)
				}
				pgpoolsMock["mydb"].ExpectExec(`GRANT USAGE ON SCHEMA "myschema" TO "fakerole"`).
					WillReturnResult(pgxmock.NewResult("", 1))

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
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "bar",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
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

		When("the schema is owned by another role", func() {
			It("should change the owner of the schema", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetSchemaSQLStatement))).
					WithArgs("myschema").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"name",
							"owner",
						}).
							AddRow(
								"myschema",
								"anotherrole",
							),
					)
				pgpoolsMock["mydb"].ExpectExec(`ALTER SCHEMA "myschema" OWNER TO "myrole"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				// Loop over all privileges
				existingPrivileges := map[string]bool{
					"CREATE": false,
					"USAGE":  true,
				}
				for _, privilege := range postgresql.ListSchemaAvailablePrivileges() {
					pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta("SELECT has_schema_privilege($1, $2, $3)"))).
						WithArgs(
							"fakerole",
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

		When("role privileges are updated", func() {
			It("should grant missing privileges and revoke the others", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				resource.Spec.PrivilegesByRole = map[string]managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchemaPrivilegesSpec{
					"fakerole": {
						Create: true,
						Usage:  false,
					},
				}
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetSchemaSQLStatement))).
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
				// Loop over all privileges
				existingPrivileges := map[string]bool{
					"CREATE": false,
					"USAGE":  true,
				}

				for _, privilege := range postgresql.ListSchemaAvailablePrivileges() {
					pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta("SELECT has_schema_privilege($1, $2, $3)"))).
						WithArgs(
							"fakerole",
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
				pgpoolsMock["mydb"].ExpectExec(`GRANT CREATE ON SCHEMA "myschema" TO "fakerole"`).
					WillReturnResult(pgxmock.NewResult("", 1))
				pgpoolsMock["mydb"].ExpectExec(`REVOKE USAGE ON SCHEMA "myschema" FROM "fakerole"`).
					WillReturnResult(pgxmock.NewResult("", 1))

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
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				controllerutil.AddFinalizer(resource, PostgresSchemaFinalizer)
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetSchemaSQLStatement))).
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

				pgpoolsMock["mydb"].ExpectExec(fmt.Sprintf("^%s$", regexp.QuoteMeta(`DROP SCHEMA "myschema"`))).
					WillReturnResult(pgxmock.NewResult("", 1))

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolsMock["mydb"].ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

		When("the resource is deleted but the schema doesn't exist", func() {
			It("should delete the resource but skip the DROP SCHEMA", func() {
				resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}
				Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
				resource.ObjectMeta.Annotations = map[string]string{
					utils.OperatorInstanceAnnotationName: "foo",
				}
				controllerutil.AddFinalizer(resource, PostgresSchemaFinalizer)
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

				controllerReconciler := &PostgresSchemaReconciler{
					Client:               k8sClient,
					Scheme:               k8sClient.Scheme(),
					PGPools:              pgpools,
					OperatorInstanceName: "foo",
				}

				pgpoolsMock["mydb"].ExpectQuery(fmt.Sprintf("^%s$", regexp.QuoteMeta(postgresql.GetSchemaSQLStatement))).
					WithArgs("myschema").
					WillReturnRows(
						pgxmock.NewRows([]string{
							"name",
							"owner",
						}),
					)

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})

				Expect(err).NotTo(HaveOccurred())
				if err := pgpoolsMock["mydb"].ExpectationsWereMet(); err != nil {
					Fail(err.Error())
				}
			})
		})

	})
})
