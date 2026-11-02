package gateway

// Friend presence on the service-26 stream (wire spec: scratch/wharf/kdcstub/notes/re-presence-push.md).
// In memory, one gateway process: a registry xuid -> stream of the users whose
// command 100 is on an open stream; each stream remembers the friends the
// console asked to watch (commands 102/103). A user coming up or going away is
// pushed as a command 500 record to every stream that watches it, and the 303
// reply to a watch carries the watched friends that are online right now.

import (
	"encoding/binary"
	"time"
)

const (
	dashboardTitle = 0xFFFE07D1 // the dashboard; the gateway does not know the running title
	presenceRecord = 56         // 20-byte sub-header + 36-byte type 1001 payload
	xbox2003       = 1041379200 // unix time of 2003-01-01 00:00 UTC, the console's presence epoch
	statePresent   = 0x1
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
}

// presenceFrame is a command 500 push; every record is a 56-byte type 1001 record.
func presenceFrame(flags uint32, recs []byte) []byte {
	f := make([]byte, 0, 28+len(recs))
	for _, v := range []uint32{1, cmdPush, 0, flags, uint32(28 + len(recs)), 0, uint32(len(recs) / presenceRecord)} {
		f = binary.LittleEndian.AppendUint32(f, v)
	}
	return append(f, recs...)
}

// presenceRec is one type 1001 record: xuid, title id, time (seconds since 2003), state flags.
func presenceRec(xuid uint64, title uint32, t time.Time, state uint32) []byte {
	r := binary.LittleEndian.AppendUint16(nil, presenceRecord)
	r = binary.LittleEndian.AppendUint16(r, typePresence)
	r = binary.LittleEndian.AppendUint32(r, 0) // listener key
	r = binary.LittleEndian.AppendUint64(r, xuid)
	r = binary.LittleEndian.AppendUint32(r, 0) // c, never read
	r = append(r, make([]byte, 16)...)         // gamertag slot, unused
	r = binary.LittleEndian.AppendUint32(r, title)
	r = binary.LittleEndian.AppendUint32(r, uint32(t.Unix()-xbox2003))
	r = append(r, make([]byte, 8)...) // session XNKID: none known
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
	g.online[xuid] = &onlineUser{s: s, c: c, title: dashboardTitle, since: now}
	c.stream.users = append(c.stream.users, xuid)
	if prev == nil {
		g.pushPresence(xuid, dashboardTitle, now, statePresent)
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
			out = append(out, presenceRec(x, o.title, o.since, statePresent)...)
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
func (g *Gateway) pushPresence(xuid uint64, title uint32, t time.Time, state uint32) {
	rec := presenceRec(xuid, title, t, state)
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
		g.pushPresence(x, 0, now, 0)
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
