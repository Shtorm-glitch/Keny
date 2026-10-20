package tunnel

import (
	"github.com/sagernet/sing/common/logger"
	"testing"
	"time"
)

type idleTestTunnel struct{}

func (*idleTestTunnel) SendData([]byte)        {}
func (*idleTestTunnel) SetOnData(func([]byte)) {}
func (*idleTestTunnel) SetOnClose(func())      {}
func (*idleTestTunnel) Reconfigure(int, int)   {}
func TestIdleTracksPayloadButNotControlTraffic(t *testing.T) {
	rb := NewRelayBridge(&idleTestTunnel{}, "creator", 32768, nil, logger.NOP())
	rb.lastData.Store(time.Now().Add(-time.Minute).UnixNano())
	rb.handleTunnelData(EncodeFrame(ControlConnID, MsgHello, []byte("nonce")))
	if !rb.IdleFor(time.Now(), 30*time.Second) {
		t.Fatal("heartbeat deferred planned rotation")
	}
	rb.handleTunnelData(EncodeFrame(42, MsgData, []byte("payload")))
	if rb.IdleFor(time.Now(), 30*time.Second) {
		t.Fatal("received payload did not defer planned rotation")
	}
	rb.lastData.Store(time.Now().Add(-time.Minute).UnixNano())
	rb.send(42, MsgData, []byte("payload"))
	if rb.IdleFor(time.Now(), 30*time.Second) {
		t.Fatal("sent payload did not defer planned rotation")
	}
}
