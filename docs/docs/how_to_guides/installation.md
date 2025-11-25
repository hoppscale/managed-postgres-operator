# Installation

## Deploying with Helm

Before deploying our operator, we must create a Secret containing the credentials for our PostgreSQL server.

```shell
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: mypg-creds
stringData:
  PGDATABASE: "postgres"
  PGHOST: "127.0.0.1"
  PGPORT: "5432"
  PGUSER: "superuser"
  PGPASSWORD: "superpassword"
EOF
```

!!! note "Supported PostgreSQL variables"

    To connect to a server, the operator supports the standard [libpq](https://www.postgresql.org/docs/current/libpq-envars.html) variables (`PGHOST`, `PGUSER`, `PGPASSWORD`, etc.).

    If you want, you can also use the `DATABASE_URL` environment variable in the following format:

    ```
    postgresql://<user>:<password>@<host>:<port>/<database>
    ```


Now, we install the Managed Postgres Operator with [Helm](https://helm.sh).

```shell
helm install \
         managed-postgres-operator \
         --set 'envFrom[0].secretRef.name=mypg-creds' \
         oci://ghcr.io/hoppscale/charts/managed-postgres-operator
```


**🎉 Congratulations, the operator is now deployed and connected to your PostgreSQL server!**

## Managing multiple PostgreSQL servers

By default, the operator manages all its resources in the Kubernetes cluster and reconciles them with its PostgreSQL server.

But what if you have multiple PostgreSQL servers? It's easy, just deploy as many operators as you have servers!

On each operator, you will define a unique "instance name". Then, all you have to do is reference this "instance name" on your resources, and you're done!

### Configure the "instance name"

To set an "instance name" on an operator, you can either set the Helm value `operatorInstanceName` or add the environment variable `OPERATOR_INSTANCE`.

### Link a resource and an operator

To link a resource to an operator, you must set the annotation `managed-postgres-operator.hoppscale.com/instance`.

For example, let's say we want to create a database `mydb` on the PostgreSQL server `foo`. Then, we will create a resource `PostgresDatabase` with the annotation `managed-postgres-operator.hoppscale.com/instance=foo`.
