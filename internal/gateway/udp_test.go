package gateway

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSealOpenUDP(t *testing.T) {
	c2s, _ := testKeys(t)
	spi := []byte{0, 1, 2, 3}
	for n := 0; n < 20; n++ { // pads 0..7, so the run is split every way
		in := bytes.Repeat([]byte{0x41}, n)
		pkt := sealUDP(c2s, spi, 5, 5001, 1000, in)
		if pkt[0]&0x0f != codeUDP16 || len(pkt) != 4+n+(-n&7)+4-min(-n&7, 4)+12 {
			t.Fatalf("len %d: packet %x", n, pkt)
		}
		src, dst, p, ctr, err := openUDP(c2s, pkt, 0)
		if err != nil || src != 5001 || dst != 1000 || ctr != 5 || !bytes.Equal(p, in) {
			t.Fatalf("len %d: %d>%d %x ctr %d %v", n, src, dst, p, ctr, err)
		}
		pkt[len(pkt)-1] ^= 1
		if _, _, _, _, err := openUDP(c2s, pkt, 0); err == nil {
			t.Fatal("tampered ICV accepted")
		}
	}
}

// An arbitration datagram becomes an HTTP request to the backend, and the
// backend's answer comes back as one datagram from port 1000.
func TestXArbOverUDP(t *testing.T) {
	var gotPath, gotAcct string
	var gotBody []byte
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAcct = r.URL.Path, r.Header.Get(AccountHeader)
		gotBody, _ = io.ReadAll(r.Body)
	}))
	defer up.Close()
	f := newFake(t, up.URL)
	body := "\x01\x00\x00\x00\x07\x08\x09\x0a"
	req := "POST /XArb/TimeExtend.ashx HTTP/1.0\r\nUser-Agent: 16/1.00.17559\r\nContent-type: xon/16\r\nContent-Length: 8\r\n\r\n" + body
	pkt := sealUDP(f.c2s, []byte{0, 0xab, 0xc1, 0x23}, 1, 5001, xarbUDPPort, []byte(req))
	out := f.g.Handle(pkt, f.src)
	if len(out) != 1 {
		t.Fatalf("%d replies", len(out))
	}
	src, dst, resp, _, err := openUDP(f.s2c, out[0], 0)
	if err != nil || src != xarbUDPPort || dst != 5001 || !strings.Contains(string(resp), "200 OK") {
		t.Fatalf("%d>%d %q %v", src, dst, resp, err)
	}
	if gotPath != "/XArb/TimeExtend.ashx" || gotAcct != "1@xbox.com" || string(gotBody) != body {
		t.Fatalf("backend saw %s %q %x", gotPath, gotAcct, gotBody)
	}
}
