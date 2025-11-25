# API Reference for v1alpha1

**Managed Postgres Operator's API documentation for version v1alpha1**

The package `managed-postgres-operator.hoppscale.com/v1alpha1` contains the following custom resources:

## Packages

- [PostgresDatabase](#postgresdatabase)
- [PostgresRole](#postgresrole)
- [PostgresSchema](#postgresschema)

## PostgresDatabase

PostgresDatabase represents a database in a PostgreSQL server.

| Field                                                                                                                       | Required         | Description                                                   |
|-----------------------------------------------------------------------------------------------------------------------------|------------------|---------------------------------------------------------------|
| **`apiVersion`**<br />*string*                                                                                              | :material-check: | `managed-postgres-operator.hoppscale.com/v1alpha1`            |
| **`kind`**<br />*string*                                                                                                    | :material-check: | `PostgresDatabase`                                            |
| **`metadata`**<br />*[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)* | :material-check: | Refer to Kubernetes API documentation for fields of metadata. |
| **`spec`**<br />*[PostgresDatabaseSpec](#postgresdatabasespec)*                                                             | :material-check: |                                                               |
| **`status`**<br />*[PostgresDatabaseStatus](#postgresdatabasestatus)*                                                       | :material-minus: |                                                               |

### PostgresDatabaseSpec

PostgresDatabaseSpec holds the specification of a PostgreSQL database.

| Field                                                                                              | Required         | Description                                                                                    |
|----------------------------------------------------------------------------------------------------|------------------|------------------------------------------------------------------------------------------------|
| **`name`**<br />*string* | :material-check: | The database's name. |
| **`owner`**<br />*string* | :material-close: | Database's owner role. If omitted, the owner will be the operator's role.<br />*Default: `""`* |
| **`extensions`**<br />*[]string* | :material-close: | List of the extensions to install in the database.<br />*Default: `[]`* |
| **`keepOnDelete`**<br />*bool* | :material-close: | On `true`, the Kubernetes resource deletion will not delete the associated PostgreSQL database.<br />*Default: `false`* |
| **`preserveConnectionsOnDelete`**<br />*bool* | :material-close: | On `true`, the operator will drop all connections before deleting the PostgreSQL database.<br />*Default: `false`* |
| **`privilegesByRole`**<br />*map[string][DatabasePrivilegesSpec](#postgresdatabaseprivilegesspec)* | :material-close: | For a given role, grant privileges on the database.<br />*Default: `{}`* |

### PostgresDatabasePrivilegesSpec

| Field | Required | Description |
|---|---|---|
| **`create`**<br />*bool* | :material-close: | On `true`, grant [`CREATE` privilege](https://www.postgresql.org/docs/current/ddl-priv.html#DDL-PRIV-CREATE) on the database to the role.<br />*Default: `false`* |
| **`connect`**<br />*bool* | :material-close: | On `true`, grant [`CONNECT` privilege](https://www.postgresql.org/docs/current/ddl-priv.html#DDL-PRIV-CONNECT) on the database to the role.<br />*Default: `false`* |
| **`temporary`**<br />*bool* | :material-close: | On `true`, grant [`TEMPORARY` privilege](https://www.postgresql.org/docs/current/ddl-priv.html#DDL-PRIV-TEMPORARY) on the database to the role.<br />*Default: `false`* |

### PostgresDatabaseStatus

| Field                       | Description            |
|-----------------------------|------------------------|
| **`succeeded`**<br />*bool* | Whether the database is has been successfully reconciled or not. |


## PostgresRole

PostgresRole represents a role in a PostgreSQL server.

This resource aims to implement most of the PostgreSQL role's parameters: [https://www.postgresql.org/docs/current/sql-createrole.html](https://www.postgresql.org/docs/current/sql-createrole.html).

| Field                                                                                                                       | Required         | Description                                                   |
|-----------------------------------------------------------------------------------------------------------------------------|------------------|---------------------------------------------------------------|
| **`apiVersion`**<br />*string*                                                                                              | :material-check: | `managed-postgres-operator.hoppscale.com/v1alpha1`            |
| **`kind`**<br />*string*                                                                                                    | :material-check: | `PostgresRole`                                            |
| **`metadata`**<br />*[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)* | :material-check: | Refer to Kubernetes API documentation for fields of metadata. |
| role **`spec`**<br />*[PostgresRoleSpec](#postgresrolespec)*                                                             | :material-check: |                                                               |
| **`status`**<br />*[PostgresRoleStatus](#postgresrolestatus)*                                                       | :material-minus: |                                                               |

### PostgresRoleSpec

PostgresRoleSpec holds the specification of a PostgreSQL role.

| Field | Required | Description |
|-------|----------|-------------|
| **`name`**<br />*string* | :material-check: | The role's name. |
| **`superUser`**<br />*bool* | :material-close: | On `true`, the role is a "superuser" who can override all access restrictions within the database.<br />*Default: `false`* |
| **`createDB`**<br />*bool* | :material-close: | On `true`, the role is allowed to create new databases.<br />*Default: `false`* |
| **`createRole`**<br />*bool* | :material-close: | On `true`, the role is allowed to create, alter, drop, comment on, and change the security label for other roles.<br />*Default: `false`* |
| **`inherit`**<br />*bool* | :material-close: | On `true`, the role inherit the permissions of the role of which it is member.<br />*Default: `false`* |
| **`login`**<br />*bool* | :material-close: | On `true`, the role is allowed to log in.<br />*Default: `false`* |
| **`replication`**<br />*bool* | :material-close: | On `true`, the role is a replication role.<br />*Default: `false`* |
| **`bypassRLS`**<br />*bool* | :material-close: | On `true`, the role bypasses every row-level security (RLS) policy.<br />*Default: `false`* |
| **`keepOnDelete`**<br />*bool* | :material-close: | On `true`, the Kubernetes resource deletion will not delete the associated PostgreSQL role.<br />*Default: `false`* |
| **`passwordFromSecret`**<br />*PostgresRolePasswordFromSecret* | :material-close: | Reference to a Secret containing the role's password.<br />*Default: `null`* |
| **`secretName`**<br />*string* | :material-close: | Name of the Secret the operator should create, containing the role's log in information.<br />*Default: `""`* |
| **`secretTemplate`**<br />*map[string]string* | :material-close: | Dictionnary containing the key/value to configure in the Secret created by the operator (cf. `secretName`).<br />*Default: `{}`* |
| **`memberOfRoles`**<br />*[]string* | :material-close: | List of role's names of which the role should be member of.<br />*Default: `[]`* |

### PostgresRoleStatus

| Field                       | Description            |
|-----------------------------|------------------------|
| **`succeeded`**<br />*bool* | Whether the role is has been successfully reconciled or not. |


## PostgresSchema

PostgresSchema represents a [schema](https://www.postgresql.org/docs/current/ddl-schemas.html) in a PostgreSQL server.

| Field                                                                                                                       | Required         | Description                                                   |
|-----------------------------------------------------------------------------------------------------------------------------|------------------|---------------------------------------------------------------|
| **`apiVersion`**<br />*string* | :material-check: | `managed-postgres-operator.hoppscale.com/v1alpha1` |
| **`kind`**<br />*string* | :material-check: | `PostgresSchema` |
| **`metadata`**<br />*[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)* | :material-check: | Refer to Kubernetes API documentation for fields of metadata. |
| **`spec`**<br />*[PostgresSchemaSpec](#postgresschemaspec)* | :material-check: | |
| **`status`**<br />*[PostgresRoleStatus](#postgresschemastatus)* | :material-minus: | |

### PostgresSchemaSpec

PostgresSchemaSpec holds the specification of a PostgreSQL schema.

| Field | Required | Description |
|-------|----------|-------------|
| **`database`**<br />*string* | :material-check: | The database's name containing the schema. |
| **`name`**<br />*bool* | :material-check: | The schema's name. |
| **`owner`**<br />*bool* | :material-close: | Schema's owner role. If omitted, the owner will be the database's owner.<br />*Default: `""`* |
| **`keepOnDelete`**<br />*bool* | :material-close: | On `true`, the Kubernetes resource deletion will not delete the associated PostgreSQL schema.<br />*Default: `false`* |
| **`privilegesByRole`**<br />*map[string][PostgresSchemaPrivilegesSpec](#postgresschemaprivilegesspec)* | :material-close: | For a given role, grant privileges on the schema.<br />*Default: `{}`* |

### PostgresSchemaPrivilegesSpec

| Field | Required | Description |
|---|---|---|
| **`create`**<br />*bool* | :material-close: | On `true`, grant [`CREATE` privilege](https://www.postgresql.org/docs/current/ddl-priv.html#DDL-PRIV-CREATE) on the schema to the role.<br />*Default: `false`* |
| **`usage`**<br />*bool* | :material-close: | On `true`, grant [`USAGE` privilege](https://www.postgresql.org/docs/current/ddl-priv.html#DDL-PRIV-USAGE) on the schema to the role.<br />*Default: `false`* |


### PostgresSchemaStatus

| Field                       | Description            |
|-----------------------------|------------------------|
| **`succeeded`**<br />*bool* | Whether the schema has been successfully reconciled or not. |


