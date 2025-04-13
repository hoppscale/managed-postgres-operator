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
