package controller

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

const PostgresDatabaseFinalizer = "postgresdatabase.managed-postgres-operator.hoppscale.com/finalizer"

// PostgresDatabaseReconciler reconciles a PostgresDatabase object
type PostgresDatabaseReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	logging logr.Logger

	PGPools              *postgresql.PGPools
	OperatorInstanceName string
}

// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresdatabases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresdatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresdatabases/finalizers,verbs=update
func (r *PostgresDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logging = log.FromContext(ctx)

	ctrlSuccessResult := ctrl.Result{RequeueAfter: time.Minute}
	ctrlFailResult := ctrl.Result{}

	resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}

	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		return ctrlFailResult, client.IgnoreNotFound(err)
	}

	// Skip reconcile if the resource is not managed by this operator
	if !utils.IsManagedByOperatorInstance(resource.ObjectMeta.Annotations, r.OperatorInstanceName) {
		return ctrlSuccessResult, nil
	}

	existingDatabase, err := postgresql.GetDatabase(r.PGPools.Default, resource.Spec.Name)
	if err != nil {
		return ctrlFailResult, fmt.Errorf("failed to retrieve database: %s", err)
	}

	desiredDatabase := postgresql.Database{
		Name:       resource.Spec.Name,
		Owner:      resource.Spec.Owner,
		Extensions: resource.Spec.Extensions,
	}

	//
	// Deletion logic
	//

	if resource.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(resource, PostgresDatabaseFinalizer) {
			controllerutil.AddFinalizer(resource, PostgresDatabaseFinalizer)
			if err := r.Update(ctx, resource); err != nil {
				return ctrlFailResult, err
			}
		}
	} else {
		// If there is no finalizer, delete the resource immediately
		if !controllerutil.ContainsFinalizer(resource, PostgresDatabaseFinalizer) {
			return ctrlSuccessResult, nil
		}

		err = r.reconcileOnDeletion(resource, existingDatabase)
		if err != nil {
			return ctrlFailResult, err
		}

		// Remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(resource, PostgresDatabaseFinalizer)
		if err := r.Update(ctx, resource); err != nil {
			return ctrlFailResult, err
		}

		// Stop reconciliation as the item is being deleted
		return ctrlSuccessResult, nil
	}

	//
	// Creation logic
	//

	err = r.reconcileOnCreation(existingDatabase, &desiredDatabase)
	if err != nil {
		return ctrlFailResult, err
	}

	err = r.reconcileExtensions(&desiredDatabase)
	if err != nil {
		return ctrlFailResult, err
	}

	for roleName, rolePrivileges := range resource.Spec.PrivilegesByRole {
		err = r.reconcilePrivileges(
			desiredDatabase.Name,
			roleName,
			r.convertPrivilegesSpecToList(rolePrivileges),
		)
		if err != nil {
			return ctrlFailResult, err
		}
	}

	if !resource.Status.Succeeded {
		resource.Status.Succeeded = true
		if err = r.Client.Status().Update(context.Background(), resource); err != nil {
			return ctrlFailResult, fmt.Errorf("failed to update object: %s", err)
		}
	}

	return ctrlSuccessResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase{}).
		Named("postgresdatabase").
		Complete(r)
}

// reconcileOnDeletion performs all actions related to deleting the resource
func (r *PostgresDatabaseReconciler) reconcileOnDeletion(resource *managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabase, existingDatabase *postgresql.Database) (err error) {
	if existingDatabase == nil {
		// If the remote database doesn't exist
		r.logging.Info("Database doesn't exist, skipping DROP DATABASE")
		return
	}

	if resource.Spec.KeepOnDelete {
		// If the resource is configured to keep the remote database on delete
		r.logging.Info("keepOnDelete is true, skipping DROP DATABASE")
		return
	}

	// If the resource is not configured to preserve connections to the remote database on delete
	if !resource.Spec.PreserveConnectionsOnDelete {
		err = postgresql.DropDatabaseConnections(r.PGPools.Default, existingDatabase.Name)
		if err != nil {
			r.logging.Error(err, "failed to drop connections")
			return
		}
	}

	// Drop the remote database
	err = postgresql.DropDatabase(r.PGPools.Default, existingDatabase.Name)
	if err != nil {
		r.logging.Error(err, "failed to delete database")
		return
	}

	return
}

