# Changelog

- 2026-10-18: v0.9.0 split out of x360-kdc v0.8.14 into its own repository and image (`ghcr.io/openpak/x360-gateway`),
  history kept; shared packages now come from `x360-common`. Earlier entries: the x360-kdc CHANGELOG.

- 2026-10-18: v0.9.1 the proxied response drops the Date header: the console's XSTS client (XHttp stack) parses Date
  strictly and converts it with SystemTimeToFileTime; the old xon clients never needed it.
