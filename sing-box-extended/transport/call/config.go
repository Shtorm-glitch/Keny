package call

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/transport/call/bitrix"
	"github.com/sagernet/sing-box/transport/call/dion"
	"github.com/sagernet/sing-box/transport/call/telemost"
	"github.com/sagernet/sing-box/transport/call/tunnel"
	"github.com/sagernet/sing-box/transport/call/vk"
	"github.com/sagernet/sing-box/transport/call/wbstream"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type Role int

const (
	RoleCreator Role = iota
	RoleJoiner
)

type Config struct {
	Rotating       bool
	VP8FPS         int
	VP8Batch       int
	DualTrack      bool
	Reliable       *bool
	Platform       string
	Mode           string
	JoinLink       string
	Cookies        string
	CookieString   string
	Email          string
	Password       string
	ReadBuffer     int
	Role           Role
	Dialer         N.Dialer
	DNSRouter      adapter.DNSRouter
	Logger         logger.ContextLogger
	OwnerSession   *wbstream.OwnerSession
	PSK            []byte
	Rotation       *RotationConfig
	connectCarrier func(context.Context, Config) (*Bridge, error)
}

func Connect(ctx context.Context, cfg Config) (*Bridge, error) {
	if cfg.Rotation != nil {
		return connectRotating(ctx, cfg)
	}
	readBuf := cfg.ReadBuffer
	if readBuf <= 0 {
		readBuf = 32768
	}
	log := cfg.Logger
	if log == nil {
		log = logger.NOP()
	}
	cookieStr := cfg.CookieString
	if cookieStr == "" {
		cookieStr = cfg.Cookies
	}
	switch cfg.Platform {
	case "telemost":
		switch cfg.Role {
		case RoleCreator:
			relay, joinLink, err := telemost.ConnectCreator(ctx, cookieStr, cfg.JoinLink, readBuf, cfg.Dialer, log, telemost.CreatorOptions{PSK: cfg.PSK, ReadyWithoutPeer: cfg.Rotating})
			if err != nil {
				return nil, err
			}
			log.Notice(fmt.Sprintf("call[telemost]: join_link=%s", joinLink))
			return &Bridge{relay: relay}, nil
		case RoleJoiner:
			relay, err := telemost.ConnectJoiner(ctx, cfg.JoinLink, "", readBuf, cfg.Dialer, cfg.DNSRouter, log, telemost.JoinerOptions{VP8FPS: cfg.VP8FPS, VP8Batch: cfg.VP8Batch, DualTrack: cfg.DualTrack, Reliable: cfg.Reliable, PSK: cfg.PSK})
			if err != nil {
				return nil, err
			}
			return &Bridge{relay: relay}, nil
		}
	case "wbstream":
		switch cfg.Role {
		case RoleCreator:
			owner := cfg.OwnerSession
			if owner == nil {
				var err error
				owner, err = wbstream.NewOwnerSession(cookieStr, "")
				if err != nil {
					return nil, err
				}
			}
			relay, _, err := wbstream.ConnectCreatorWithSession(ctx, owner, cfg.JoinLink, cfg.Mode, readBuf, cfg.Dialer, log, cfg.PSK)
			if err != nil {
				return nil, err
			}
			log.Notice("call[wbstream]: creator connected")
			return &Bridge{relay: relay}, nil
		case RoleJoiner:
			relay, err := wbstream.ConnectJoiner(ctx, cfg.JoinLink, "", cfg.Mode, readBuf, cfg.Dialer, cfg.DNSRouter, log, cfg.PSK)
			if err != nil {
				return nil, err
			}
			return &Bridge{relay: relay}, nil
		}
	case "vk":
		switch cfg.Role {
		case RoleCreator:
			relay, joinLink, err := vk.ConnectCreator(ctx, cookieStr, cfg.JoinLink, readBuf, cfg.Dialer, cfg.DNSRouter, log)
			if err != nil {
				return nil, err
			}
			log.Notice(fmt.Sprintf("call[vk]: join_link=%s", joinLink))
			return &Bridge{relay: relay}, nil
		case RoleJoiner:
			relay, err := vk.ConnectJoiner(ctx, cfg.JoinLink, "", readBuf, cfg.Dialer, cfg.DNSRouter, log)
			if err != nil {
				return nil, err
			}
			return &Bridge{relay: relay}, nil
		}
	case "bitrix":
		switch cfg.Role {
		case RoleJoiner:
			relay, err := bitrix.ConnectJoiner(ctx, cfg.JoinLink, "", cfg.Mode, readBuf, cfg.Dialer, log)
			if err != nil {
				return nil, err
			}
			return &Bridge{relay: relay}, nil
		}
	case "dion":
		switch cfg.Role {
		case RoleCreator:
			relay, joinLink, err := dion.ConnectCreator(ctx, cookieStr, cfg.JoinLink, cfg.Email, cfg.Password, readBuf, cfg.Dialer, cfg.DNSRouter, log)
			if err != nil {
				return nil, err
			}
			log.Notice(fmt.Sprintf("call[dion]: join_link=%s", joinLink))
			return &Bridge{relay: relay}, nil
		case RoleJoiner:
			relay, err := dion.ConnectJoiner(ctx, cfg.JoinLink, "", readBuf, cfg.Dialer, cfg.DNSRouter, log)
			if err != nil {
				return nil, err
			}
			return &Bridge{relay: relay}, nil
		}
	}
	return nil, E.New("call: unsupported platform ", cfg.Platform)
}

