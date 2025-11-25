# Configure a schema with PostgresSchema

## TL;DR

To create a PostgreSQL schema, you can use the object `PostgresSchema`:

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresSchema
metadata:
  name: myschema
spec:
  database: mydb
  name: myschema
  owner: myrole
```

```
mydb=> SELECT schema_name as name, schema_owner as owner FROM information_schema.schemata WHERE schema_name = 'myschema';
   name   | owner  
----------+--------
 myschema | myrole
(1 row)
```

In this example, a PostgreSQL schema named `myschema` has been created.

## Basic usage

To create a schema, the only two required fields are:

- `database`: the database's name in which to create the schema
- `name`: the schema's name

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresSchema
metadata:
  name: myschema
spec:
  database: mydb
  name: myschema
```

```
mydb=> SELECT schema_name as name, schema_owner as owner FROM information_schema.schemata WHERE schema_name = 'myschema';
   name   | owner 
----------+-------
 myschema | admin
(1 rows)
```

## Setting the schema owner

If you want to change the default owner of the schema, you can configure the field `owner` with the name of an existing role.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresSchema
metadata:
  name: myschema
spec:
  database: mydb
  name: myschema
  owner: myrole
```

```
mydb=> SELECT schema_name as name, schema_owner as owner FROM information_schema.schemata WHERE schema_name = 'myschema';
   name   | owner  
----------+--------
 myschema | myrole
(1 row)
```

In this example, the schema owner is then `myrole`.

## Preserving the schema if the resource is deleted

You can prevent the remote PostgreSQL schema to be dropped if the Kubernetes resource is being deleted.

It can be interesting to prevent an accidental deletion of the role but also when you want to configure an externaly managed role with the operator.

To do so, you can set the option `keepOnDelete` to `true`.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresSchema
metadata:
  name: myschema
spec:
  database: mydb
  name: myschema
  keepOnDelete: true
```

In this example, deleting the Kubernetes resource will not impact the remote PostgreSQL schema.

## Granting privileges to roles

You can grant schema privileges to you roles with the setting `privilegesByRole`.

It's a dictionnary with the role's name as key and the privileges as value.

```yaml
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresSchema
metadata:
  name: myschema
spec:
  database: mydb
  name: myschema
  privilegesByRole:
    my-read-only-role:
      usage: true
    my-admin-role:
      create: true
      usage: true
```

In this example:

- We grant the `USAGE` privileges to our role `my-read-only-role`.
- We grant the `CREATE` and `USAGE` privileges to our role `my-admin-role`.

*For more details regarding the available privileges, please refer to the [API reference](../../reference/api/v1alpha1/index.md#postgresschemaprivilegesspec).*
