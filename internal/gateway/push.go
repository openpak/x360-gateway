package gateway

// Friend presence on the service-26 stream (wire spec: scratch/wharf/kdcstub/notes/re-presence-push.md).
// In memory, one gateway process: a registry xuid -> stream of the users whose
// command 100 is on an open stream; each stream remembers the friends the
// console asked to watch (commands 102/103). A user coming up or going away is
// pushed as a command 500 record to every stream that watches it, and the 303
// reply to a watch carries the watched friends that are online right now.

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"
)

const (
	dashboardTitle = 0xFFFE07D1 // the dashboard; the gateway does not know the running title
	presenceRecord = 56         // 20-byte sub-header + 36-byte type 1001 payload
	xbox2003       = 1041379200 // unix time of 2003-01-01 00:00 UTC, the console's presence epoch
	statePresent   = 0x1
	statePlaying   = 0x2  // [I SDK name]
	stateJoinable  = 0x10 // FUN_8172c228 turns it into the friend's joinable flag
	cmdPush        = 500
	typePresence   = 1001
)

// streamState is the presence state of one service-26 connection.
type streamState struct {
	serving  bool              // inside streamServe: pushes only queue, the caller sends
	pushMask uint32            // user slots registered by command 100 (the console's push mask)
	watch    map[uint64]uint32 // watched XUID -> user slots that watch it
	users    []uint64          // XUIDs this stream registered
}

type onlineUser struct {
	s     *Session
	c     *tcpConn
	title uint32
	since time.Time
	kid   [8]byte // the Live session this user hosts (XNKID), zero when none
	state uint32
}

// presenceFrame is a command 500 push; every record is a 56-byte type 1001 record.
func presenceFrame(flags uint32, recs []byte) []byte {
	f := make([]byte, 0, 28+len(recs))
	for _, v := range []uint32{1, cmdPush, 0, flags, uint32(28 + len(recs)), 0, uint32(len(recs) / presenceRecord)} {
		f = binary.LittleEndian.AppendUint32(f, v)
	}
	return append(f, recs...)
}

// presenceRec is one type 1001 record: xuid, title id, time (seconds since 2003), session XNKID, state flags.
func presenceRec(xuid uint64, title uint32, t time.Time, kid [8]byte, state uint32) []byte {
	r := binary.LittleEndian.AppendUint16(nil, presenceRecord)
	r = binary.LittleEndian.AppendUint16(r, typePresence)
	r = binary.LittleEndian.AppendUint32(r, 0) // listener key
	r = binary.LittleEndian.AppendUint64(r, xuid)
	r = binary.LittleEndian.AppendUint32(r, 0) // c, never read
	r = append(r, make([]byte, 16)...)         // gamertag slot, unused
	r = binary.LittleEndian.AppendUint32(r, title)
	r = binary.LittleEndian.AppendUint32(r, uint32(t.Unix()-xbox2003))
	r = append(r, kid[:]...) // session XNKID
	return binary.LittleEndian.AppendUint32(r, state)
}

// streamLogon handles command 100 (body u16 1 | online XUID): the user is online while the stream is.
func (g *Gateway) streamLogon(c *tcpConn, msg []byte) {
	s := c.sess
	if s == nil || len(msg) < streamHeader+10 {
		return
	}
	xuid := binary.LittleEndian.Uint64(msg[streamHeader+2:])
	slot := -1
	for i, u := range s.Users {
		if u.XUID == xuid {
			slot = i
		}
	}
	if slot < 0 {
		return // not a user of this session's ticket
	}
	flags := binary.LittleEndian.Uint32(msg[12:])
	if flags == 0 {
		flags = 1 << slot
	}
	c.stream.pushMask |= flags
	if g.online == nil {
		g.online = map[uint64]*onlineUser{}
	}
	prev := g.online[xuid]
	now := g.now()
	u := &onlineUser{s: s, c: c, title: dashboardTitle, since: now, state: statePresent}
	if prev != nil && prev.s == s { // a reconnect keeps what the console hosts
		u.title, u.kid, u.state = prev.title, prev.kid, prev.state
	}
	g.online[xuid] = u
	c.stream.users = append(c.stream.users, xuid)
	if prev == nil {
		g.pushPresence(xuid, u.title, now, u.kid, u.state)
	}
}

// streamWatch records a command 102 watch list and returns the 303 records of the friends online now.
func (g *Gateway) streamWatch(c *tcpConn, msg []byte) []byte {
	var out []byte
	if c.stream.watch == nil {
		c.stream.watch = map[uint64]uint32{}
	}
	flags := binary.LittleEndian.Uint32(msg[12:])
	for _, x := range watchXUIDs(msg) {
		c.stream.watch[x] |= flags
		if o := g.online[x]; o != nil {
			out = append(out, presenceRec(x, o.title, o.since, o.kid, o.state)...)
		}
	}
	return out
}

func (g *Gateway) streamUnwatch(c *tcpConn, msg []byte) {
	flags := binary.LittleEndian.Uint32(msg[12:])
	for _, x := range watchXUIDs(msg) {
		if c.stream.watch[x] &^= flags; c.stream.watch[x] == 0 {
			delete(c.stream.watch, x)
		}
	}
}

