package tunnel

import (
	"context"
	"github.com/sagernet/sing/common/logger"
	"testing"
	"time"
)

type linkedTunnel struct {
	peer   *linkedTunnel
	onData func([]byte)
}

func (t *linkedTunnel) SendData(data []byte) {
	if t.peer != nil && t.peer.onData != nil {
		t.peer.onData(data)
	}
}
func (t *linkedTunnel) SetOnData(fn func([]byte)) { t.onData = fn }
func (*linkedTunnel) SetOnClose(func())           {}
func (*linkedTunnel) Reconfigure(int, int)        {}

func TestPeerConfirmationNeedsRemoteResponse(t *testing.T) {
	local := NewRelayBridge(&linkedTunnel{}, "joiner", 32768, nil, logger.NOP())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if local.WaitPeer(ctx) == nil {
		t.Fatal("empty room reported ready")
	}
}

func TestPeerConfirmationRoundTrip(t *testing.T) {
	a, b := &linkedTunnel{}, &linkedTunnel{}
	a.peer = b
	b.peer = a
	creator := NewRelayBridge(a, "creator", 32768, nil, logger.NOP())
	joiner := NewRelayBridge(b, "joiner", 32768, nil, logger.NOP())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := joiner.WaitPeer(ctx); err != nil {
		t.Fatal(err)
	}
	if err := creator.WaitPeer(ctx); err != nil {
		t.Fatal(err)
	}
	if !creator.PeerAlive(time.Now()) || !joiner.PeerAlive(time.Now()) {
		t.Fatal("confirmed peer not alive")
	}
}
