package call

import (
	"context"
	"net"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/call"
	"github.com/sagernet/sing-box/transport/call/wbstream"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

func RegisterInbound(registry *inbound.Registry) {
	inbound.Register[option.CallInboundOptions](registry, C.TypeCall, NewInbound)
}

type Inbound struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	closed bool
	psk    []byte
	owner  *wbstream.OwnerSession
	inbound.Adapter
	ctx     context.Context
	router  adapter.ConnectionRouterEx
	logger  logger.ContextLogger
	options option.CallInboundOptions
	dialer  N.Dialer
	bridge  *call.Bridge
}

func NewInbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.CallInboundOptions) (adapter.Inbound, error) {
	if options.Platform == "" {
		return nil, E.New("missing platform")
	}
	if options.Rotation != nil && (options.Rotation.StateFile == "" || (options.Platform == "wbstream" && options.SessionStateFile == "")) {
		return nil, E.New("call: creator rotation requires durable session and rotation state files")
	}
	psk, err := readPSK(options.CallCommonOptions)
	if err != nil {
		return nil, err
	}
	var owner *wbstream.OwnerSession
	if options.Platform == "wbstream" {
		owner, err = wbstream.NewOwnerSession(options.Cookies.Header(), options.SessionStateFile)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, true)
	if err != nil {
		cancel()
		return nil, err
	}
	return &Inbound{
		cancel: cancel, psk: psk, owner: owner,
		Adapter: inbound.NewAdapter(C.TypeCall, tag),
		ctx:     ctx,
		router:  router,
		logger:  logger,
		options: options,
		dialer:  outboundDialer,
	}, nil
}

func (h *Inbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStatePostStart {
		return nil
	}
	go h.run()
	return nil
}

func (h *Inbound) Close() error {
	h.mu.Lock()
	h.closed = true
	h.cancel()
	bridge := h.bridge
	h.mu.Unlock()
	if bridge == nil {
		return nil
	}
	return bridge.Close()
}

func (h *Inbound) run() {
	dnsRouter := service.FromContext[adapter.DNSRouter](h.ctx)
	bridge, err := call.Connect(h.ctx, call.Config{
		OwnerSession: h.owner, PSK: h.psk, Rotation: rotationConfig(h.options.CallCommonOptions),
		Platform:     h.options.Platform,
		Mode:         h.options.Mode,
		JoinLink:     h.options.JoinLink,
		CookieString: h.options.Cookies.Header(),
		Email:        h.options.Email,
		Password:     h.options.Password,
		ReadBuffer:   h.options.ReadBuffer,
		Role:         call.RoleCreator,
		Dialer:       h.dialer,
		DNSRouter:    dnsRouter,
		Logger:       h.logger,
	})
	if err != nil {
		h.logger.ErrorContext(h.ctx, "call: creator connection failed")
		return
	}
	bridge.SetAcceptHandler(func(conn net.Conn, destination string) {
		h.handleConnection(conn, M.ParseSocksaddr(destination))
	})
	bridge.SetUDPAcceptHandler(func(conn net.Conn, destination string) {
		h.handlePacketConnection(bufio.NewUnbindPacketConnWithAddr(conn, M.ParseSocksaddr(destination)), M.ParseSocksaddr(destination))
	})
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		bridge.Close()
		return
	}
	h.bridge = bridge
	h.mu.Unlock()
}

func (h *Inbound) handleConnection(conn net.Conn, destination M.Socksaddr) {
	ctx := log.ContextWithNewID(h.ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.Source = M.Socksaddr{}
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "inbound connection to ", destination)
	h.router.RouteConnectionEx(ctx, deadline.NewConn(conn), metadata, N.OnceClose(func(it error) {
		conn.Close()
	}))
}

func (h *Inbound) handlePacketConnection(conn N.PacketConn, destination M.Socksaddr) {
	ctx := log.ContextWithNewID(h.ctx)
	var metadata adapter.InboundContext
	metadata.Inbound = h.Tag()
	metadata.InboundType = h.Type()
	metadata.Source = M.Socksaddr{}
	metadata.Destination = destination
	h.logger.InfoContext(ctx, "inbound packet connection to ", destination)
	h.router.RoutePacketConnectionEx(ctx, conn, metadata, N.OnceClose(func(it error) {
		conn.Close()
	}))
}
