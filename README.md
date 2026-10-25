# Xbox 360 Gateway Server

OpenPak's Xbox 360 Gateway Server.

| | |
| --- | --- |
| Network | Xbox 360 |
| Role | Xbox 360 security gateway: terminates the secure channel, proxies Live service traffic, publishes presence |
| Depends on | [`x360-common`](https://github.com/openpak/x360-common) |
| Image | `ghcr.io/openpak/x360-gateway` |
| Licence | AGPL-3.0-only |

## What it serves

| Endpoint | What |
| --- | --- |
| Secure channel | UDP, opened by the console after its ticket exchange |
| User-logon ticket, service-26 notifications | Answered on the secure channel |
| Live service proxy | Tunnelled TCP connections, proxied to the HTTP backends of the Live services |
| Presence | Signed-in users, published in the OpenPak account core (namespace `xbox360`) |

## Run

The live quadlet is in `deploy/secondary/`. Details are in [Deploy](docs/deploy.md).

## Build and test

```sh
TMPDIR=<scratch> X360_KDC_TESTKEYS=<dir with the dev keys> go test -mod=vendor ./...
```

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
