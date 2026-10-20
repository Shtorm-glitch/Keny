package v2raygrpclite

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
)

type testDialer struct{}

func (testDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address.String())
}

func (testDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	panic("unused")
}

// Exercise the HTTP/2 wire path, including the escaping of a service name
// containing slashes. Both peers compare the decoded service path.
func TestServiceNameWirePath(t *testing.T) {
	for _, tc := range []struct{ name, wire, decoded string }{
		{"", "//Tun", "//Tun"},
		{"service", "/service/Tun", "/service/Tun"},
		{"api/test", "/api%2Ftest/Tun", "/api/test/Tun"},
		{"/api/test/Tun", "/%2Fapi%2Ftest%2FTun/Tun", "//api/test/Tun/Tun"},
		{"api%2Ftest", "/api%252Ftest/Tun", "/api%2Ftest/Tun"},
		{"api test?#", "/api%20test%3F%23/Tun", "/api test?#/Tun"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound, err := NewServer(context.Background(), nil, option.V2RayGRPCOptions{ServiceName: tc.name}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if inbound.path != tc.decoded {
				t.Fatalf("server path = %q, want %q", inbound.path, tc.decoded)
			}
			paths := make(chan [2]string, 1)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths <- [2]string{r.RequestURI, r.URL.Path}
				w.Header().Set("Content-Type", "application/grpc")
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				io.Copy(io.Discard, r.Body)
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()
			client := NewClient(context.Background(), testDialer{}, M.ParseSocksaddr(server.Listener.Addr().String()), option.V2RayGRPCOptions{ServiceName: tc.name}, nil).(*Client)
			// Use the test server's trusted TLS connection; keep the production
			// HTTP/2 transport and request construction intact.
			client.transport.DialTLSContext = func(ctx context.Context, network, address string, _ *tls.Config) (net.Conn, error) {
				config := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
				config.NextProtos = []string{"h2"}
				return (&tls.Dialer{Config: config}).DialContext(ctx, network, address)
			}
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, err := client.DialContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			select {
			case got := <-paths:
				if got[0] != tc.wire || got[1] != tc.decoded {
					t.Fatalf("wire/decoded path = %q, want %q / %q", got, tc.wire, tc.decoded)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
