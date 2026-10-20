package call

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/sagernet/sing-box/transport/call/tunnel"
	"github.com/sagernet/sing-box/transport/call/wbstream"
	"github.com/sagernet/sing/common/logger"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelemostUnavailableJoinerLeavesBeforeRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var firstClosed atomic.Bool
	firstReady := make(chan *tunnel.RelayBridge, 1)
	recovering := make(chan bool, 1)
	var attempts atomic.Int32
	cfg := Config{Platform: "telemost", Role: RoleJoiner, Logger: logger.NOP(), PSK: []byte("01234567890123456789012345678901"), Rotation: &RotationConfig{Rooms: []RotationRoom{{Name: "a", JoinLink: "https://telemost.yandex.ru/j/111"}, {Name: "b", JoinLink: "https://telemost.yandex.ru/j/222"}}, StateFile: filepath.Join(t.TempDir(), "client.json")}}
	cfg.connectCarrier = func(context.Context, Config) (*Bridge, error) {
		if attempts.Add(1) != 1 {
			recovering <- firstClosed.Load()
			return nil, errors.New("test room unavailable")
		}
		rb := tunnel.NewRelayBridge(&testTunnel{}, "joiner", 32768, nil, logger.NOP())
		rb.SetShutdownHandler(func() { firstClosed.Store(true) })
		rb.MarkReady()
		firstReady <- rb
		return NewBridge(rb), nil
	}
	bridge, err := connectRotating(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	var first *tunnel.RelayBridge
	select {
	case first = <-firstReady:
	case <-ctx.Done():
		t.Fatal("first carrier not started")
	}
	// await guarantees the supervisor installed the unavailable callback.
	if _, err := bridge.managed.await(ctx); err != nil {
		t.Fatal(err)
	}
	first.NotifyUnavailable()
	select {
	case closed := <-recovering:
		if !closed {
			t.Fatal("recovery opened another joiner before closing unavailable peer")
		}
	case <-ctx.Done():
		t.Fatal("recovery did not start")
	}
}

func TestRotationAnnouncementRejectsTamperReplayAndWrongKey(t *testing.T) {
	key := []byte("test-key-01234567890123456789012345")
	now := time.Now()
	s := rotationAnnouncement{Version: 1, Epoch: 2, Active: 0, Next: 1, SwitchAt: now.Add(3 * time.Second)}
	s.sign(key)
	if !s.valid(key, 1, now) {
		t.Fatal("valid announcement rejected")
	}
	if s.valid(key, 2, now) {
		t.Fatal("replay accepted")
	}
	if s.valid([]byte("wrong-key"), 1, now) {
		t.Fatal("wrong key accepted")
	}
	s.Next = 0
	if s.valid(key, 1, now) {
		t.Fatal("tampered room accepted")
	}
	s.Next = 1
	s.SwitchAt = now.Add(time.Hour)
	s.sign(key)
	if s.valid(key, 1, now) {
		t.Fatal("unbounded switch time accepted")
	}
}

// Offline room broker exercises both supervisors and their real control frames.
type roomBroker struct {
	mu                      sync.Mutex
	owners, joiners         map[string]*brokerTunnel
	ownerCount, ownerStarts int
	secondRotation          chan struct{}
	t                       *testing.T
}
type brokerTunnel struct {
	broker  *roomBroker
	room    string
	creator bool
	onData  func([]byte)
	onClose func()
}

func (t *brokerTunnel) SendData(data []byte) {
	b := t.broker
	b.mu.Lock()
	peer := b.owners[t.room]
	if t.creator {
		peer = b.joiners[t.room]
	}
	var callback func([]byte)
	if peer != nil {
		callback = peer.onData
	}
	b.mu.Unlock()
	if callback != nil {
		callback(data)
	}
}
func (t *brokerTunnel) SetOnData(fn func([]byte)) {
	t.broker.mu.Lock()
	t.onData = fn
	t.broker.mu.Unlock()
}
func (t *brokerTunnel) SetOnClose(fn func()) {
	t.broker.mu.Lock()
	t.onClose = fn
	t.broker.mu.Unlock()
}
func (*brokerTunnel) Reconfigure(int, int) {}
func (b *roomBroker) connect(ctx context.Context, c Config) (*Bridge, error) {
	tun := &brokerTunnel{broker: b, room: c.JoinLink, creator: c.Role == RoleCreator}
	b.mu.Lock()
	if tun.creator {
		if b.ownerCount != 0 {
			b.t.Error("overlapping creator sessions")
		}
		b.ownerCount++
		b.ownerStarts++
		b.owners[tun.room] = tun
		if b.ownerStarts == 3 {
			close(b.secondRotation)
		}
	} else {
		b.joiners[tun.room] = tun
	}
	b.mu.Unlock()
	role := "joiner"
	if tun.creator {
		role = "creator"
	}
	rb := tunnel.NewRelayBridge(tun, role, 32768, nil, logger.NOP())
	rb.SetShutdownHandler(func() {
		b.mu.Lock()
		peers := b.owners
		peer := b.joiners[tun.room]
		if !tun.creator {
			peers = b.joiners
			peer = b.owners[tun.room]
		}
		if peers[tun.room] == tun {
			delete(peers, tun.room)
			if tun.creator {
				b.ownerCount--
			}
		}
		var callback func()
		if peer != nil {
			callback = peer.onClose
		}
		b.mu.Unlock()
		if callback != nil {
			callback()
		}
	})
	if err := rb.WaitPeer(ctx); err != nil {
		rb.Close()
		return nil, err
	}
	rb.MarkReady()
	return NewBridge(rb), nil
}

