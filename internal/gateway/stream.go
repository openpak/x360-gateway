package gateway

import (
	"encoding/binary"
	"encoding/hex"
)

// streamPort is Live service 26, a long-lived TCP stream the console's XLive
// worker opens after the user logon (FUN_818f07a8, state machine FUN_818d02b8).
const streamPort = 26

// Messages are little-endian: u16 version | u16 0 | u32 command | u32 request
// id | u32 flags | u32 total length | u32 retry count | body. The console's
// first message is command 100 (body: u16 1 | online XUID); the state machine
// accepts command+201 with the same id and a u32 status after the header
// (0 = success). Every client command 100..105 waits for its reply (25 s for
// 100, 60-120 s for the others) and resets the whole stream on expiry, so all
// of them are answered: 301/302/304/306 are exactly the header + status;
// 303/305 add a u32 record count (records are 20 bytes; none are sent).
// Command 102 is the friend-presence watch the console sends right after 100:
// u32 n, then n x {u16 0x40 | u16 type 1001 | u32 0 | u64 friend XUID}.
const streamHeader = 24

// streamServe consumes whole messages from c.req and appends the replies to
// c.resp (no FIN: the stream stays open for the console's later messages).
func (g *Gateway) streamServe(c *tcpConn, log interface{ Info(string, ...any) }) {
	for len(c.req) >= streamHeader {
		total := int(binary.LittleEndian.Uint32(c.req[16:]))
		if total < streamHeader || total > maxRequest {
			c.req = nil
			return
		}
		if len(c.req) < total {
			return
		}
		msg := c.req[:total]
		c.req = c.req[total:]
		cmd := binary.LittleEndian.Uint32(msg[4:])
		log.Info("stream message", "cmd", cmd, "id", binary.LittleEndian.Uint32(msg[8:]), "len", total, "hex", hex.EncodeToString(msg))
		if cmd < 100 || cmd > 105 {
			continue
		}
		size := streamHeader + 4
		if cmd == 102 || cmd == 104 {
			size += 4 // record count 0
		}
		reply := make([]byte, size)
		copy(reply, msg[:streamHeader])
		binary.LittleEndian.PutUint32(reply[4:], cmd+201)
		binary.LittleEndian.PutUint32(reply[16:], uint32(len(reply)))
		c.resp = append(c.resp, reply...)
		c.responded = true
	}
}
