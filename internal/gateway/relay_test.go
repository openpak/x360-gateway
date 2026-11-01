package gateway

import (
	"bytes"
	"net/netip"
	"testing"
	"time"
)

func relaySession(t *testing.T, addr string, spi byte, seed byte) *Session {
	c2s, s2c := derive(bytes.Repeat([]byte{seed}, 100))
	s := &Session{Client: netip.MustParseAddrPort(addr), ClientSPI: []byte{0, 1, 2, seed}, ServerSPI: []byte{0, 9, 9, spi},
		c2s: c2s, s2c: s2c, expires: time.Now().Add(time.Hour)}
	ip := s.Client.Addr().As4()
	copy(s.SGAddr[:4], ip[:])
	copy(s.SGAddr[4:], s.ServerSPI)
	return s
}

func TestRelay(t *testing.T) {
	a := relaySession(t, "203.0.113.5:3074", 1, 0x11)
	b := relaySession(t, "198.51.100.7:51000", 2, 0x22)
	g := &Gateway{sessions: map[netip.AddrPort]*Session{a.Client: a, b.Client: b}}

	body := make([]byte, 40, 120)
	copy(body[0:20], bytes.Repeat([]byte{0xee}, 20)) // forged sender
	copy(body[20:40], b.SGAddr[:])
	body = append(body, []byte("\x58\x0d\x76\x00key exchange")...)
	out := g.HandleAll(seal(a.c2s, a.ServerSPI, 1, TypeRelay, body), a.Client)
	if len(out) != 1 || out[0].To != b.Client {
		t.Fatalf("relay: %+v", out)
	}
	_, m, err := open(b.s2c, out[0].Data)
	if err != nil || m.Type != TypeRelay || m.Counter != 1 {
		t.Fatalf("relayed packet: %+v %v", m, err)
	}
	if !bytes.Equal(m.Body[:20], a.SGAddr[:]) || !bytes.Equal(m.Body[20:], body[20:]) {
		t.Fatalf("relayed body %x", m.Body)
	}
	// Handle (one socket, src only) drops it; an unknown target is dropped.
	copy(body[20:40], bytes.Repeat([]byte{1}, 20))
	if out := g.HandleAll(seal(a.c2s, a.ServerSPI, 2, TypeRelay, body), a.Client); len(out) != 0 {
		t.Fatalf("unknown target relayed: %+v", out)
	}
}
