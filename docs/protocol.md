# Protocol: x360-gateway

The Xbox 360 security gateway (SG): the secure channel the console opens after its ticket exchange (UDP 3074 behind
an nft redirect to :13075). It terminates the channel, proxies the tunnelled TCP connections of the Live services to their HTTP
backends, answers the user-logon ticket message and the service-26 notification stream, and publishes signed-in users as
presence (namespace `xbox360`) in the OpenPak account core.

## Peer connections (XAM 17559, static RE 2026-11-01)

The 0x5803 reply core gives the console its online identity (XAM `FUN_81752af8` copies it to the primary SG
connection):

| core bytes | XAM field | XNADDR field | gateway fills |
| --- | --- | --- | --- |
| 28-48 | conn +0xcc | `abOnline` (SGADDR: ina, spi, xbox id, reserved) | console IP, server SPI, zeros |
| 48-52 | conn +0xe0 | `inaOnline` | console public IP as seen by the gateway |
| 52-54 | conn +0xe4 | `wPortOnline` (big-endian) | console public UDP port as seen by the gateway |

Before 2026-11-01 bytes 32-54 were zero, so every console advertised `inaOnline` 0.0.0.0:0 and the SGADDRs only
differed by IP.

Peers send each other XNet traffic directly to `inaOnline:wPortOnline` (`FUN_8174a958`; same `inaOnline` = same LAN,
use `ina`). For an online peer XAM also sends the key exchange through the gateway as message **0x1253**
(`FUN_81750878`; XNet QoS probes the same way, `FUN_81758238`):

```
0x1253 body: SGADDR from (20) | SGADDR to (20) | XNet packet (key exchange 0x58 family, QoS 0xd58)
```

The receiver (`FUN_81754c38`) takes a 0x1253 only on its primary connection, longer than 0x2c bytes, and only if
"SGADDR to" is its own SGADDR; it passes "from" on so it can answer the same way. The gateway relays a 0x1253 to the
session whose SGADDR equals "to", with "from" overwritten by the sender's real SGADDR; unknown targets are dropped.
