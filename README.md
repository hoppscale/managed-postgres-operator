# managed-postgres-operator

Managed Postgres Operator aims to manage PostgreSQL resources like databases, roles or functions, directly from a Kubernetes cluster.

## Usage

### PostgresDatabase

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb # Database's name
  owner: myrole # Database owner role
  extensions: # List of extensions to install
    - plpgsql
  keepDatabaseOnDelete: true # Should the database be kept if the Kubernetes resource is deleted?
  preserveConnectionsOnDelete: false # Should the operator wait until the open connections are closed before deleting the database?
```

### PostgresRole

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole # Role's name
  superUser: false # Should the role be a superuser?
  createDB: false # Should the role be able to create databases?
  createRole: false # Should the role be able to create roles?
  inherit: false # Should the role inherit the permissions of the role of which it is a member?
  login: false # Should the role be able to log in?
  replication: false # Is the role used for replication?
  bypassRLS: false # Should the role bypass the defined row-level security (RLS) policies?
  passwordSecretName: "my-secret" # Name of the secret from where the role's password should be retrieved under the key `password`
  memberOfRoles: # List of roles the role should be member of
    - anotherRole
```
