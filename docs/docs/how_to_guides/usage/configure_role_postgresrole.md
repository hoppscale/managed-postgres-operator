# Configure a role with PostgresRole

## TL;DR

To create a PostgreSQL role, you can use the object `PostgresRole`:

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  login: true
  secretName: myrole-credentials
```

```
postgres=# SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = 'myrole';
 rolname | rolsuper | rolinherit | rolcreaterole | rolcreatedb | rolcanlogin | rolreplication | rolbypassrls
 ---------+----------+------------+---------------+-------------+-------------+----------------+--------------
  myrole  | f        | f          | f             | f           | t           | f              | f
  (1 row)
```

In this example, a PostgreSQL role named `myrole` will be created with the permission to log in and a password.

The login credentials (IP address, port, role's name, role's password, database's name) are stored in the secret `myrole-credentials`.

## Basic usage

The only field required to create a role is `name`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
```

```
postgres=# SELECT rolname, rolsuper, rolinherit, rolcreaterole, rolcreatedb, rolcanlogin, rolreplication, rolbypassrls FROM pg_roles WHERE rolname = 'myrole';
 rolname | rolsuper | rolinherit | rolcreaterole | rolcreatedb | rolcanlogin | rolreplication | rolbypassrls
 ---------+----------+------------+---------------+-------------+-------------+----------------+--------------
  myrole  | f        | f          | f             | f           | f           | f              | f
  (1 row)
```

The role has no permission and no password.

## Setting role's options

You can configure the following role's options:

- SUPERUSER
- CREATEDB
- CREATEROLE
- INHERIT
- LOGIN
- REPLICATION
- BYPASSRLS

By default, all these options are disabled.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  createDB: true
  createRole: true
  login: true
```

## Preserving the role if the resource is deleted

You can prevent the remote PostgreSQL role to be dropped if the Kubernetes resource is being deleted.

It can be interesting to prevent an accidental deletion of the role but also when you want to configure an externaly managed role with the operator.

To do so, you can set the option `keepOnDelete` to `true`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  keepOnDelete: true
```

In this example, deleting the Kubernetes resource will not impact the remote PostgreSQL role.

## Assigning a custom password to the role

By default, the operator will generate a random password when creating or updating a role.

However, you can define a custom password through the setting `passwordFromSecret`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  passwordFromSecret:
    name: myrole-password
    key: password
```

In this example, the operator will read the password from the key `password` in the Secret `myrole-password` and assign it to the role.

## Reading my role's login credentials

With the setting `secretName`, you can export the login credentials of a role to a Secret.

A Secret will be created by the operator with a name corresponding to the value of `secretName` and containing all the information you need to log in the PostgreSQL server.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  secretName: myrole-credentials
```

In this example, the operator will create the following Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: myrole-credentials
data:
  PGDATABASE: XXXX
  PGHOST: XXXX
  PGPORT: XXXX
  PGUSER: XXXX
  PGPASSWORD: XXXX
```

## Adding custom data to the role' Secret

In addition to the default values, it's also possible to add custom values using the setting `secretTemplate`.

Your custom values can be templated using a [Go templating format](https://pkg.go.dev/text/template).

The following variables are available:

- `.Host`: the PostgreSQL's address (computed from the operator's configuration)
- `.Port`: the PostgreSQL's port (computed from the operator's configuration)
- `.Database`: the PostgreSQL's database (computed from the operator's configuration)
- `.Role`: the role's name
- `.Password`: the role's password

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  secretName: myrole-credentials
  secretTemplate:
    JDBC_URL: "jdbc:postgresql://{{ .Host }}:{{ .Port }}/{{ .Database }}?user={{ .Role }}&password={{ .Password }}"
```

In this example, the operator will create the following Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: myrole-credentials
data:
  PGDATABASE: <database>
  PGHOST: <host>
  PGPORT: <port>
  PGUSER: <role>
  PGPASSWORD: <password>
  JDBC_URL: jdbc:postgresql://<host>:<port>/<database>?user=<role>&password=<password>
```


!!! tip "Override default fields"

    If you can add fields to the Secret with `secretTemplate`, you can also override the default fields.

    For example, if you want to set a custom database name, you can set the following setting:

    ```yaml
    secretTemplate:
      PGDATABASE: mycustomdatabase
    ```

## Assigning our role to group roles

You can assign your role to other roles using the setting `memberOfRoles`.

Depending on the group roles' options, it could grant to your role additional permissions.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  memberOfRoles:
    - admin-role
```

In this example, we assign our role `myrole` to the role `admin-role`.

## Change the objects' ownership before deleting the role

You can configure the resource to change the ownership on the objects that the role owns by setting the option `onDelete.reassignOwnedTo`.

The value is the name of a role that must already exist in the PostgreSQL instance.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
spec:
  name: myrole
  onDelete:
    reassignOwnedTo: myotherrole
```
