# Configure a database with PostgresDatabase

## TL;DR

To create a PostgreSQL database, you can use the object `PostgresDatabase`:

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  owner: myrole
  extensions:
    - plpgsql
  keepOnDelete: true
```

```
postgres=# \l
                                                   List of databases
   Name    | Owner    | Encoding | Locale Provider |  Collate   |   Ctype    | ICU Locale | ICU Rules | Access privileges
-----------+----------+----------+-----------------+------------+------------+------------+-----------+-------------------
 mydb      | myrole   | UTF8     | libc            | en_US.utf8 | en_US.utf8 |            |           |
```

In this example, a PostgreSQL database named `mydb` will be created with, as owner, the role `myrole` and the extension `plpgsql`.

On deletion, the remote database will not be dropped.

## Basic usage

The only field required to create a database is `name`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
```

```
postgres=# \l
                                                   List of databases
   Name    | Owner    | Encoding | Locale Provider |  Collate   |   Ctype    | ICU Locale | ICU Rules | Access privileges
-----------+----------+----------+-----------------+------------+------------+------------+-----------+-------------------
 mydb      | postgres | UTF8     | libc            | en_US.utf8 | en_US.utf8 |            |           |
```

The database's owner will then be the operator's role (here `postgres`).

No extension will be configured on the database. If some extensions are automatically set, they will be removed.

## Setting database owner

If you want to change the default owner of the database, you can configure the field `owner` with the name of an existing role.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  owner: myrole
```

```
postgres=# \l
                                                   List of databases
   Name    | Owner    | Encoding | Locale Provider |  Collate   |   Ctype    | ICU Locale | ICU Rules | Access privileges
-----------+----------+----------+-----------------+------------+------------+------------+-----------+-------------------
 mydb      | myrole   | UTF8     | libc            | en_US.utf8 | en_US.utf8 |            |           |
```

Here, the database owner is then `myrole`.

## Setting database extensions

To enable extensions in your database, you can list them in the field `extensions`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  extensions:
    - plpgsql
```

```
mydb=# \dx
                 List of installed extensions
  Name   | Version |   Schema   |         Description
---------+---------+------------+------------------------------
 plpgsql | 1.0     | pg_catalog | PL/pgSQL procedural language
```

Here, only one extension is enabled : `plpgsql`. The extension's version cannot be configured.

## Preserving the database if the resource is deleted

You can prevent the remote PostgreSQL database to be dropped if the Kubernetes resource is being deleted.

It can be interesting to prevent an accidental deletion of the database but also when you want to configure an externaly managed database with the operator.

To do so, you can set the option `keepOnDelete` to `true`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  keepOnDelete: true
```

In this example, deleting the Kubernetes resource will not impact the remote PostgreSQL database.

## Preserving the open connections when dropping database

It's common to see the `DROP DATABASE` command fail because the database still has open connections.

By default, the operator will drop all connections on deletion.

But, if you want to preserve the open connections, you can set the option `preserveConnectionsOnDelete` to `true`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  preserveConnectionsOnDelete: true
```

In this example, the Kubernetes resource may remain in _deletion_ for some time, as you will have to wait for the connections to be closed manually.

## Granting privileges to roles

You can grant database privileges to you roles with the setting `privilegesByRole`.

It's a dictionnary with the role's name as key and the privileges as value.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
spec:
  name: mydb
  privilegesByRole:
    my-read-only-role:
      connect: true
      temporary: true
    my-admin-role:
      create: true
      connect: true
      temporary: true
```

In this example:

- We grant the `CONNECT` and `TEMPORARY` privileges to our role `my-read-only-role`.
- We grant the `CREATE`, `CONNECT` and `TEMPORARY` privileges to our role `my-admin-role`.

*For more details regarding the available privileges, please refer to the [API reference](../../reference/api/v1alpha1/index.md#postgresdatabaseprivilegesspec).*
