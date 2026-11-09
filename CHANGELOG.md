# Changelog

- 2026-11-09: v0.12.1: Peers can key-exchange. The SGADDR in the reply core (XNADDR `abOnline`) now carries reserved byte
  `01` (XNADDR byte 0x20, XAM's own default). With 0 there the host's XAM dropped every online key-exchange initiation
  (0xa58, relayed as 0x1253), so a joiner never got a security association (Minecraft join, card #163). Consoles pick the
  new SGADDR up at their next sign-in.

- 2026-11-09: v0.12.0: Friends see a hosted game. When a console's `xmatchhost2.ashx` create/modify (Live service 6) is relayed
  with a 200, its online users are pushed (command 500, and in later 303 snapshots) in the session's title with the session
  XNKID and state online|playing|joinable (0x13); `xmatchhostdelete2.ashx` drops joinable again. Before, every friend was in
  the dashboard title with XNKID 0, so a friend's game never showed in a title's join list (Minecraft, card #163).

- 2026-11-04: v0.11.1: x360-common v0.8.0 (re-vendored). Account erasure of card #120 needs no endpoint here: the gateway only reads the KDC's `x360_kdc` accounts and reloads them on every initiation, so an account erased through x360-kdc (v0.11.0) is gone from the gateway on the next initiation.

- 2026-11-03: v0.11.0: Accounts are read from Postgres (the KDC's schema `x360_kdc`, `X360_GATEWAY_DATABASE_URL`, required), reloaded on every initiation; if a reload fails the last loaded accounts are used and a warning is logged. x360-common v0.6.0.

- 2026-11-03: v0.10.2: No secrets in logs: `http request` no longer logs the request line, headers or body hex (counts only), `stream message` and `unverifiable data` no longer log hex dumps. The access line `http <method> <path> -> <status> <ms>` is the per-request record. Test added.

- 2026-11-03: v0.10.1: Per-command logging: each tunnelled service request writes `http <method> <path> -> <status> <duration>ms` (path only, never the query or headers) and each SG initiation writes `cmd initiation -> ok|rejected <duration>ms` (x360-common `httplog`). The existing debug-style `http request` / `stream message` lines are unchanged.

- 2026-10-18: v0.9.0 split out of x360-kdc v0.8.14 into its own repository and image (`ghcr.io/openpak/x360-gateway`),
  history kept; shared packages now come from `x360-common`. Earlier entries: the x360-kdc CHANGELOG.

- 2026-10-18: v0.9.1 the proxied response drops the Date header: the console's XSTS client (XHttp stack) parses Date
  strictly and converts it with SystemTimeToFileTime; the old xon clients never needed it.

- 2026-11-01: v0.9.5 peer connections: the reply core carries the console's public IP:port (XNADDR inaOnline,
  wPortOnline) and a per-session SGADDR (server SPI), and 0x1253 key-exchange/QoS packets are relayed to the
  session whose SGADDR they name.

- 2026-11-01: v0.9.6 UDP inside the secure channel (codes 4..6, ports-only run): an arbitration datagram to port
  1000 (HTTP request text) is proxied to the service backend and the HTTP response goes back as one datagram. TCP
  seal/open now share the run layout code.

- 2026-11-02: v0.10.0 friend presence on the service-26 stream: users online by command 100, command 500 pushes
  (online/offline) to the streams that watch them (commands 102/103), 303 replies carry the online friends. In memory;
  title is always the dashboard; untested on a console.
