package transport

import (
	"context"
	"net"
	"testing"
	"time"

	mDNS "github.com/miekg/dns"
)

// A TUN dispatcher calls ExchangeAsync inline. Waiting for a dial here stops
// unrelated TCP packets and DNS responses from traversing the interface.
func TestExchangeAsyncReturnsWhileDialPending(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dialEntered := make(chan struct{})
	callback := make(chan error, 1)
	m := newQueryMultiplexer(queryMultiplexerOptions{
		dial: func(ctx context.Context) (net.Conn, error) {
			close(dialEntered)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	defer m.Close()
	message := new(mDNS.Msg)
	message.SetQuestion("pending.example.", mDNS.TypeA)
	returned := make(chan struct{})
	go func() {
		m.ExchangeAsync(ctx, message, func(_ *mDNS.Msg, err error) { callback <- err })
		close(returned)
	}()
	select {
	case <-dialEntered:
	case <-time.After(time.Second):
		t.Fatal("dial did not start")
	}
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Error("ExchangeAsync blocked its caller while waiting for a DNS connection")
	}
	cancel()
	select {
	case err := <-callback:
		if err == nil {
			t.Error("expected cancellation error")
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not complete query")
	}
}
