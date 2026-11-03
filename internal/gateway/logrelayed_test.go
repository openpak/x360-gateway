package gateway

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestLogRelayedHasNoQuery(t *testing.T) {
	var buf bytes.Buffer
	old, fl := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() { log.SetOutput(old); log.SetFlags(fl) }()

	logRelayed("POST /xuacs/login?token=sekrit HTTP/1.1", "200 OK", time.Now())
	if !regexp.MustCompile(`^http POST /xuacs/login -> 200 \d+ms\n$`).MatchString(buf.String()) {
		t.Fatalf("line = %q", buf.String())
	}
	if strings.Contains(buf.String(), "sekrit") {
		t.Fatal("query leaked")
	}
	buf.Reset()
	logRelayed("garbage", "", time.Now()) // must neither panic nor log
	logRelayed("GET /x HTTP/1.1", "", time.Now())
	if !strings.Contains(buf.String(), "http GET /x -> 0 ") {
		t.Fatalf("line = %q", buf.String())
	}
}
