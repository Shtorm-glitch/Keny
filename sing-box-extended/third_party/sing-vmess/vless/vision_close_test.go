package vless

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestVisionCloseAfterDirectWrite(t *testing.T) {
	wrapped, wrappedPeer := net.Pipe()
	defer wrapped.Close()
	defer wrappedPeer.Close()
	raw, rawPeer := net.Pipe()
	defer raw.Close()
	defer rawPeer.Close()
	conn := &VisionConn{Conn: wrapped, netConn: raw, directConn: raw}
	conn.directWrite.Store(true)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	_ = rawPeer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := rawPeer.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("raw connection remained open after Vision close: %v", err)
	}
}
