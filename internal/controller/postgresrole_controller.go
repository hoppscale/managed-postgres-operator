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
	"bytes"
	"context"
	"fmt"
	"math/rand/v2"
	"text/template"
	"time"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	managedpostgresoperatorhoppscalecomv1alpha1 "github.com/hoppscale/managed-postgres-operator/api/v1alpha1"
	"github.com/hoppscale/managed-postgres-operator/internal/postgresql"
	"github.com/hoppscale/managed-postgres-operator/internal/utils"
	"github.com/jackc/pgx/v5"
)

const PostgresRoleFinalizer = "postgresrole.managed-postgres-operator.hoppscale.com/finalizer"

// PostgresRoleReconciler reconciles a PostgresRole object
type PostgresRoleReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	logging logr.Logger

	RequeueInterval time.Duration

	PGPools              *postgresql.PGPools
	OperatorInstanceName string

	CacheRolePasswords map[string]string
}

// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=managed-postgres-operator.hoppscale.com,resources=postgresroles/finalizers,verbs=update
func (r *PostgresRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logging = log.FromContext(ctx)

	resource := &managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}

	if err := r.Client.Get(ctx, req.NamespacedName, resource); err != nil {
		return r.Result(client.IgnoreNotFound(err))
	}

	// Skip reconcile if the resource is not managed by this operator
	if !utils.IsManagedByOperatorInstance(resource.ObjectMeta.Annotations, r.OperatorInstanceName) {
		return r.Result(nil)
	}

	rolePassword, err := r.retrieveRolePassword(resource)
	if err != nil {
		return r.Result(err)
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
		return r.Result(fmt.Errorf("failed to get role: %s", err))
	}

	operatorRole, err := postgresql.GetRole(r.PGPools.Default, r.PGPools.Default.Config().ConnConfig.User)
	if err != nil {
		return r.Result(fmt.Errorf("failed to get operator's role: %s", err))
	}

	if resource.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(resource, PostgresRoleFinalizer) {
			controllerutil.AddFinalizer(resource, PostgresRoleFinalizer)
			if err := r.Update(ctx, resource); err != nil {
				return r.Result(err)
			}
		}
	} else {

		//
		// Deletion logic
		//

		// If there is no finalizer, delete the resource immediately
		if !controllerutil.ContainsFinalizer(resource, PostgresRoleFinalizer) {
			return r.Result(nil)
		}

		err = r.reconcileOnDeletion(existingRole, resource.Spec.KeepOnDelete, resource.Spec.OnDelete)
		if err != nil {
			return r.Result(err)
		}

		// Remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(resource, PostgresRoleFinalizer)
		if err := r.Update(ctx, resource); err != nil {
			return r.Result(err)
		}

		// Stop reconciliation as the item is being deleted
		return r.Result(nil)
	}

	//
	// Creation logic
	//

	err = r.reconcileOnCreation(operatorRole, existingRole, &desiredRole)
	if err != nil {
		return r.Result(err)
	}

	err = r.reconcileRoleMembership(desiredRole.Name, resource.Spec.MemberOfRoles)
	if err != nil {
		return r.Result(err)
	}

	err = r.reconcileRoleSecret(
		resource.ObjectMeta.Namespace,
		resource.Spec.SecretName,
		resource.Spec.SecretTemplate,
		&desiredRole,
		r.PGPools.Default.Config().ConnConfig,
	)
	if err != nil {
		return r.Result(err)
	}

	if !resource.Status.Succeeded {
		resource.Status.Succeeded = true
		if err = r.Client.Status().Update(context.Background(), resource); err != nil {
			return r.Result(fmt.Errorf("failed to update object: %s", err))
		}
	}

	return r.Result(nil)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole{}).
		Named("postgresrole").
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](time.Second, r.RequeueInterval),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
		}).
		Complete(r)
}

// Result builds reconciler result depending on error
func (r *PostgresRoleReconciler) Result(err error) (ctrl.Result, error) {
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
}

// reconcileOnDeletion performs all actions related to deleting the resource
func (r *PostgresRoleReconciler) reconcileOnDeletion(existingRole *postgresql.Role, keepOnDelete bool, onDeleteOptions *managedpostgresoperatorhoppscalecomv1alpha1.PostgresRoleOnDeleteSpec) (err error) {
	if existingRole == nil {
		r.logging.Info("Role doesn't exist, skipping DROP ROLE")
		return nil
	}

	if keepOnDelete {
		// If the resource is configured to keep the remote role on delete
		r.logging.Info("keepOnDelete is true, skipping DROP ROLE")
		return nil
	}

	if onDeleteOptions != nil {
		if onDeleteOptions.ReassignOwnedTo != "" {
			databases, err := postgresql.ListDatabases(r.PGPools.Default)
			if err != nil {
				return fmt.Errorf("failed to list databases: %s", err)
			}

			for _, database := range databases {
				err := postgresql.EnsurePGPoolExists(r.PGPools, database)
				if err != nil {
					return fmt.Errorf("failed to open pg pool: %s", err)
				}

				err = postgresql.ReassignOwnedToRole(r.PGPools.Databases[database], existingRole.Name, onDeleteOptions.ReassignOwnedTo)
				if err != nil {
					return fmt.Errorf("failed to reassign owned objects in database before deletion: %s", err)
				}
			}
			r.logging.Info(fmt.Sprintf("Objects owned by '%s' have been reassigned to '%s'", existingRole.Name, onDeleteOptions.ReassignOwnedTo))
		}
	}

	err = postgresql.DropRole(r.PGPools.Default, existingRole.Name)
	if err != nil {
		return fmt.Errorf("failed to delete role: %s", err)
	}

	r.logging.Info("Role has been deleted")

	return nil
}

