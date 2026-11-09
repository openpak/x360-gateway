package gateway

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openpak/x360-common/kdc"
)

// Frames from re-presence-push.md section 6: friend 0x0009000000ABCDEF, slot 0,
// t = 2026-11-02 00:00 UTC.
const (
	pushOnlineHex  = "01000000f401000000000000010000005400000000000000010000003800e90300000000efcdab00000009000000000000000000000000000000000000000000e607534d00a3d52c000000000000000001000000"
	pushOfflineHex = "01000000f401000000000000010000005400000000000000010000003800e90300000000efcdab000000090000000000000000000000000000000000000000000000000000a3d52c000000000000000000000000"
)

func TestPresenceFrames(t *testing.T) {
	when := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	x := uint64(0x0009000000ABCDEF)
	if got := hex.EncodeToString(presenceFrame(1, presenceRec(x, 0x4D5307E6, when, [8]byte{}, 1))); got != pushOnlineHex {
		t.Fatalf("online %s", got)
	}
	if got := hex.EncodeToString(presenceFrame(1, presenceRec(x, 0, when, [8]byte{}, 0))); got != pushOfflineHex {
		t.Fatalf("offline %s", got)
	}
}

func pushStreamConn(g *Gateway, addr string, seed byte, xuid uint64) (*Session, *tcpConn) {
	s := relaySession(nil, addr, seed, seed)
	s.Users = []kdc.TicketUser{{XUID: xuid, Account: "A"}}
	s.lastSeen = g.now()
	s.expires = g.now().Add(time.Hour)
	c := &tcpConn{key: tcpKey{0x7098, streamPort}, sess: s, iss: 100, sndUna: 101, sndNxt: 101, rcvNxt: 5, peerWindow: 0x4000,
		established: true}
	s.conns = map[tcpKey]*tcpConn{c.key: c}
	g.sessions[s.Client] = s
	return s, c
}

func streamMsg(cmd, flags uint32, body []byte) []byte {
	m := make([]byte, 0, 24+len(body))
	for _, v := range []uint32{1, cmd, 9, flags, uint32(24 + len(body)), 0} {
		m = binary.LittleEndian.AppendUint32(m, v)
	}
	return append(m, body...)
}

func watchBody(xuids ...uint64) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(len(xuids)))
	for _, x := range xuids {
		b = append(b, 0x40, 0, 0xe9, 3, 0, 0, 0, 0)
		b = binary.LittleEndian.AppendUint64(b, x)
	}
	return b
}

func logonBody(x uint64) []byte { return binary.LittleEndian.AppendUint64([]byte{1, 0}, x) }

