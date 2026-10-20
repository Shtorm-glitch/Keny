package wbstream

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/transport/call/common"
	"github.com/sagernet/sing-box/transport/call/tunnel"
	"github.com/sagernet/sing-box/transport/call/tunnel/rtc"
	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

func ConnectCreator(ctx context.Context, cookieStr, roomID, mode string, readBuf int, dialer N.Dialer, logger logger.ContextLogger) (*tunnel.RelayBridge, string, error) {
	owner, err := NewOwnerSession(cookieStr, "")
	if err != nil {
		return nil, "", err
	}
	return ConnectCreatorWithSession(ctx, owner, roomID, mode, readBuf, dialer, logger)
}

func ConnectCreatorWithSession(ctx context.Context, owner *OwnerSession, roomID, mode string, readBuf int, dialer N.Dialer, logger logger.ContextLogger, psk ...[]byte) (*tunnel.RelayBridge, string, error) {
	ctx, cancel := context.WithCancel(ctx)
	transferred := false
	defer func() {
		if !transferred {
			cancel()
		}
	}()
	httpClient := common.HttpClient(dialer)
	httpClient.Timeout = 30 * time.Second
	httpClient.Transport = contextTransport{ctx: ctx, base: httpClient.Transport}
	cookieHeader, bearer, generation, err := owner.Credentials(ctx, httpClient, 0)
	if err != nil {
		return nil, "", fmt.Errorf("wbstream: slide-v3 refresh: %w", err)
	}
	requestedRoom := ParseRoomID(roomID)
	resolvedRoomID, roomToken, accessToken, serverURL, err := AuthAsLoggedIn(httpClient, cookieHeader, bearer, requestedRoom, "Creator")
	if err != nil {
		return nil, "", fmt.Errorf("wbstream: auth: %w", err)
	}
	if readBuf <= 0 {
		readBuf = 32768
	}
	if mode == "" {
		mode = TunnelModeDC
	}
	secret := tunnel.DeriveSecretFromJoinLink(resolvedRoomID)
	if len(psk) > 0 && len(psk[0]) > 0 {
		secret = psk[0]
	}
	obf, err := tunnel.NewTunnelObfuscator(secret)
	if err != nil {
		return nil, "", fmt.Errorf("wbstream: obfuscator init: %w", err)
	}

	var relay *tunnel.RelayBridge
	joinSession := func(token, access, server string) (*Session, <-chan tunnel.DataTunnel) {
		tunCh := make(chan tunnel.DataTunnel, 1)
		sess := NewSession(SessionConfig{
			Context:     ctx,
			RoomToken:   token,
			ServerURL:   server,
			DisplayName: "Creator",
			TunnelMode:  mode,
			Reliable:    mode == TunnelModeVideo && len(psk) > 0 && len(psk[0]) > 0,
			Obfuscator:  obf,
			Logger:      logger,
			Dialer:      dialer,
			RoomID:      resolvedRoomID,
			AccessToken: access,
			ReadBuf:     readBuf,
		})
		sess.OnConnected = func(tun tunnel.DataTunnel) {
			select {
			case tunCh <- tun:
			default:
			}
		}
		sess.OnPeerRestart = func() {
			if relay != nil {
				relay.Reset()
			}
		}
		return sess, tunCh
	}

	sess, tunCh := joinSession(roomToken, accessToken, serverURL)
	if err := sess.Start(); err != nil {
		sess.Close()
		return nil, "", fmt.Errorf("wbstream: session start: %w", err)
	}
	var firstTun tunnel.DataTunnel
	select {
	case firstTun = <-tunCh:
	case <-ctx.Done():
		sess.Close()
		return nil, "", ctx.Err()
	case <-time.After(60 * time.Second):
		sess.Close()
		return nil, "", fmt.Errorf("wbstream: creator tunnel timed out")
	}

	relay = tunnel.NewRelayBridge(firstTun, "creator", bridgeReadBufFor(firstTun, readBuf), dialer, logger)
	done := make(chan struct{})
	relay.SetShutdownHandler(func() { cancel(); <-done })
	go func() {
		defer close(done)
		creatorReconnectLoop(ctx, relay, sess, joinSession, httpClient, owner, generation, resolvedRoomID, readBuf, logger)
	}()
	transferred = true
	peerCtx, peerCancel := context.WithTimeout(ctx, 60*time.Second)
	defer peerCancel()
	if err := relay.WaitPeer(peerCtx); err != nil {
		relay.Close()
		return nil, "", fmt.Errorf("wbstream: creator peer confirmation failed")
	}
	return relay, APIBase + "/room/" + resolvedRoomID, nil
}

