---
title: Managed Postgres Operator
hide:
  - navigation
  - tabs
  - toc
---
<h1 align="center">
    <img src="/assets/images/logo.svg">
</h1>

<p align="center">
  <i align="center">Manage your PostgreSQL resources (databases, roles, schemas, etc.) from your Kubernetes cluster</i>
</p>

<h4 align="center">
  <a href="https://github.com/hoppscale/managed-postgres-operator/actions/workflows/test.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/hoppscale/managed-postgres-operator/test.yml?branch=master&label=pipeline&style=flat-square" alt="continuous integration" style="height: 20px;">
  </a>
  <a>
    <img src="https://goreportcard.com/badge/github.com/hoppscale/managed-postgres-operator" alt="goreportcard" style="height: 20px;">
  </a>
  <a href="https://github.com/hoppscale/managed-postgres-operator/graphs/contributors">
    <img src="https://img.shields.io/github/contributors-anon/hoppscale/managed-postgres-operator?color=yellow&style=flat-square" alt="contributors" style="height: 20px;">
  </a>
  <a href="https://opensource.org/licenses/Apache-2.0">
    <img src="https://img.shields.io/badge/apache%202.0-blue.svg?style=flat-square&label=license" alt="license" style="height: 20px;">
  </a>
  <a>
    <img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/hoppscale/managed-postgres-operator" style="height: 20px;">
  </a>
  <br>
</h4>

## Introduction

Managed Postgres Operator aims to automate a PostgreSQL server content from a Kubernetes cluster. It allows you to manage various types of PostgreSQL resources like databases, roles or schemas.

## Supported Resources

The Managed Postgres Operator currently manages the following resources:

- Databases, with [PostgresDatabase](reference/api/v1alpha1/index.md#postgresdatabase)
- Roles, with [PostgresRole](reference/api/v1alpha1/index.md#postgresrole)
- Schemas, with [PostgresSchema](reference/api/v1alpha1/index.md#postgresschema)

## Usage

We recommend deploying the [official Docker image](https://github.com/hoppscale/managed-postgres-operator/pkgs/container/managed-postgres-operator), with our [Helm chart](https://github.com/hoppscale/managed-postgres-operator/tree/master/deploy/charts/managed-postgres-operator) in your Kubernetes cluster.

One operator instance must be connected to one PostgreSQL server. If you need to manage mutiple PostgreSQL servers, you will have to deploy as many operators.

To install the operator, follow the [installation guide](how_to_guides/installation.md).

## Troubleshooting

If you encounter any issues while using the Managed Postgres Operator, we recommend checking the documentation and reviewing the existing [Github issues](https://github.com/hoppscale/managed-postgres-operator/issues) for assistance.

If you think you've identified a bug and can't find a related issue, don't hesitate to [submit a new one](https://github.com/hoppscale/managed-postgres-operator/issues/new)! Make sure to provide as much information as possible about your environment.

## Contributing

We gladly welcome [pull requests](https://github.com/hoppscale/managed-postgres-operator/pulls)! PostgreSQL offers a wide range of features, and the operator currently implements only a small portion of them. Please feel free to suggest improvements or changes to enhance its stability and reliability.

## Changelog

[Every release on GitHub](https://github.com/hoppscale/managed-postgres-operator/releases) has a changelog, and we also publish release notes on the [documentation's website](reference/release-notes/index.md).
