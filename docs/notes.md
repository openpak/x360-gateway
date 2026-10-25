# Notes: x360-gateway

Part of OpenPak's Xbox 360 stack: one repository and one image per service ([`x360-kdc`](../../x360-kdc),
[`x360-gateway`](../../x360-gateway), [`x360-xuacs`](../../x360-xuacs), [`x360-liveinfo`](../../x360-liveinfo),
[`x360-notice`](../../x360-notice), [`x360-xexds`](../../x360-xexds)) around the shared Go library
[`x360-common`](../../x360-common) (`replace` + committed `vendor/`; after changing x360-common run
`go mod vendor` here and commit).
