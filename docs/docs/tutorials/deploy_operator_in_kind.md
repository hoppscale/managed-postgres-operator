# Deploy the operator in a local K8S cluster (with kind)

## Prerequisites

On your workstation:

- **Docker** is installed ([instructions](https://docs.docker.com/engine/install/))
- **kind** is installed ([instructions](https://kind.sigs.k8s.io/docs/user/quick-start/#installation))
- **kubectl** is installed ([instructions](https://kubernetes.io/fr/docs/tasks/tools/install-kubectl/))
- **Helm** is installed ([instructions](https://helm.sh/fr/docs/intro/install/))

## 1. Setup a Kind cluster

!!! question "What's "kind"?"

    [kind](https://kind.sigs.k8s.io/) is a tool that allows you to run a local Kubernetes cluster using Docker containers as nodes.

    As everything runs in a container, it's easy to deploy and destroy without other requirements than a Docker Engine.

First, we start a basic kind cluster:

```shell
kind create cluster --name testing-managed-pg-operator
```

Then, you should be able to validate that everything is deployed successfully using `kubectl`:

```shell
kubectl cluster-info --context kind-testing-managed-pg-operator
kubectl get pods -A
```

## 2. Deploy a PostgreSQL server

Now, we create a Namespace named `pg-servers`, a Pod containing a PostgreSQL server and a Service.

```shell
kubectl create namespace pg-servers
cat <<EOF | kubectl apply -f -
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgresql
  namespace: pg-servers
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      containers:
        - name: postgres
          image: postgres:16
          args: ["-c", "log_statement=all"]
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: postgres
            - name: POSTGRES_USER
              value: admin
            - name: POSTGRES_PASSWORD
              value: admin
---
apiVersion: v1
kind: Service
metadata:
  name: postgresql
  namespace: pg-servers
  labels:
    app: postgresql
spec:
  type: ClusterIP
  ports:
    - port: 5432
      targetPort: 5432
      protocol: TCP
  selector:
    app: postgresql
EOF
```

## 3. Deploy the Managed PostgreSQL Operator

To get started, we'll create the Namespace in which we'll deploy the operator.

```shell
kubectl create namespace managed-postgres-operator
```

Then, let's create the Secret containing the PostgreSQL's credentials.

```shell
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: Secret
metadata:
  name: postgresql-creds
  namespace: managed-postgres-operator
stringData:
  PGDATABASE: "postgres"
  PGHOST: "postgresql.pg-servers.svc"
  PGPORT: "5432"
  PGUSER: "admin"
  PGPASSWORD: "admin"
EOF
```

Now, everything is ready to deploy the Managed Postgres Operator with Helm.

```shell
helm install \
         managed-postgres-operator \
         -n managed-postgres-operator \
         --set 'envFrom[0].secretRef.name=postgresql-creds' \
         oci://ghcr.io/hoppscale/charts/managed-postgres-operator
```

The operator will start and begin to watch its custom resources.

## 4. Deploy your first database and role

Let's play with the operator now!

We will deploy a database and a role to connect to it. 

```shell
kubectl create namespace myproject
cat <<EOF | kubectl apply -f -
---
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresRole
metadata:
  name: myrole
  namespace: myproject
spec:
  name: myrole
  login: true
  secretName: pg-credentials-myrole
---
apiVersion: managed-postgres-operator.hoppscale.com/v1alpha1
kind: PostgresDatabase
metadata:
  name: mydb
  namespace: myproject
spec:
  name: mydb
  owner: myrole
  extensions:
    - plpgsql
EOF
```

!!! question "How to know if a resource is reconciled?"

	You can check if a resource has been successfully reconciled by looking at the field `.status.succeeded`.

## 5. Connect to your database with your role

Once our database and its role have been created, we will deploy a psql container within our cluster.

```shell
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: psql-client
  namespace: myproject
spec:
  replicas: 1
  selector:
    matchLabels:
      app: psql-client
  template:
    metadata:
      labels:
        app: psql-client
    spec:
      containers:
        - name: psql-client
          image: docker.io/alpine/psql
          command:
            - sh
          args:
            - -c
            - 'while sleep 3600; do :; done'
          envFrom:
            - secretRef:
                name: pg-credentials-myrole
          env:
            - name: PGDATABASE
              value: mydb
EOF
```

Once the pod is running, you can open a shell and execute `psql`:

```shell
kubectl exec -it deployment/psql-client -n myproject -- sh
# / # psql
# psql (17.6, server 16.11 (Debian 16.11-1.pgdg13+1))
# Type "help" for help.
# 
# mydb=> 
```

**🎉 Congratulations! You're now connected to your database, with your role!**