// watchXUIDs reads the records of a 102/103 body: u32 n, then n x 16 bytes (len, type, 0, XUID).
func watchXUIDs(msg []byte) []uint64 {
	if len(msg) < streamHeader+4 {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(msg[streamHeader:]))
	var out []uint64
	for i := 0; i < n && streamHeader+4+16*(i+1) <= len(msg); i++ {
		out = append(out, binary.LittleEndian.Uint64(msg[streamHeader+4+16*i+8:]))
	}
	return out
}

// pushPresence sends xuid's state to every open stream that watches it.
func (g *Gateway) pushPresence(xuid uint64, title uint32, t time.Time, kid [8]byte, state uint32) {
	rec := presenceRec(xuid, title, t, kid, state)
	for _, s := range g.sessions {
		for _, c := range s.conns {
			if c.key.server != streamPort || !c.established || !c.responded {
				continue
			}
			if mask := c.stream.watch[xuid] & c.stream.pushMask; mask != 0 {
				g.pushStream(c, presenceFrame(mask, rec))
			}
		}
	}
}

// pushStream queues frame on stream c and sends what the window allows.
func (g *Gateway) pushStream(c *tcpConn, frame []byte) {
	c.resp = append(c.resp, frame...)
	if c.stream.serving { // the segment loop of the message being served sends it
		return
	}
	n := uint32(len(c.resp))
	win := uint32(max(int(c.peerWindow), respSegment))
	from := c.sndNxt
	for off := from - c.iss - 1; off < n && from-c.sndUna < win; {
		l := min(uint32(respSegment), n-off, c.sndUna+win-from)
		fl := byte(flagACK)
		if off+l == n {
			fl |= flagPSH
		}
		g.outbox = append(g.outbox, Datagram{c.sess.Client, c.sess.segment(c.key, fl, from, c.rcvNxt, nil, c.resp[off:off+l])})
		from += l
		off += l
		c.sndNxt = from
	}
}

// reapStreams takes users offline whose stream is gone (reset, closed, session
// ended or expired) or whose console has been silent for PresenceIdle.
func (g *Gateway) reapStreams() {
	now := g.now()
	for x, o := range g.online {
		if g.sessions[o.s.Client] == o.s && o.s.conns[o.c.key] == o.c && !now.After(o.s.expires) && now.Sub(o.s.lastSeen) <= PresenceIdle {
			continue
		}
		delete(g.online, x)
		g.pushPresence(x, 0, now, [8]byte{}, 0)
	}
}

// Sweep reaps streams of silent consoles between datagrams; the datagrams to send are returned.
func (g *Gateway) Sweep() []Datagram {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outbox = nil
	for k, s := range g.sessions {
		if g.now().After(s.expires) {
			delete(g.sessions, k)
		}
	}
	g.reapStreams()
	return g.outbox
}

// Host sessions. A console hosting a Live session tells matchmaking (Live
// service 6, xmatchhost2.ashx), not presence; on Live the presence service
// links the two, so friends see the host joinable in its title and their
// console can look the session up by its XNKID. The gateway relays those
// requests and applies them to the hosting console's online users.

const (
	hostNone = iota
	hostCreate
	hostModify
	hostDelete
)

// hostSessionUpdate reads a relayed request and its response: XRL 0x100 create
// (request u32 len | XNKID | u32 title | ..., reply XNKID | XNKEY), 0x101 modify
// (same request, the existing XNKID, no reply) and 0x102 delete (u32 | XNKID |
// u32). Only 200 replies count.
func hostSessionUpdate(req, resp []byte) (kind int, title uint32, kid [8]byte) {
	line, _, _ := bytes.Cut(req, []byte("\r\n"))
	_, reqBody, ok := bytes.Cut(req, []byte("\r\n\r\n"))
	status, respBody, ok2 := bytes.Cut(resp, []byte("\r\n\r\n"))
	if !ok || !ok2 || !bytes.Contains(status, []byte(" 200 ")) || len(reqBody) < 12 {
		return hostNone, 0, kid
	}
	f := strings.Fields(string(line))
	if len(f) < 2 {
		return hostNone, 0, kid
	}
	path, _, _ := strings.Cut(strings.ToLower(f[1]), "?")
	switch path {
	case "/xmatch/xmatchhost2.ashx":
		if len(reqBody) < 16 {
			return hostNone, 0, kid
		}
		title = binary.LittleEndian.Uint32(reqBody[12:])
		if len(respBody) >= 24 { // create: the server's XNKID
			copy(kid[:], respBody[:8])
			return hostCreate, title, kid
		}
		copy(kid[:], reqBody[4:12])
		return hostModify, title, kid
	case "/xmatch/xmatchhostdelete2.ashx":
		copy(kid[:], reqBody[4:12])
		return hostDelete, 0, kid
	}
	return hostNone, 0, kid
}

// noteHostSession applies a relayed host request to session s's online users.
func (g *Gateway) noteHostSession(s *Session, req, resp []byte) {
	kind, title, kid := hostSessionUpdate(req, resp)
	if kind == hostNone {
		return
	}
	now := g.now()
	for xuid, o := range g.online {
		if o.s != s {
			continue
		}
		switch kind {
		case hostCreate, hostModify:
			if o.kid == kid && o.title == title {
				continue
			}
			o.title, o.kid, o.state = title, kid, statePresent|statePlaying|stateJoinable
		case hostDelete:
			if o.kid != kid {
				continue
			}
			o.kid, o.state = [8]byte{}, statePresent|statePlaying
		}
		g.pushPresence(xuid, o.title, now, o.kid, o.state)
	}
}
