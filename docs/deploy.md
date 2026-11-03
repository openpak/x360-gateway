# Deploy: x360-gateway

## Configuration

Environment `X360_GATEWAY_*` (see `deploy/secondary/openpak-x360-gateway.container` and `x360-common/config`): `ADDR`, `TICKET_KEY`, `ACCOUNTS` (legacy file), `DATABASE_URL` (required), `CONSOLE_CA`, `SERVICE_URL(S)`, `DEBUG`, plus the account core URL/token for presence.

## Accounts in Postgres

Since gateway v0.11.0 and KDC v0.10.0 the machine accounts live in Postgres, not in `accounts.json`: database `x360`,
schema `x360_kdc` (document `accounts`, created for the role ahead of time), shared by `x360-kdc` (the only writer) and
`x360-gateway` (reads, reloaded on every initiation; if the database is down it keeps the last loaded accounts).
`X360_KDC_DATABASE_URL` / `X360_GATEWAY_DATABASE_URL` are required: the service refuses to start without them. The
secondary reaches Postgres over ZeroTier: role `x360kdc` (connection limit 3, database `x360` only) at
`10.212.63.196:19433` on the primary, a forwarder (`openpak-x360-pg-forward.service`) that accepts only the secondary
(`10.212.63.175`), backed by an INPUT rule and `pg_hba.conf` (`host x360 x360kdc 10.89.2.217/32 scram-sha-256`, then
`host all x360kdc all reject`). The URL lives in `~/openpak/deploy/x360-kdc/db.env` (0600, never committed).
If the database holds no accounts yet, an existing `accounts.json` is imported once and left in place as a backup.

## Build and test

`TMPDIR=<scratch> X360_KDC_TESTKEYS=<dir with the dev keys> go test -mod=vendor ./...`

Tag `vX.Y.Z` and CI publishes `ghcr.io/openpak/x360-gateway:latest` (amd64 + arm64); hosts auto-update `:latest`.
`deploy/` holds the live quadlet and helpers. History before 2026-10-18 is the x360-kdc monorepo's, filtered to
this service. Licensed AGPL-3.0, see LICENSE.

## Shared library

The shared Go library [`x360-common`](https://github.com/openpak/x360-common) is consumed with `replace` and a committed `vendor/`. After changing x360-common, run `go mod vendor` here and commit.
