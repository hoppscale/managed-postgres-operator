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
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

const PostgresSchemaFinalizer = "postgresschema.managed-postgres-operator.hoppscale.com/finalizer"

// PostgresSchemaReconciler reconciles a PostgresSchema object
type PostgresSchemaReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	logging logr.Logger

	PGPools              *postgresql.PGPools
	OperatorInstanceName string
}

// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresschemas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresschemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresschemas/finalizers,verbs=update
func (r *PostgresSchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logging = log.FromContext(ctx)

	ctrlSuccessResult := ctrl.Result{RequeueAfter: time.Minute}
	ctrlFailResult := ctrl.Result{}

	resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}

	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		return ctrlFailResult, client.IgnoreNotFound(err)
	}

	// Skip reconcile if the resource is not managed by this operator
	if !utils.IsManagedByOperatorInstance(resource.ObjectMeta.Annotations, r.OperatorInstanceName) {
		return ctrlSuccessResult, nil
	}

	err := postgresql.EnsurePGPoolExists(r.PGPools, resource.Spec.Database)
	if err != nil {
		r.logging.Error(err, "failed to open pg pool")
		return ctrlFailResult, err
	}

	existingSchema, err := postgresql.GetSchema(r.PGPools.Databases[resource.Spec.Database], resource.Spec.Name)
	if err != nil {
		return ctrlFailResult, fmt.Errorf("failed to retrieve schema: %s", err)
	}

	if existingSchema != nil {
		existingSchema.Database = resource.Spec.Database
	}

	desiredSchema := postgresql.Schema{
		Database: resource.Spec.Database,
		Name:     resource.Spec.Name,
		Owner:    resource.Spec.Owner,
	}

	//
	// Deletion logic
	//

	if resource.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(resource, PostgresSchemaFinalizer) {
			controllerutil.AddFinalizer(resource, PostgresSchemaFinalizer)
			if err := r.Update(ctx, resource); err != nil {
				return ctrlFailResult, err
			}
		}
	} else {
		// If there is no finalizer, delete the resource immediately
		if !controllerutil.ContainsFinalizer(resource, PostgresSchemaFinalizer) {
			return ctrlSuccessResult, nil
		}

		err = r.reconcileOnDeletion(existingSchema, resource.Spec.KeepOnDelete)
		if err != nil {
			return ctrlFailResult, err
		}

		// Remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(resource, PostgresSchemaFinalizer)
		if err := r.Update(ctx, resource); err != nil {
			return ctrlFailResult, err
		}

		// Stop reconciliation as the item is being deleted
		return ctrlSuccessResult, nil
	}

	//
	// Creation logic
	//

	err = r.reconcileOnCreation(existingSchema, &desiredSchema)
	if err != nil {
		return ctrlFailResult, err
	}

	if !resource.Status.Succeeded {
		resource.Status.Succeeded = true
		if err = r.Client.Status().Update(context.Background(), resource); err != nil {
			return ctrlFailResult, fmt.Errorf("failed to update object: %s", err)
		}
	}

	for roleName, rolePrivileges := range resource.Spec.PrivilegesByRole {
		err = r.reconcilePrivileges(
			desiredSchema.Database,
			desiredSchema.Name,
			roleName,
			r.convertPrivilegesSpecToList(rolePrivileges),
		)
		if err != nil {
			return ctrlFailResult, err
		}
	}

	return ctrlSuccessResult, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresSchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchema{}).
		Named("postgresschema").
		Complete(r)
}

// reconcileOnDeletion performs all actions related to deleting the resource
func (r *PostgresSchemaReconciler) reconcileOnDeletion(schema *postgresql.Schema, keepOnDelete bool) (err error) {
	if schema == nil {
		// If the remote schema doesn't exists
		r.logging.Info("Schema doesn't exist, skipping DROP SCHEMA")
		return
	}

	if keepOnDelete {
		// If the resource is configured to keep the remote schema on delete
		r.logging.Info("keepOnDelete is true, skipping DROP SCHEMA")
		return
	}

	// Drop the schema
	err = postgresql.DropSchema(r.PGPools.Databases[schema.Database], schema.Name)
	if err != nil {
		r.logging.Error(err, "failed to delete schema")
		return
	}

	r.logging.Info("Schema has been deleted")

	return
}

