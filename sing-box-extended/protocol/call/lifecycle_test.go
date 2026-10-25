package call

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
)

func TestCallScopeCloseCancelsPendingStartup(t *testing.T) {
	for _, inbound := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		scope := adapter.NewScope(context.Background(), nil)
		if inbound {
			h := &Inbound{ctx: ctx, cancel: cancel}
			scope.Add(h.Close)
		} else {
			h := &Outbound{ctx: ctx, cancel: cancel}
			scope.Add(h.Close)
		}
		if err := scope.Close(); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != context.Canceled {
			t.Fatal("Call startup context remained live after scope cleanup")
		}
	}
}
