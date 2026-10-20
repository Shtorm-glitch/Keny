package vk

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
)

type captchaTestPlatform struct {
	adapter.PlatformInterface
	notice    *adapter.Notification
	cancelled bool
}

func (p *captchaTestPlatform) UsePlatformNotification() bool { return true }
func (p *captchaTestPlatform) SendNotification(n *adapter.Notification) error {
	p.notice = n
	return nil
}
func (p *captchaTestPlatform) CancelNotification(identifier string, id int32) error {
	p.cancelled = p.notice != nil && identifier == p.notice.Identifier && id == p.notice.TypeID
	return nil
}

func TestCaptchaNotificationCancelledWithConnection(t *testing.T) {
	StopCaptchaProxy()
	ctx, cancel := context.WithCancel(context.Background())
	p := &captchaTestPlatform{}
	ctx = service.ContextWith[adapter.PlatformInterface](ctx, p)
	activeCaptchaProxy.Lock()
	activeCaptchaProxy.keyCh = make(chan string, 1)
	activeCaptchaProxy.doneCh = make(chan struct{})
	activeCaptchaProxy.Unlock()
	defer StopCaptchaProxy()
	cancel()
	if result := waitVKCaptcha(ctx, 12345, logger.NOP()); result != "" {
		t.Fatal("cancelled challenge returned a token")
	}
	if p.notice == nil || p.notice.OpenURL != "http://127.0.0.1:12345/" || !p.cancelled {
		t.Fatal("challenge notification was not delivered and cancelled")
	}
}

func TestCaptchaNotificationCancelledAfterSolution(t *testing.T) {
	StopCaptchaProxy()
	p := &captchaTestPlatform{}
	ctx := service.ContextWith[adapter.PlatformInterface](context.Background(), p)
	activeCaptchaProxy.Lock()
	activeCaptchaProxy.keyCh = make(chan string, 1)
	activeCaptchaProxy.keyCh <- "test-result"
	activeCaptchaProxy.doneCh = make(chan struct{})
	activeCaptchaProxy.Unlock()
	defer StopCaptchaProxy()
	if result := waitVKCaptcha(ctx, 12345, logger.NOP()); result != "test-result" || !p.cancelled {
		t.Fatal("solution did not complete the notification lifecycle")
	}
}
