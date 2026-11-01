package gateway

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// UDP inside the secure channel: codes 4..6 (4 + port mode), the TCP layout
// with a run of the ports alone (XAM receive path FUN_817551e0; port modes as
// for TCP: 0 = both 1000, 1 = one byte each + 1000, 2 = u16 each big-endian).
//
// XAM sends one kind of UDP to the gateway: arbitration (Live service 16,
// default port 1000, FUN_81917ab0) posts a whole HTTP/1.0 request in one
// datagram (TimeExtend, and ReportResult when the XRL post fails), waits 1 s,
// tries 3 times, and only looks for "200 OK" and "X-Err: <hex>" in the reply
// (FUN_81918858).
const (
	codeUDP16   = 6
	xarbUDPPort = 1000
)

func isUDP(pkt []byte) bool {
	return len(pkt) >= 4 && pkt[0]&0x0f >= 4 && pkt[0]&0x0f <= 6
}

func sealUDP(d dirKeys, spi []byte, counter uint32, src, dst uint16, payload []byte) []byte {
	run := binary.BigEndian.AppendUint16(nil, src)
	run = binary.BigEndian.AppendUint16(run, dst)
	return sealRun(d, spi, counter, codeUDP16, payload, run)
}

func openUDP(d dirKeys, pkt []byte, hi uint16) (src, dst uint16, payload []byte, ctr uint32, err error) {
	mode := int(pkt[0]&0x0f) - 4
	run, payload, ctr, err := openRun(d, pkt, hi, 2*mode)
	if err != nil {
		return
	}
	switch mode {
	case 0:
		src, dst = 1000, 1000
	case 1:
		src, dst = uint16(run[0])+1000, uint16(run[1])+1000
	case 2:
		src, dst = binary.BigEndian.Uint16(run), binary.BigEndian.Uint16(run[2:])
	}
	return
}

func (g *Gateway) udpData(s *Session, pkt []byte, now time.Time) ([][]byte, error) {
	if string(pkt[1:4]) != string(s.ServerSPI[1:]) {
		return nil, errors.New("UDP SPI is not this session's")
	}
	var (
		src, dst uint16
		payload  []byte
		ctr      uint32
		err      error
	)
	for _, hi := range []uint16{uint16(s.lastRecv >> 16), uint16(s.lastRecv>>16) + 1} {
		if src, dst, payload, ctr, err = openUDP(s.c2s, pkt, hi); err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	s.expires = now.Add(SessionTTL)
	s.lastSeen = now
	if ctr > s.lastRecv {
		s.lastRecv = ctr
	}
	log := g.log().With("src", s.Client, "account", s.Account)
	log.Info("udp rx", "ports", fmt.Sprintf("%d>%d", src, dst), "len", len(payload), "ctr", ctr)
	if dst != xarbUDPPort {
		log.Info("udp to an unserved port, dropped", "port", dst)
		return nil, nil
	}
	// ponytail: the HTTP round trip runs on the listener loop, as for TCP requests.
	resp := g.serve(payload, dst, s.Account, usersHeader(s.Users), log)
	s.sendCtr++
	return [][]byte{sealUDP(s.s2c, s.ClientSPI, s.sendCtr, dst, src, resp)}, nil
}