// reconcileOnCreation performs all actions related to creating the resource
func (r *PostgresDatabaseReconciler) reconcileOnCreation(existingDatabase, desiredDatabase *postgresql.Database) (err error) {
	alterOwner := false

	if existingDatabase == nil {
		err = postgresql.CreateDatabase(r.PGPools.Default, desiredDatabase.Name)
		if err != nil {
			r.logging.Error(err, "failed to create database")
			return
		}
		r.logging.Info("Database has been created")
		alterOwner = true
	} else {
		if existingDatabase.Owner != desiredDatabase.Owner {
			alterOwner = true
		}
	}

	if alterOwner && desiredDatabase.Owner != "" {
		err = postgresql.AlterDatabaseOwner(r.PGPools.Default, desiredDatabase.Name, desiredDatabase.Owner)
		if err != nil {
			r.logging.Error(err, "failed to alter database owner")
			return
		}
		r.logging.Info(fmt.Sprintf("Owner of the database \"%s\" has been updated", desiredDatabase.Name))
	}

	return
}

// reconcileOnCreation performs all actions related to the database extensions management
func (r *PostgresDatabaseReconciler) reconcileExtensions(database *postgresql.Database) (err error) {
	err = postgresql.EnsurePGPoolExists(r.PGPools, database.Name)
	if err != nil {
		r.logging.Error(err, "failed to open pg pool")
		return err
	}

	existingExtensions, err := postgresql.GetExtensions(r.PGPools.Databases[database.Name])
	if err != nil {
		r.logging.Error(err, "failed to retrieve extensions")
		return err
	}

	// Listing extensions to drop
	for _, existingExt := range existingExtensions {
		found := false
		for _, desiredExt := range database.Extensions {
			if desiredExt == existingExt {
				found = true
				break
			}
		}

		if !found {
			err = postgresql.DropExtension(r.PGPools.Databases[database.Name], existingExt)
			if err != nil {
				r.logging.Error(err, "failed to drop extension")
				return err
			}
			r.logging.Info(fmt.Sprintf("Extension \"%s\" has been dropped from database \"%s\"", existingExt, database.Name))
		}
	}

	// Listing extensions to create
	for _, desiredExt := range database.Extensions {
		found := false
		for _, existingExt := range existingExtensions {
			if existingExt == desiredExt {
				found = true
				break
			}
		}

		if !found {
			err = postgresql.CreateExtension(r.PGPools.Databases[database.Name], desiredExt)
			if err != nil {
				r.logging.Error(err, "failed to create extension")
				return err
			}
			r.logging.Info(fmt.Sprintf("Extension \"%s\" has been created in database \"%s\"", desiredExt, database.Name))
		}
	}
	return err
}

// reconcilePrivileges performs all actions related to the database privileges for a single role
func (r *PostgresDatabaseReconciler) reconcilePrivileges(databaseName, roleName string, desiredPrivileges []string) (err error) {
	// We retrieve the existing privileges
	existingPrivileges, err := postgresql.GetDatabaseRolePrivileges(r.PGPools.Default, databaseName, roleName)
	if err != nil {
		r.logging.Error(err, "failed to retrieve privileges of database \"%s\" on role \"%s\": %s", databaseName, roleName, err)
		return err
	}

	// We grant the missing privileges
	for _, desiredPrivilege := range desiredPrivileges {
		if !slices.Contains(existingPrivileges, desiredPrivilege) {
			err := postgresql.GrantDatabaseRolePrivilege(r.PGPools.Default, databaseName, roleName, desiredPrivilege)
			if err != nil {
				r.logging.Error(err, "failed to grant \"%s\" privilege on database \"%s\" to role \"%s\"", desiredPrivilege, databaseName, roleName)
				return err
			}

			r.logging.Info(fmt.Sprintf("Privilege \"%s\" has been granted to \"%s\" on database \"%s\"", desiredPrivilege, roleName, databaseName))
		}
	}

	// We revoke the non-declared privileges
	for _, existingPrivilege := range existingPrivileges {
		if !slices.Contains(desiredPrivileges, existingPrivilege) {
			err := postgresql.RevokeDatabaseRolePrivilege(r.PGPools.Default, databaseName, roleName, existingPrivilege)
			if err != nil {
				r.logging.Error(err, "failed to revoke \"%s\" privilege on database \"%s\" to role \"%s\"", existingPrivilege, databaseName, roleName)
				return err
			}

			r.logging.Info(fmt.Sprintf("Privilege \"%s\" has been revoked from \"%s\" on database \"%s\"", existingPrivilege, roleName, databaseName))
		}
	}
	return err
}

func (r *PostgresDatabaseReconciler) convertPrivilegesSpecToList(privilegesSpec managedpostgresoperatorhoppscalecomv1alpha1.PostgresDatabasePrivilegesSpec) []string {
	privileges := []string{}
	if privilegesSpec.Create {
		privileges = append(privileges, "CREATE")
	}
	if privilegesSpec.Connect {
		privileges = append(privileges, "CONNECT")
	}
	if privilegesSpec.Temporary {
		privileges = append(privileges, "TEMPORARY")
	}
	return privileges
}
