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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
)

const PostgresRoleFinalizer = "postgresrole.managed-postgres-operator.hoppscale.com/finalizer"

// PostgresRoleReconciler reconciles a PostgresRole object
type PostgresRoleReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	logging logr.Logger

	PGPools              *postgresql.PGPools
	OperatorInstanceName string

	CacheRolePasswords map[string]string
}

// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles/finalizers,verbs=update
func (r *PostgresRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logging = log.FromContext(ctx)

	ctrlSuccessResult := ctrl.Result{RequeueAfter: time.Minute}
	ctrlFailResult := ctrl.Result{RequeueAfter: time.Second}

	resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}

	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		return ctrlFailResult, client.IgnoreNotFound(err)
	}

	// Skip reconcile if the resource is not managed by this operator
	if !utils.IsManagedByOperatorInstance(resource.ObjectMeta.Annotations, r.OperatorInstanceName) {
		return ctrlSuccessResult, nil
	}

	rolePassword := ""

	if resource.Spec.PasswordSecretName != "" {
		secretNamespacedName := types.NamespacedName{
			Namespace: resource.ObjectMeta.Namespace,
			Name:      resource.Spec.PasswordSecretName,
		}

		resourceSecret := &corev1.Secret{}

		if err := r.Client.Get(ctx, secretNamespacedName, resourceSecret); err != nil {
			return ctrlFailResult, client.IgnoreNotFound(err)
		}

		rolePassword = string(resourceSecret.Data["password"])
	}

	desiredRole := postgresql.Role{
		Name:        resource.Spec.Name,
		SuperUser:   resource.Spec.SuperUser,
		Inherit:     resource.Spec.Inherit,
		CreateRole:  resource.Spec.CreateRole,
		CreateDB:    resource.Spec.CreateDB,
		Login:       resource.Spec.Login,
		Replication: resource.Spec.Replication,
		BypassRLS:   resource.Spec.BypassRLS,
		Password:    rolePassword,
	}

	existingRole, err := postgresql.GetRole(r.PGPools.Default, resource.Spec.Name)
	if err != nil {
		return ctrlFailResult, fmt.Errorf("failed to get role: %s", err)
	}

	//
	// Deletion logic
	//

	if resource.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(resource, PostgresRoleFinalizer) {
			controllerutil.AddFinalizer(resource, PostgresRoleFinalizer)
			if err := r.Update(ctx, resource); err != nil {
				return ctrlFailResult, err
			}
		}
	} else {
		// If there is no finalizer, delete the resource immediately
		if !controllerutil.ContainsFinalizer(resource, PostgresRoleFinalizer) {
			return ctrlSuccessResult, nil
		}

		err = r.reconcileOnDeletion(existingRole)
		if err != nil {
			return ctrlFailResult, err
		}

		// Remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(resource, PostgresRoleFinalizer)
		if err := r.Update(ctx, resource); err != nil {
			return ctrlFailResult, err
		}

		// Stop reconciliation as the item is being deleted
		return ctrlSuccessResult, nil
	}

	//
	// Creation logic
	//

	err = r.reconcileOnCreation(existingRole, &desiredRole)
	if err != nil {
		return ctrlFailResult, err
	}

	err = r.reconcileRoleMembership(desiredRole.Name, resource.Spec.MemberOfRoles)
	if err != nil {
		return ctrlFailResult, err
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
func (r *PostgresRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}).
		Named("postgresrole").
		Complete(r)
}

// reconcileOnDeletion performs all actions related to deleting the resource
func (r *PostgresRoleReconciler) reconcileOnDeletion(existingRole *postgresql.Role) (err error) {
	if existingRole == nil {
		r.logging.Info("Role doesn't exist, skipping DROP ROLE")
		return
	}

	err = postgresql.DropRole(r.PGPools.Default, existingRole.Name)
	if err != nil {
		r.logging.Error(err, "failed to delete role")
		return
	}

	r.logging.Info("Role has been deleted")

	return
}

// reconcileOnCreation performs all actions related to creating the resource
func (r *PostgresRoleReconciler) reconcileOnCreation(existingRole, desiredRole *postgresql.Role) (err error) {
	if existingRole == nil {
		err = postgresql.CreateRole(r.PGPools.Default, desiredRole)
		if err != nil {
			r.logging.Error(err, "failed to create role")
			return err
		}
		r.logging.Info("Role has been created")

		r.CacheRolePasswords[desiredRole.Name] = desiredRole.Password

		return err
	}

	needUpdate := false

	// Update the role if the desired role password is different than the one in cache
	if desiredRole.Password != r.CacheRolePasswords[desiredRole.Name] {
		needUpdate = true
		r.logging.Info("Desired role's password and the cached password are different, an update is needed")
	}

	copyDesiredRole := *desiredRole
	copyDesiredRole.Password = ""

	// Update the role if the the existing role is different than the desired role
	if *existingRole != copyDesiredRole {
		needUpdate = true
		r.logging.Info("Existing role and desired role are different, an update is needed")
	}

	if needUpdate {
		err = postgresql.AlterRole(r.PGPools.Default, desiredRole)
		if err != nil {
			r.logging.Error(err, "failed to alter role")
			return err
		}
		r.logging.Info("Role has been updated")

		r.CacheRolePasswords[desiredRole.Name] = desiredRole.Password
	}

	return err
}

func (r *PostgresRoleReconciler) reconcileRoleMembership(role string, desiredMembership []string) (err error) {
	// Listing current membership
	existingRoleMembership, err := postgresql.GetRoleMembership(r.PGPools.Default, role)
	if err != nil {
		r.logging.Error(err, "failed to retrieve role's membership")
		return err
	}

	// Revoking membership
	for _, existingGroupRole := range existingRoleMembership {
		found := false
		for _, desiredGroupRole := range desiredMembership {
			if desiredGroupRole == existingGroupRole {
				found = true
				break
			}
		}

		if !found {
			err = postgresql.RevokeRoleMembership(r.PGPools.Default, existingGroupRole, role)
			if err != nil {
				r.logging.Error(err, "failed to revoke role membership")
				return err
			}
			r.logging.Info(fmt.Sprintf("Role \"%s\" has been revoked from the group \"%s\"", role, existingGroupRole))
		}
	}

	// Granting membership
	for _, desiredGroupRole := range desiredMembership {
		found := false
		for _, existingGroupRole := range existingRoleMembership {
			if desiredGroupRole == existingGroupRole {
				found = true
				break
			}
		}

		if !found {
			err = postgresql.GrantRoleMembership(r.PGPools.Default, desiredGroupRole, role)
			if err != nil {
				r.logging.Error(err, "failed to grant role membership")
				return err
			}
			r.logging.Info(fmt.Sprintf("Role \"%s\" has been granted to the group \"%s\"", role, desiredGroupRole))
		}
	}

	return err
}