// reconcileOnCreation performs all actions related to creating the resource
func (r *PostgresSchemaReconciler) reconcileOnCreation(existingSchema, desiredSchema *postgresql.Schema) (err error) {
	alterOwner := false

	if existingSchema == nil {
		err = postgresql.CreateSchema(r.PGPools.Databases[desiredSchema.Database], desiredSchema.Name)
		if err != nil {
			r.logging.Error(err, "failed to create schema")
			return err
		}
		r.logging.Info("Schema has been created")
		alterOwner = true
	} else {
		if existingSchema.Owner != desiredSchema.Owner {
			alterOwner = true
		}
	}

	if alterOwner && desiredSchema.Owner != "" {
		err = postgresql.AlterSchemaOwner(
			r.PGPools.Databases[desiredSchema.Database],
			desiredSchema.Name,
			desiredSchema.Owner,
		)
		if err != nil {
			r.logging.Error(err, "failed to alter schema owner")
			return err
		}
		r.logging.Info(fmt.Sprintf("Owner of the schema \"%s\" has been updated", desiredSchema.Name))
	}

	return err
}

// reconcilePrivileges performs all actions related to the schema privileges for a single role
func (r *PostgresSchemaReconciler) reconcilePrivileges(databaseName, schemaName, roleName string, desiredPrivileges []string) (err error) {
	// We retrieve the existing privileges
	existingPrivileges, err := postgresql.GetSchemaRolePrivileges(r.PGPools.Databases[databaseName], schemaName, roleName)
	if err != nil {
		r.logging.Error(err, fmt.Sprintf("failed to retrieve privileges of schema \"%s\" in database \"%s\" on role \"%s\": %s", schemaName, databaseName, roleName, err))
		return err
	}

	// We grant the missing privileges
	for _, desiredPrivilege := range desiredPrivileges {
		if !slices.Contains(existingPrivileges, desiredPrivilege) {
			err := postgresql.GrantSchemaRolePrivilege(r.PGPools.Databases[databaseName], schemaName, roleName, desiredPrivilege)
			if err != nil {
				r.logging.Error(err, fmt.Sprintf("failed to grant \"%s\" privilege on schema \"%s\" in database \"%s\" to role \"%s\"", desiredPrivilege, schemaName, databaseName, roleName))
				return err
			}

			r.logging.Info(fmt.Sprintf("Privilege \"%s\" has been granted to \"%s\" on schema \"%s\" in database \"%s\"", desiredPrivilege, roleName, schemaName, databaseName))
		}
	}

	// We revoke the non-declared privileges
	for _, existingPrivilege := range existingPrivileges {
		if !slices.Contains(desiredPrivileges, existingPrivilege) {
			err := postgresql.RevokeSchemaRolePrivilege(r.PGPools.Databases[databaseName], schemaName, roleName, existingPrivilege)
			if err != nil {
				r.logging.Error(err, fmt.Sprintf("failed to revoke \"%s\" privilege on schema \"%s\" in database \"%s\" to role \"%s\"", existingPrivilege, schemaName, databaseName, roleName))
				return err
			}

			r.logging.Info(fmt.Sprintf("Privilege \"%s\" has been revoked from \"%s\" on schema \"%s\" in database \"%s\"", existingPrivilege, roleName, schemaName, databaseName))
		}
	}
	return err
}

func (r *PostgresSchemaReconciler) convertPrivilegesSpecToList(privilegesSpec managedpostgresoperatorhoppscalecomv1alpha1.PostgresSchemaPrivilegesSpec) []string {
	privileges := []string{}
	if privilegesSpec.Create {
		privileges = append(privileges, "CREATE")
	}
	if privilegesSpec.Usage {
		privileges = append(privileges, "USAGE")
	}
	return privileges
}
