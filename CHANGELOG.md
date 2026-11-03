# Changelog

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
