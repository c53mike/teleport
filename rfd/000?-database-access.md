---
authors: Roman Tkachenko (roman@gravitational.com)
state: draft
---

# RFD 000? - Teleport Database Access (Preview)

## What

This document discusses high-level design points, user experience and some
implementation details of the Teleport Database Access feature.

_Note: This document refers to an early preview of the Database Access feature 
and covers functionality that will be available in the initial release._

With Teleport Database Access users can:

- Provide secure access to databases without exposing them over the public
  network through Teleport's reverse tunnel subsystem.
- Control access to specific database instances as well as individual
  databases and database users through Teleport's RBAC model.
- Track individual users access to databases as well as query activity
  through Teleport's audit log.

## Use cases

The feature is being developed with the following use-cases in mind.

### Human access

Users should be able to access the databases connected to Teleport using
regular database clients they normally use to connect directly such as
CLI clients (`psql`, `mysql`, etc.) as well as graphical interfaces (`pgAdmin`,
`MySQL Workbench`, etc.).

The use-case for this is to grant users access to a database in a transparent
fashion, for example to let them do development in a test/stage environment
or perform an emergency recovery on a production database instance using
familiar tools.

### Robot access

The feature should be automation friendly so existing CI systems can take
advantage of it.

An example would be letting the tools like Ansible or Drone perform routine
actions on a database such as migrations or backups and be able to audit it.

### Programmatic access

Programmatic access, as in configuring an application to talk to a database
through Teleport proxy, should work automatically as long as it uses a driver
that properly implements a particular database protocol and supports mutual
TLS authentication.

However, it is not the primary use-case, at least for the initial release,
since it comes with a number of additional concerns and considerations such
as performance requirements for high-traffic applications, automatic failover
and so on.

## Scope

For the initial release we're focusing on supporting a single type of the
database, PostgreSQL, with full protocol parsing.

The following PostgreSQL deployment models are supported:

* PostgreSQL instances deployed on-premises.
* AWS RDS for PostgreSQL.
* PostgreSQL-compatible AWS Aurora.

The following features are provided:

* Connecting to the database through the Teleport proxy, incl. trusted
  clusters support.
* Limiting access to database instances by labels with Teleport roles.
* Limiting access to individual databases (within a particular database
  instance) and database users.
* Auditing of database connections and executed queries.

## Authentication



## Configuration

The following new configuration section is added to the Teleport file config:

```yaml
# New global key housing the database service configuration.
db_service:
  # Enable or disable the database service.
  enabled: "yes"
  # List of the database this service is proxying.
  databases:
    # Database instance name, used to refer to an instance in CLI like tsh.
  - name: "postgres-prod"
    # Optional free-form verbose description of a database instance.
    description: "Production instance of PostgreSQL 13.0"
    # Database procotol, only "postgres" is supported initially.
    protocol: "postgres"
    # Database connection endpoint, should be reachable from the service.
    endpoint: "postgres-rds.xxx.us-east-1.rds.amazonaws.com:5432"
    # Optional CA certificate path, e.g. for AWS RDS/Aurora.
    ca_path: "/opt/rds/rds-ca-2019-root.pem"
    # Optional AWS region RDS/Aurora database is running in.
    region: "us-east-1"
    # Use AWS IAM authentication with RDS/Aurora database.
    auth: "aws-iam"
    # Static labels assigned to the database instance, used in RBAC.
    labels:
      env: "stage"
    # Dynamic labels assigned to the database instance, used in RBAC.
    commands:
    - name: "time"
      command: ["date", "+%H:%M:%S"]
      period: "1m"
```

## Routing


## RBAC


## Audit log
