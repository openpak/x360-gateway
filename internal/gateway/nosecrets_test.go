package gateway

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRelayLogCarriesNoSecrets(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	g := &Gateway{Log: log}
	raw := []byte("POST /svc/login?token=QUERYSECRET HTTP/1.1\r\nHost: h\r\nAuthorization: HEADERSECRET\r\nCookie: COOKIESECRET\r\nContent-Length: 10\r\n\r\nBODYSECRET")
	g.serve(raw, 80, "", "", log)
	out := logs.String()
	if !strings.Contains(out, "http request") {
		t.Fatalf("request line missing: %q", out)
	}
	for _, v := range []string{"QUERYSECRET", "BODYSECRET", "HEADERSECRET", "COOKIESECRET", "424f445953454352", "token="} {
		if strings.Contains(out, v) {
			t.Fatalf("%q leaked: %s", v, out)
		}
	}
}
