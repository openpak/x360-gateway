# Protocol: x360-gateway

The Xbox 360 security gateway (SG): the secure channel the console opens after its ticket exchange (UDP 3074 behind
an nft redirect to :13075). It terminates the channel, proxies the tunnelled TCP connections of the Live services to their HTTP
backends, answers the user-logon ticket message and the service-26 notification stream, and publishes signed-in users as
presence (namespace `xbox360`) in the OpenPak account core.

## Public address

XAM always dials UDP 3074 for the gateway: it ignores a port in an extended D7 entry for service 20. The 360
secure-channel initiation cannot be told apart from an original-Xbox user-session initiation at fixed payload
offsets (the first 12 payload bytes match), so a payload-matching redirect on a shared 3074 steals sessions from
xb-gateway. The gateway therefore needs its own public IPv4 address (`X360_KDC_SG_ADDRESS` in the KDC points at it),
redirected by destination address only.

## Peer connections (XAM 17559, static RE)

The 0x5803 reply core gives the console its online identity (XAM `FUN_81752af8` copies it to the primary SG
connection):

| core bytes | XAM field | XNADDR field | gateway fills |
| --- | --- | --- | --- |
| 28-48 | conn +0xcc | `abOnline` (SGADDR: ina, spi, xbox id, reserved) | console IP, server SPI, zeros |
| 48-52 | conn +0xe0 | `inaOnline` | console public IP as seen by the gateway |
| 52-54 | conn +0xe4 | `wPortOnline` (big-endian) | console public UDP port as seen by the gateway |

Bytes 32-54 must not be zero: otherwise every console advertises `inaOnline` 0.0.0.0:0 and the SGADDRs differ only
by IP.

Peers send each other XNet traffic directly to `inaOnline:wPortOnline` (`FUN_8174a958`; same `inaOnline` = same LAN,
use `ina`). For an online peer XAM also sends the key exchange through the gateway as message **0x1253**
(`FUN_81750878`; XNet QoS probes the same way, `FUN_81758238`):

```
0x1253 body: SGADDR from (20) | SGADDR to (20) | XNet packet (key exchange 0x58 family, QoS 0xd58)
```

The receiver (`FUN_81754c38`) takes a 0x1253 only on its primary connection, longer than 0x2c bytes, and only if
"SGADDR to" is its own SGADDR; it passes "from" on so it can answer the same way. The gateway relays a 0x1253 to the
session whose SGADDR equals "to", with "from" overwritten by the sender's real SGADDR; unknown targets are dropped.

## UDP (XAM 17559, static RE)

Data codes 4..6 are UDP: code = 4 + port mode (0: both ports 1000, 1: one byte each + 1000, 2: `u16` each,
big-endian), the TCP datagram layout with a run of the ports alone (receive path `FUN_817551e0`):

    SPI(4) | 3DES-CBC(payload | zero fill | run[:k]) | run[k:] | ctr low16 LE | ICV10,   k = min(pad, len(run))

The ICV input is `run[k:] | ctr lo | ctr hi | SPI`, zero-filled to a multiple of 4 (the receiver writes the zeros
itself), then the ciphertext. The only UDP XAM sends the gateway is arbitration (Live service 16, port 1000,
`FUN_81917ab0`): a whole HTTP/1.0 request in one datagram. The gateway proxies it like a tunnelled TCP request and
answers with the HTTP response in one code-6 datagram from port 1000; XAM looks only for `200 OK` and `X-Err:` in
it. UDP to any other port is dropped.
