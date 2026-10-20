package telemost

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/transport/call/common"
	N "github.com/sagernet/sing/common/network"
)

const apiRequestTimeout = 30 * time.Second

// A creator owns its transport: never share it across routing/session contexts.
type creatorHTTPDialer struct {
	carrierDialer
	client *http.Client
}

func newCreatorHTTPDialer(ctx context.Context, dialer N.Dialer) *creatorHTTPDialer {
	d := &creatorHTTPDialer{carrierDialer: carrierDialer{Dialer: dialer, ctx: ctx}}
	d.client = common.HttpClient(d.carrierDialer)
	d.client.Timeout = apiRequestTimeout
	return d
}

func creatorHTTP(dialer N.Dialer) (*http.Client, context.Context, func()) {
	if owner, ok := dialer.(*creatorHTTPDialer); ok {
		return owner.client, owner.ctx, func() {}
	}
	// Standalone helpers still own and release their one-use transport.
	client := common.HttpClient(dialer)
	client.Timeout = apiRequestTimeout
	return client, context.Background(), client.CloseIdleConnections
}

func creatorAPI(dialer N.Dialer, cookie string, cfg TMConfig) (Client, func()) {
	client, ctx, release := creatorHTTP(dialer)
	return Client{HTTP: client, Context: ctx, Cookie: cookie,
		AppVersion: cfg.AppVersion, InstanceID: clientInstanceID}, release
}

func creatorGET(dialer N.Dialer, endpoint string) ([]byte, error) {
	client, ctx, release := creatorHTTP(dialer)
	defer release()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", common.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
