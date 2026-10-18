# x360-gateway

The Xbox 360 security gateway (SG): the secure channel the console opens after its ticket exchange (UDP 3074 behind
an nft redirect to :13075). It terminates the channel, proxies the tunnelled TCP connections of the Live services to their HTTP
backends, answers the user-logon ticket message and the service-26 notification stream, and publishes signed-in users as
presence (namespace `xbox360`) in the OpenPak account core.

Part of OpenPak's Xbox 360 stack: one repository and one image per service ([`x360-kdc`](../x360-kdc),
[`x360-gateway`](../x360-gateway), [`x360-xuacs`](../x360-xuacs), [`x360-liveinfo`](../x360-liveinfo),
[`x360-notice`](../x360-notice), [`x360-xexds`](../x360-xexds)) around the shared Go library
[`x360-common`](../x360-common) (`replace` + committed `vendor/`; after changing x360-common run
`go mod vendor` here and commit).


## Configuration

Environment `X360_GATEWAY_*` (see `deploy/secondary/openpak-x360-gateway.container` and `x360-common/config`): `ADDR`, `TICKET_KEY`, `ACCOUNTS`, `CONSOLE_CA`, `SERVICE_URL(S)`, `DEBUG`, plus the account core URL/token for presence.

## Build and test

`TMPDIR=<scratch> X360_KDC_TESTKEYS=<dir with the dev keys> go test -mod=vendor ./...`

Tag `vX.Y.Z` and CI publishes `ghcr.io/openpak/x360-gateway:latest` (amd64 + arm64); hosts auto-update `:latest`.
`deploy/` holds the live quadlet and helpers. History before 2026-10-18 is the x360-kdc monorepo's, filtered to
this service. Licensed AGPL-3.0, see LICENSE.
