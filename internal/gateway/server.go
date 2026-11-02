package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"
)

// Server reads datagrams from one UDP socket and answers them inline.
type Server struct {
	Gateway *Gateway
	Log     *slog.Logger
}

// Serve runs until ctx is cancelled or the socket fails.
func (s *Server) Serve(ctx context.Context, conn *net.UDPConn) error {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go func() { // offline pushes for consoles that went silent
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				for _, d := range s.Gateway.Sweep() {
					conn.WriteToUDPAddrPort(d.Data, d.To)
				}
			}
		}
	}()
	buf := make([]byte, 65535)
	for {
		n, src, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		for _, d := range s.Gateway.HandleAll(buf[:n], src) {
			if _, err := conn.WriteToUDPAddrPort(d.Data, d.To); err != nil {
				s.Log.Warn("send failed", "to", d.To, "err", err)
			}
		}
	}
}