// reconcileOnCreation performs all actions related to creating the resource
func (r *PostgresRoleReconciler) reconcileOnCreation(operatorRole, existingRole, desiredRole *postgresql.Role) (err error) {
	if existingRole == nil {
		err = postgresql.CreateRole(r.PGPools.Default, operatorRole, desiredRole)
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
		err = postgresql.AlterRole(r.PGPools.Default, operatorRole, existingRole, desiredRole)
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

func (r *PostgresRoleReconciler) reconcileRoleSecret(secretNamespace, secretName string, secretTemplate map[string]string, role *postgresql.Role, pgConfig *pgx.ConnConfig) (err error) {
	// Do not create Secret if no name provided by the user
	if secretName == "" {
		return err
	}

	secretNamespacedName := types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}

	resourceSecret := &corev1.Secret{}

	secretDataTemplateVars := struct {
		Role     string
		Password string
		Host     string
		Port     string
		Database string
	}{
		Role:     role.Name,
		Password: role.Password,
		Host:     pgConfig.Host,
		Port:     fmt.Sprintf("%d", pgConfig.Port),
		Database: pgConfig.Database,
	}

	desiredSecretData := map[string][]byte{
		"PGUSER":     []byte(secretDataTemplateVars.Role),
		"PGPASSWORD": []byte(secretDataTemplateVars.Password),
		"PGHOST":     []byte(secretDataTemplateVars.Host),
		"PGPORT":     []byte(secretDataTemplateVars.Port),
		"PGDATABASE": []byte(secretDataTemplateVars.Database),
	}

	for secretKey, secretValue := range secretTemplate {
		t := template.Must(template.New("secret").Parse(secretValue))
		var tpl bytes.Buffer
		if err := t.Execute(&tpl, secretDataTemplateVars); err != nil {
			return fmt.Errorf("failed to render secret template: %s", err)
		}
		desiredSecretData[secretKey] = tpl.Bytes()
	}

	// Retrieve Secret
	err = r.Client.Get(context.Background(), secretNamespacedName, resourceSecret)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve secret: %s", err)
	}

	// If Secret is not found, create it
	if errors.IsNotFound(err) {
		resourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: secretNamespace,
				Name:      secretName,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "managed-postgres-operator.hoppscale.com",
				},
			},
			Type: "Opaque",
			Data: desiredSecretData,
		}

		err = r.Client.Create(context.Background(), resourceSecret)
		if err != nil {
			return fmt.Errorf("failed to create secret: %s", err)
		}

		r.logging.Info("Role's secret has been created")

		return err
	}

	// Update secret if needed
	toUpdate := false
	if val, ok := resourceSecret.ObjectMeta.Labels["app.kubernetes.io/managed-by"]; !ok || val != "managed-postgres-operator.hoppscale.com" {
		if resourceSecret.ObjectMeta.Labels == nil {
			resourceSecret.ObjectMeta.Labels = make(map[string]string)
		}
		resourceSecret.ObjectMeta.Labels["app.kubernetes.io/managed-by"] = "managed-postgres-operator.hoppscale.com"
		toUpdate = true
	}

	if fmt.Sprint(resourceSecret.Data) != fmt.Sprint(desiredSecretData) {
		toUpdate = true
		resourceSecret.Data = desiredSecretData
	}

	if toUpdate {
		err = r.Client.Update(context.Background(), resourceSecret)
		if err != nil {
			return fmt.Errorf("failed to update secret: %s", err)
		}
		r.logging.Info("Role's secret has been updated")
	}

	return err
}

func (r *PostgresRoleReconciler) generatePassword(length int) (password string) {
	// Generate password
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	random := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

	result := make([]byte, length)
	for i := range result {
		result[i] = charset[random.IntN(len(charset))]
	}

	return string(result)
}

func (r *PostgresRoleReconciler) retrieveRolePassword(resource *managedpostgresoperatorhoppscalecomv1alpha1.PostgresRole) (password string, err error) {
	// Retrieve password from user-provided Secret
	if resource.Spec.PasswordFromSecret != nil {
		secretNamespacedName := types.NamespacedName{
			Namespace: resource.ObjectMeta.Namespace,
			Name:      resource.Spec.PasswordFromSecret.Name,
		}

		resourceSecret := &corev1.Secret{}

		err := r.Client.Get(context.Background(), secretNamespacedName, resourceSecret)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve password from secret `%s`: %s", secretNamespacedName, err)
		}

		password, ok := resourceSecret.Data[resource.Spec.PasswordFromSecret.Key]
		if !ok {
			err = fmt.Errorf("failed to retrieve password from secret `%s`: key `%s` doesn't exist", secretNamespacedName, resource.Spec.PasswordFromSecret.Key)
		}
		return string(password), err
	}

	// Retrieve password from generated Secret
	if resource.Spec.SecretName != "" {
		secretNamespacedName := types.NamespacedName{
			Namespace: resource.ObjectMeta.Namespace,
			Name:      resource.Spec.SecretName,
		}

		resourceSecret := &corev1.Secret{}

		err := r.Client.Get(context.Background(), secretNamespacedName, resourceSecret)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				return "", fmt.Errorf("failed to retrieve password from secret `%s`: %s", secretNamespacedName, err)
			}
		} else {
			// Retrieve password from the Secret
			password, ok := resourceSecret.Data["PGPASSWORD"]
			if !ok {
				err = fmt.Errorf("failed to retrieve password from secret `%s`: key `%s` doesn't exist", secretNamespacedName, "PGPASSWORD")
			}
			return string(password), err
		}
	}

	// Retrieve password from cache
	if val, ok := r.CacheRolePasswords[resource.Spec.Name]; ok {
		return val, nil
	}

	// Generate a new password if an existing one cannot be retrieve
	return r.generatePassword(64), nil
}