func TestPresenceFanOut(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	g := &Gateway{Now: func() time.Time { return now }, sessions: map[netip.AddrPort]*Session{}}
	log := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	const ann, bob = 0x0009000000000001, 0x0009000000000002
	sa, ca := pushStreamConn(g, "203.0.113.5:3074", 0x11, ann)
	sb, cb := pushStreamConn(g, "198.51.100.7:51000", 0x22, bob)

	// Bob signs in and watches Ann (offline: empty 303).
	cb.req = append(streamMsg(100, 1, logonBody(bob)), streamMsg(102, 1, watchBody(ann))...)
	g.streamServe(cb, log)
	if n := binary.LittleEndian.Uint32(cb.resp[28+28:]); n != 0 || len(g.outbox) != 0 {
		t.Fatalf("offline friend: n=%d outbox=%d", n, len(g.outbox))
	}
	cb.resp = nil
	cb.sndNxt = cb.sndUna // nothing was sent in this unit test

	// Ann signs in: Bob gets the online push.
	ca.req = streamMsg(100, 1, logonBody(ann))
	g.streamServe(ca, log)
	if len(g.outbox) != 1 || g.outbox[0].To != sb.Client {
		t.Fatalf("outbox %+v", g.outbox)
	}
	sg, err := openTCP(sb.s2c, g.outbox[0].Data, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := presenceFrame(1, presenceRec(ann, dashboardTitle, now, [8]byte{}, 1))
	if !bytes.Equal(sg.Payload, want) || sg.Seq != 101 || sg.Ack != 5 {
		t.Fatalf("push %x seq %d", sg.Payload, sg.Seq)
	}

	// A later watcher gets Ann in the 303 snapshot.
	_, cc := pushStreamConn(g, "192.0.2.9:4000", 0x33, 0x0009000000000003)
	cc.req = append(streamMsg(100, 1, logonBody(0x0009000000000003)), streamMsg(102, 1, watchBody(ann, bob))...)
	g.streamServe(cc, log)
	r := cc.resp[28:]
	if binary.LittleEndian.Uint32(r[28:]) != 2 { // Ann and Bob are both online
		t.Fatalf("303 %x", r)
	}

	// Ann's stream goes away: Bob gets the offline push.
	g.outbox = nil
	delete(sa.conns, ca.key)
	g.reapStreams()
	if len(g.outbox) != 2 { // Bob and the third console both watch Ann
		t.Fatalf("offline outbox %d", len(g.outbox))
	}
	for _, d := range g.outbox {
		if d.To != sb.Client {
			continue
		}
		sg, _ := openTCP(sb.s2c, d.Data, 0)
		if !bytes.Equal(sg.Payload, presenceFrame(1, presenceRec(ann, 0, now, [8]byte{}, 0))) {
			t.Fatalf("offline push %x", sg.Payload)
		}
	}
	if g.online[ann] != nil || g.online[bob] == nil {
		t.Fatal("registry")
	}
}

func httpPost(path string, body []byte) []byte {
	return append([]byte("POST "+path+" HTTP/1.1\r\nHost: xmatch\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
}

func httpReply(body []byte) []byte {
	return append([]byte("HTTP/1.1 200 OK\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"), body...)
}

// hostBody is an xmatchhost2 request: u32 len | XNKID | u32 title | XNADDR ... (only the head matters here).
func hostBody(kid [8]byte, title uint32) []byte {
	b := binary.LittleEndian.AppendUint32(nil, 0x90)
	b = append(b, kid[:]...)
	b = binary.LittleEndian.AppendUint32(b, title)
	return append(b, make([]byte, 52)...)
}

func TestHostSessionUpdate(t *testing.T) {
	fresh := [8]byte{0x90}
	server := [8]byte{0x90, 0x39, 0x20, 0x4C, 0x44, 0xAE, 0x8C, 0x6E}
	kind, title, kid := hostSessionUpdate(httpPost("/xmatch/xmatchhost2.ashx", hostBody(fresh, 0x584111F7)),
		httpReply(append(server[:], make([]byte, 16)...)))
	if kind != hostCreate || title != 0x584111F7 || kid != server {
		t.Fatalf("create %d %08x %x", kind, title, kid)
	}
	kind, _, kid = hostSessionUpdate(httpPost("/xmatch/xmatchhost2.ashx", hostBody(server, 0x584111F7)), httpReply(nil))
	if kind != hostModify || kid != server {
		t.Fatalf("modify %d %x", kind, kid)
	}
	del := append(append([]byte{0, 0, 0, 0}, server[:]...), 0, 0, 0, 0)
	kind, _, kid = hostSessionUpdate(httpPost("/xmatch/xmatchhostdelete2.ashx", del), httpReply(nil))
	if kind != hostDelete || kid != server {
		t.Fatalf("delete %d %x", kind, kid)
	}
	if k, _, _ := hostSessionUpdate(httpPost("/xmatch/xmatchclient2.ashx", hostBody(server, 1)), httpReply(nil)); k != hostNone {
		t.Fatal("search counted")
	}
	failed := []byte("HTTP/1.1 500 Internal Server Error\r\nContent-Length: 0\r\n\r\n")
	if k, _, _ := hostSessionUpdate(httpPost("/xmatch/xmatchhost2.ashx", hostBody(fresh, 1)), failed); k != hostNone {
		t.Fatal("failed create counted")
	}
}

// A friend who hosts a Live session is pushed joinable in its title with the session XNKID.
func TestPresenceHostJoinable(t *testing.T) {
	now := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	g := &Gateway{Now: func() time.Time { return now }, sessions: map[netip.AddrPort]*Session{}}
	log := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	const ann, bob = 0x0009000000000001, 0x0009000000000002
	sa, ca := pushStreamConn(g, "203.0.113.5:3074", 0x11, ann)
	sb, cb := pushStreamConn(g, "198.51.100.7:51000", 0x22, bob)
	ca.req = streamMsg(100, 1, logonBody(ann))
	g.streamServe(ca, log)
	cb.req = append(streamMsg(100, 1, logonBody(bob)), streamMsg(102, 1, watchBody(ann))...)
	g.streamServe(cb, log)
	cb.resp = nil
	cb.sndNxt = cb.sndUna
	g.outbox = nil

	server := [8]byte{0x90, 1, 2, 3, 4, 5, 6, 7}
	g.noteHostSession(sa, httpPost("/xmatch/xmatchhost2.ashx", hostBody([8]byte{0x90}, 0x584111F7)),
		httpReply(append(server[:], make([]byte, 16)...)))
	if len(g.outbox) != 1 || g.outbox[0].To != sb.Client {
		t.Fatalf("outbox %+v", g.outbox)
	}
	sg, err := openTCP(sb.s2c, g.outbox[0].Data, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := presenceFrame(1, presenceRec(ann, 0x584111F7, now, server, statePresent|statePlaying|stateJoinable))
	if !bytes.Equal(sg.Payload, want) {
		t.Fatalf("push %x", sg.Payload)
	}
	// The same session modified again pushes nothing; deleting it drops joinable.
	g.outbox = nil
	g.noteHostSession(sa, httpPost("/xmatch/xmatchhost2.ashx", hostBody(server, 0x584111F7)), httpReply(nil))
	if len(g.outbox) != 0 {
		t.Fatal("unchanged modify pushed")
	}
	del := append(append([]byte{0, 0, 0, 0}, server[:]...), 0, 0, 0, 0)
	g.noteHostSession(sa, httpPost("/xmatch/xmatchhostdelete2.ashx", del), httpReply(nil))
	if o := g.online[ann]; o == nil || o.kid != [8]byte{} || o.state != statePresent|statePlaying || len(g.outbox) != 1 {
		t.Fatalf("after delete %+v outbox %d", o, len(g.outbox))
	}
}