type Bridge struct {
	relay   *tunnel.RelayBridge
	managed *managedBridge
}

func NewBridge(relay *tunnel.RelayBridge) *Bridge {
	return &Bridge{relay: relay}
}

func (b *Bridge) Close() error {
	if b.managed != nil {
		b.managed.cancel()
		if b.managed.done != nil {
			<-b.managed.done
		}
		return nil
	}
	b.relay.Close()
	return nil
}

func (b *Bridge) DialContext(ctx context.Context, destination string) (net.Conn, error) {
	if b.managed != nil {
		child, err := b.managed.await(ctx)
		if err != nil {
			return nil, err
		}
		return child.DialContext(ctx, destination)
	}
	return b.relay.DialContext(ctx, destination)
}

func (b *Bridge) ListenPacket(ctx context.Context, destination string) (net.Conn, error) {
	if b.managed != nil {
		child, err := b.managed.await(ctx)
		if err != nil {
			return nil, err
		}
		return child.ListenPacket(ctx, destination)
	}
	return b.relay.ListenPacket(ctx, destination)
}

func (b *Bridge) SetAcceptHandler(fn func(conn net.Conn, destination string)) {
	if b.managed != nil {
		b.managed.setHandler(fn, false)
		return
	}
	b.relay.SetAcceptHandler(fn)
}

func (b *Bridge) SetUDPAcceptHandler(fn func(conn net.Conn, destination string)) {
	if b.managed != nil {
		b.managed.setHandler(fn, true)
		return
	}
	b.relay.SetUDPAcceptHandler(fn)
}

type managedBridge struct {
	done              chan struct{}
	mu                sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	active            *Bridge
	changed           chan struct{}
	accept, udpAccept func(net.Conn, string)
}

func (m *managedBridge) await(ctx context.Context) (*Bridge, error) {
	for {
		m.mu.Lock()
		active, changed := m.active, m.changed
		m.mu.Unlock()
		if m.ctx.Err() != nil {
			return nil, m.ctx.Err()
		}
		if active != nil {
			return active, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.ctx.Done():
			return nil, m.ctx.Err()
		}
	}
}

func (m *managedBridge) setHandler(fn func(net.Conn, string), udp bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if udp {
		m.udpAccept = fn
		if m.active != nil {
			m.active.SetUDPAcceptHandler(fn)
		}
	} else {
		m.accept = fn
		if m.active != nil {
			m.active.SetAcceptHandler(fn)
		}
	}
}

func (m *managedBridge) activate(child *Bridge) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if child != nil {
		child.SetAcceptHandler(m.accept)
		child.SetUDPAcceptHandler(m.udpAccept)
	}
	m.active = child
	close(m.changed)
	m.changed = make(chan struct{})
}
