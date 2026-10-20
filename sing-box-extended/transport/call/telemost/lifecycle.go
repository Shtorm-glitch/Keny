package telemost

import (
	"context"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"net"
)

// Bind signaling sockets to the carrier, including HTTP calls that have no
// request context. Switching a room must also stop in-flight authentication.
type carrierDialer struct {
	N.Dialer
	ctx context.Context
}

var _ dialer.ResolveDialer = carrierDialer{}

func (d carrierDialer) QueryOptions() adapter.DNSQueryOptions {
	if resolver, ok := d.Dialer.(dialer.ResolveDialer); ok {
		return resolver.QueryOptions()
	}
	return adapter.DNSQueryOptions{}
}

func (d carrierDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if err := d.ctx.Err(); err != nil {
		return nil, err
	}
	request, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(d.ctx, cancel)
	conn, err := d.Dialer.DialContext(request, network, destination)
	stop()
	cancel()
	if err != nil {
		return nil, err
	}
	wrapped := &carrierConn{Conn: conn}
	wrapped.stop = context.AfterFunc(d.ctx, func() { conn.Close() })
	return wrapped, nil
}

type carrierConn struct {
	net.Conn
	stop func() bool
}

func (c *carrierConn) Close() error { c.stop(); return c.Conn.Close() }
