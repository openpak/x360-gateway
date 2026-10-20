package gateway

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
)

// The first message the console sent on service 26 (captured 2026-10-18).
const streamHello = "0100000064000000851d00000100000022000000000000000100dbda39515f000900"

func TestStreamCommand100(t *testing.T) {
	g := &Gateway{}
	log := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	msg, _ := hex.DecodeString(streamHello)
	c := &tcpConn{req: msg[:20]}
	g.streamServe(c, log)
	if len(c.resp) != 0 || len(c.req) != 20 {
		t.Fatal("partial message must wait")
	}
	c.req = msg
	g.streamServe(c, log)
	r := c.resp
	if len(r) != 28 || binary.LittleEndian.Uint32(r[4:]) != 301 || !bytes.Equal(r[8:12], msg[8:12]) ||
		binary.LittleEndian.Uint32(r[16:]) != 28 || binary.LittleEndian.Uint32(r[24:]) != 0 || len(c.req) != 0 {
		t.Fatalf("reply %x, left %d", r, len(c.req))
	}
}

// A friend-presence watch the console sent after command 100 (captured
// 2026-10-20): six records of {0x40, type 1001, 0, friend XUID}. Unanswered,
// the console resent it after 120 s and reset the stream.
const streamWatch = "010000006600000002660000010000007c00000000000000060000004000e90300000000a63ef69f200009004000e903000000003fc9e3ed490009004000e903000000000df8835c560009004000e9030000000019e86b8b8c0009004000e9030000000039b17abfae0009004000e903000000005d1ad735d3000900"

func TestStreamAnswersEveryClientCommand(t *testing.T) {
	g := &Gateway{}
	log := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	watch, _ := hex.DecodeString(streamWatch)
	c := &tcpConn{req: watch}
	g.streamServe(c, log)
	r := c.resp
	if len(r) != 32 || binary.LittleEndian.Uint32(r[4:]) != 303 || !bytes.Equal(r[8:12], watch[8:12]) ||
		binary.LittleEndian.Uint32(r[16:]) != 32 || binary.LittleEndian.Uint32(r[24:]) != 0 ||
		binary.LittleEndian.Uint32(r[28:]) != 0 || len(c.req) != 0 || !c.responded {
		t.Fatalf("watch reply %x", r)
	}
	for cmd, size := range map[uint32]int{101: 28, 103: 28, 104: 32, 105: 28} {
		msg := make([]byte, streamHeader+8)
		binary.LittleEndian.PutUint32(msg, 1)
		binary.LittleEndian.PutUint32(msg[4:], cmd)
		binary.LittleEndian.PutUint32(msg[8:], 7)
		binary.LittleEndian.PutUint32(msg[16:], uint32(len(msg)))
		c := &tcpConn{req: msg}
		g.streamServe(c, log)
		if len(c.resp) != size || binary.LittleEndian.Uint32(c.resp[4:]) != cmd+201 ||
			binary.LittleEndian.Uint32(c.resp[16:]) != uint32(size) {
			t.Fatalf("cmd %d reply %x", cmd, c.resp)
		}
	}
	// Server-side command numbers (300+, 500 push) are never echoed back.
	msg := make([]byte, streamHeader)
	binary.LittleEndian.PutUint32(msg[4:], 500)
	binary.LittleEndian.PutUint32(msg[16:], streamHeader)
	c = &tcpConn{req: msg}
	g.streamServe(c, log)
	if len(c.resp) != 0 {
		t.Fatalf("unexpected reply %x", c.resp)
	}
}