func TestTwoRoomCreatorAndClientCompleteTwoOfflineRotations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	owner, _ := wbstream.NewOwnerSession("__wb_device_id=test; wbx-refresh=test", "")
	b := &roomBroker{owners: make(map[string]*brokerTunnel), joiners: make(map[string]*brokerTunnel), secondRotation: make(chan struct{}), t: t}
	rooms := []RotationRoom{{Name: "a", JoinLink: "room-a"}, {Name: "b", JoinLink: "room-b"}}
	base := Config{Platform: "wbstream", OwnerSession: owner, Logger: logger.NOP(), PSK: []byte("01234567890123456789012345678901"), connectCarrier: b.connect}
	creatorCfg := base
	creatorCfg.Role = RoleCreator
	creatorCfg.Rotation = &RotationConfig{Rooms: rooms, StateFile: filepath.Join(t.TempDir(), "owner.json"), interval: func() time.Duration { return 20 * time.Millisecond }, switchDelay: 3 * time.Millisecond}
	clientCfg := base
	clientCfg.Role = RoleJoiner
	clientCfg.Rotation = &RotationConfig{Rooms: rooms}
	creator, err := Connect(ctx, creatorCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer creator.Close()
	client, err := Connect(ctx, clientCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-b.secondRotation:
	case <-ctx.Done():
		t.Fatal("two-room recovery/control exchange stalled")
	}
	// The third owner cannot confirm readiness until the client follows back to A.
	for {
		creator.managed.mu.Lock()
		active := creator.managed.active
		creator.managed.mu.Unlock()
		if active != nil && active.relay.PeerAlive(time.Now()) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("client did not confirm the new carrier")
		case <-time.After(time.Millisecond):
		}
	}
}

type testTunnel struct{}

func (*testTunnel) SendData([]byte)        {}
func (*testTunnel) SetOnData(func([]byte)) {}
func (*testTunnel) SetOnClose(func())      {}
func (*testTunnel) Reconfigure(int, int)   {}

func TestOwnerRotationClosesOldBeforeStartingNextAndPersistsEpoch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner, _ := wbstream.NewOwnerSession("__wb_device_id=test; wbx-refresh=test", "")
	var firstClosed atomic.Bool
	var calls atomic.Int32
	second := make(chan struct{})
	path := filepath.Join(t.TempDir(), "rotation.json")
	cfg := Config{Platform: "wbstream", Role: RoleCreator, OwnerSession: owner, Logger: logger.NOP(), PSK: []byte("01234567890123456789012345678901"), Rotation: &RotationConfig{Rooms: []RotationRoom{{Name: "a", JoinLink: "room-a"}, {Name: "b", JoinLink: "room-b"}}, StateFile: path, interval: func() time.Duration { return 5 * time.Millisecond }, switchDelay: 3 * time.Millisecond}}
	cfg.connectCarrier = func(_ context.Context, c Config) (*Bridge, error) {
		n := calls.Add(1)
		rb := tunnel.NewRelayBridge(&testTunnel{}, "creator", 32768, nil, logger.NOP())
		if n == 1 {
			rb.SetShutdownHandler(func() { firstClosed.Store(true) })
		}
		if n == 2 {
			if !firstClosed.Load() {
				t.Error("new owner started before old shutdown completed")
			}
			if c.JoinLink != "room-b" {
				t.Error("wrong alternate room")
			}
			close(second)
		}
		return NewBridge(rb), nil
	}
	bridge, err := Connect(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("rotation did not progress")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Index int
		Epoch uint64
	}
	if json.Unmarshal(data, &saved) != nil || saved.Index != 1 || saved.Epoch != 1 {
		t.Fatal("rotation epoch not persisted before switch")
	}
}

func TestManagedBridgeCancellationUnblocksWaitingDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &managedBridge{ctx: ctx, cancel: cancel, changed: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := m.await(context.Background()); done <- err }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("unexpected result %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("dial remained blocked after shutdown")
	}
}

func TestRotationRequiresIndependentPSKAndDistinctRooms(t *testing.T) {
	for _, cfg := range []Config{
		{Platform: "wbstream", Rotation: &RotationConfig{Rooms: []RotationRoom{{Name: "a", JoinLink: "a"}, {Name: "b", JoinLink: "b"}}}},
		{Platform: "wbstream", PSK: []byte("01234567890123456789012345678901"), Rotation: &RotationConfig{Rooms: []RotationRoom{{Name: "a", JoinLink: "same"}, {Name: "b", JoinLink: "same"}}}},
	} {
		if _, err := connectRotating(context.Background(), cfg); err == nil {
			t.Fatal("unsafe rotation accepted")
		}
	}
}
