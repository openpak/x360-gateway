# Deploy: x360-gateway

## Configuration

Environment `X360_GATEWAY_*` (see `deploy/secondary/openpak-x360-gateway.container` and `x360-common/config`): `ADDR`, `TICKET_KEY`, `ACCOUNTS`, `CONSOLE_CA`, `SERVICE_URL(S)`, `DEBUG`, plus the account core URL/token for presence.

## Build and test

`TMPDIR=<scratch> X360_KDC_TESTKEYS=<dir with the dev keys> go test -mod=vendor ./...`

Tag `vX.Y.Z` and CI publishes `ghcr.io/openpak/x360-gateway:latest` (amd64 + arm64); hosts auto-update `:latest`.
`deploy/` holds the live quadlet and helpers. History before 2026-10-18 is the x360-kdc monorepo's, filtered to
this service. Licensed AGPL-3.0, see LICENSE.
