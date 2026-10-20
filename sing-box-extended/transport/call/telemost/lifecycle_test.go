package telemost

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"net"
	"testing"
	"time"
)

type pipeDialer struct {
	N.Dialer
	conn net.Conn
}

type resolverPipeDialer struct {
	pipeDialer
	options adapter.DNSQueryOptions
}

func (d resolverPipeDialer) QueryOptions() adapter.DNSQueryOptions { return d.options }
func TestCarrierPreservesDNSQueryOptions(t *testing.T) {
	expected := adapter.DNSQueryOptions{DisableCache: true, Timeout: 19 * time.Millisecond}
	d := carrierDialer{Dialer: resolverPipeDialer{options: expected}, ctx: context.Background()}
	got := d.QueryOptions()
	if !got.DisableCache || got.Timeout != expected.Timeout {
		t.Fatal("carrier lost configured DNS options")
	}
}

func (d pipeDialer) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return d.conn, nil
}
func TestCarrierCancellationClosesSignalingConnection(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := carrierDialer{Dialer: pipeDialer{conn: left}, ctx: ctx}
	c, err := d.DialContext(context.Background(), "tcp", M.Socksaddr{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cancel()
	right.SetReadDeadline(time.Now().Add(time.Second))
	_, err = right.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("signaling survived room shutdown")
	}
	if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("cancellation did not close signaling")
	}
	if _, err = d.DialContext(context.Background(), "tcp", M.Socksaddr{}); err != context.Canceled {
		t.Fatal("cancelled carrier opened a new socket")
	}
}
