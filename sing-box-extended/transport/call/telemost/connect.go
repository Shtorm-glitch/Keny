package telemost

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/transport/call/common"
	"github.com/sagernet/sing-box/transport/call/tunnel"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type CreatorOptions struct {
	PSK              []byte
	ReadyWithoutPeer bool
}

func ConnectCreator(ctx context.Context, cookieStr, joinLink string, readBuf int, dialer N.Dialer, logger logger.ContextLogger, tuning ...CreatorOptions) (*tunnel.RelayBridge, string, error) {
	ctx, cancel := context.WithCancel(ctx)
	owner := newCreatorHTTPDialer(ctx, dialer)
	connected := false
	defer func() {
		if !connected {
			cancel()
			owner.client.CloseIdleConnections()
		}
	}()
	dialer = owner
	var options CreatorOptions
	if len(tuning) != 0 {
		options = tuning[0]
	}
	cfg, err := FetchConfig(dialer, logger)
	if err != nil {
		return nil, "", err
	}
	var connInfo *ConnInfo
	if joinLink != "" {
		connInfo, err = joinExistingConference(dialer, cookieStr, joinLink, cfg, logger)
	} else {
		connInfo, err = CreateAndJoinCall(dialer, cookieStr, cfg, logger)
	}
	if err != nil {
		return nil, "", err
	}
	if readBuf <= 0 {
		readBuf = 32768
	}
	bridge := &Bridge{
		ctx: ctx, ready: make(chan struct{}), done: make(chan struct{}), secret: options.PSK, readyWithoutPeer: options.ReadyWithoutPeer,
		connInfo:  connInfo,
		config:    cfg,
		cookieStr: cookieStr,
		peers:     make(map[string]string),
		seenPids:  make(map[string]bool),
		readBuf:   readBuf,
		dialer:    dialer,
		logger:    logger,
	}
	bridge.ctx = ctx
	go func() {
		defer cancel()
		defer owner.client.CloseIdleConnections()
		bridge.Run()
	}()
	go func() {
		select {
		case <-ctx.Done():
			bridge.mu.Lock()
			ws := bridge.ws
			bridge.mu.Unlock()
			common.CloseWS(ws)
		case <-bridge.done:
		}
	}()
	select {
	case <-bridge.ready:
		bridge.mu.Lock()
		rb := bridge.activeBridge
		bridge.mu.Unlock()
		rb.SetShutdownHandler(func() { cancel(); <-bridge.done })
		connected = true
		return rb, connInfo.ConferenceURI, nil
	case <-ctx.Done():
		cancel()
		<-bridge.done
		return nil, "", ctx.Err()
	}
}

type JoinerOptions struct {
	PSK       []byte `json:"psk,omitempty"`
	VP8FPS    int    `json:"vp8Fps,omitempty"`
	VP8Batch  int    `json:"vp8Batch,omitempty"`
	DualTrack bool   `json:"dualTrack,omitempty"`
	Reliable  *bool  `json:"reliable,omitempty"`
}

func joinerParams(joinLink, displayName string, options JoinerOptions) string {
	if options.Reliable == nil {
		enabled := true
		options.Reliable = &enabled
	}
	data, _ := json.Marshal(struct {
		JoinLink    string `json:"joinLink"`
		DisplayName string `json:"displayName"`
		JoinerOptions
	}{joinLink, displayName, options})
	return string(data)
}

func ConnectJoiner(ctx context.Context, joinLink, displayName string, readBuf int, dialer N.Dialer, dnsRouter adapter.DNSRouter, logger logger.ContextLogger, tuning ...JoinerOptions) (*tunnel.RelayBridge, error) {
	ctx, cancel := context.WithCancel(ctx)
	dialer = carrierDialer{Dialer: dialer, ctx: ctx}
	if displayName == "" {
		displayName = "Joiner"
	}
	if readBuf <= 0 {
		readBuf = 32768
	}
	joiner := NewTelemostJoiner(
		logger,
		dialer,
		dnsRouter,
		nil,
		common.AddTunnelTracks,
		common.ReadTrack,
	)
	tunCh := make(chan tunnel.DataTunnel, 1)
	var bridgeMu sync.Mutex
	var active *tunnel.RelayBridge
	joiner.OnConnected = func(tun tunnel.DataTunnel) {
		bridgeMu.Lock()
		defer bridgeMu.Unlock()
		if active != nil {
			active.SwapTunnel(tun)
			active.MarkReady()
			return
		}
		select {
		case tunCh <- tun:
		default:
		}
	}
	var options JoinerOptions
	if len(tuning) != 0 {
		options = tuning[0]
	}
	params := joinerParams(joinLink, displayName, options)
	go joiner.RunWithParams(params)
	select {
	case tun := <-tunCh:
		rb := tunnel.NewRelayBridge(tun, "joiner", readBuf, dialer, logger)
		rb.SetOnConfigAck(joiner.MarkConfigAcked)
		rb.MarkReady()
		bridgeMu.Lock()
		active = rb
		bridgeMu.Unlock()
		rb.SetShutdownHandler(func() { cancel(); joiner.Close() })
		go func() { <-ctx.Done(); rb.Close() }()
		if len(options.PSK) > 0 {
			if err := rb.WaitPeer(ctx); err != nil {
				rb.Close()
				return nil, err
			}
		}
		return rb, nil
	case <-ctx.Done():
		cancel()
		joiner.Close()
		return nil, ctx.Err()
	}
}

func CreateConferenceForTest(dialer N.Dialer, cookieStr string) (string, error) {
	nop := logger.NOP()
	cfg, err := FetchConfig(dialer, nop)
	if err != nil {
		return "", err
	}
	connInfo, err := CreateAndJoinCall(dialer, cookieStr, cfg, nop)
	if err != nil {
		return "", err
	}
	return connInfo.ConferenceURI, nil
}