func ConnectJoiner(ctx context.Context, roomID, displayName, mode string, readBuf int, dialer N.Dialer, dnsRouter adapter.DNSRouter, logger logger.ContextLogger, psk ...[]byte) (*tunnel.RelayBridge, error) {
	roomID = ParseRoomID(roomID)
	if displayName == "" {
		displayName = "Joiner"
	}
	if mode == "" {
		mode = TunnelModeDC
	}
	if readBuf <= 0 {
		readBuf = 32768
	}
	joiner := NewWBStreamJoiner(logger, dialer, dnsRouter, nil)
	if len(psk) > 0 {
		joiner.secret = append([]byte(nil), psk[0]...)
	}
	tunCh := make(chan tunnel.DataTunnel, 1)
	var relayMu sync.Mutex
	var relay *tunnel.RelayBridge
	joiner.OnConnected = func(tun tunnel.DataTunnel) {
		relayMu.Lock()
		defer relayMu.Unlock()
		if relay != nil {
			relay.SwapTunnel(tun)
			return
		}
		select {
		case tunCh <- tun:
		default:
		}
	}
	joiner.onDisconnected = func() {
		relayMu.Lock()
		rb := relay
		relayMu.Unlock()
		if rb != nil {
			rb.NotifyUnavailable()
		}
	}
	params := fmt.Sprintf(`{"roomId":%q,"displayName":%q,"tunnelMode":%q,"reliable":%t}`, roomID, displayName, mode, mode == TunnelModeVideo && len(psk) > 0 && len(psk[0]) > 0)
	go joiner.RunWithParams(params)
	select {
	case tun := <-tunCh:
		rb := tunnel.NewRelayBridge(tun, "joiner", bridgeReadBufFor(tun, readBuf), dialer, logger)
		relayMu.Lock()
		relay = rb
		relayMu.Unlock()
		rb.SetOnConfigAck(joiner.MarkConfigAcked)
		rb.MarkReady()
		if err := rb.WaitPeer(ctx); err != nil {
			joiner.Close()
			rb.Close()
			return nil, err
		}
		go func() { <-ctx.Done(); joiner.Close(); rb.Close() }()
		return rb, nil
	case <-ctx.Done():
		joiner.Close()
		return nil, ctx.Err()
	}
}

func creatorReconnectLoop(
	ctx context.Context,
	relay *tunnel.RelayBridge,
	sess *Session,
	joinSession func(token, access, server string) (*Session, <-chan tunnel.DataTunnel),
	httpClient *http.Client,
	owner *OwnerSession,
	generation uint64,
	roomID string,
	readBuf int,
	logger logger.ContextLogger,
) {
	current := sess
	defer func() { current.Close() }()
	for {
		select {
		case <-current.Done():
			relay.NotifyUnavailable()
		case <-ctx.Done():
			return
		}
		current.Close()
		if relay.IsClosed() {
			return
		}
		logger.Debug("wbstream: creator session ended, rejoining")

		var newTun tunnel.DataTunnel
		attempt := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(common.BackoffWithJitter(attempt, 3*time.Second, time.Minute)):
			}
			attempt++
			if relay.IsClosed() {
				return
			}
			cookieHeader, bearer, currentGeneration, err := owner.Credentials(ctx, httpClient, 0)
			if err != nil {
				logger.Warn("wbstream: owner session unavailable, retrying")
				continue
			}
			generation = currentGeneration
			_, roomToken, accessToken, serverURL, err := AuthAsLoggedIn(httpClient, cookieHeader, bearer, roomID, "Creator")
			if err != nil {
				logger.Warn("wbstream: rejoin auth failed, refreshing session")
				cookieHeader, bearer, generation, err = owner.Credentials(ctx, httpClient, generation)
				if err != nil {
					logger.Warn("wbstream: rejoin token refresh failed, retrying")
					continue
				}
				_, roomToken, accessToken, serverURL, err = AuthAsLoggedIn(httpClient, cookieHeader, bearer, roomID, "Creator")
				if err != nil {
					logger.Warn("wbstream: rejoin auth failed, retrying")
					continue
				}
			}
			newSess, tunCh := joinSession(roomToken, accessToken, serverURL)
			if err := newSess.Start(); err != nil {
				newSess.Close()
				logger.Warn("wbstream: rejoin session start failed, retrying")
				continue
			}
			select {
			case newTun = <-tunCh:
			case <-ctx.Done():
				newSess.Close()
				return
			case <-time.After(60 * time.Second):
				logger.Warn("wbstream: rejoin tunnel timed out, retrying")
				newSess.Close()
				continue
			}
			current = newSess
			break
		}
		relay.SwapTunnel(newTun)
		logger.Info(fmt.Sprintf("wbstream: creator tunnel reconnected, buf=%d", bridgeReadBufFor(newTun, readBuf)))
	}
}

func bridgeReadBufFor(tun tunnel.DataTunnel, readBuf int) int {
	switch tun.(type) {
	case *rtc.DCTunnel, *rtc.MultiTrackKCPTunnel:
		return readBuf
	}
	return common.VP8BufSize
}
