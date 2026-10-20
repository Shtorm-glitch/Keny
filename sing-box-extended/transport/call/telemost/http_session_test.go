package telemost

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"
)

type httpTestDialer struct{ net.Dialer }

func (d *httpTestDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.Dialer.DialContext(ctx, network, destination.String())
}
func (d *httpTestDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

type apiRedirect struct {
	base     http.RoundTripper
	endpoint string
}

func (r apiRedirect) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	target, err := url.Parse(r.endpoint)
	if err != nil {
		return nil, err
	}
	target.Path = req.URL.Path
	copy.URL = target
	copy.Host = target.Host
	return r.base.RoundTrip(copy)
}

var localTrustOnce sync.Once

func localHTTPServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, func() (int, int)) {
	t.Helper()
	var mu sync.Mutex
	accepted := 0
	live := make(map[net.Conn]bool)
	s := httptest.NewUnstartedServer(handler)
	s.EnableHTTP2 = true
	s.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		mu.Lock()
		defer mu.Unlock()
		if state == http.StateNew {
			accepted++
			live[conn] = true
		}
		if state == http.StateClosed {
			delete(live, conn)
		}
	}
	s.StartTLS()
	t.Setenv("GODEBUG", os.Getenv("GODEBUG")+",x509usefallbackroots=1")
	localTrustOnce.Do(func() {
		pool := x509.NewCertPool()
		pool.AddCert(s.Certificate())
		x509.SetFallbackRoots(pool)
	})
	t.Cleanup(s.Close)
	return s, func() (int, int) { mu.Lock(); defer mu.Unlock(); return accepted, len(live) }
}

func awaitLive(t *testing.T, count func() (int, int), want int) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		_, live := count()
		if live == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, live := count()
	t.Fatalf("live sockets=%d, want=%d", live, want)
}

func TestCreatorHTTPReuseAndSessionIsolation(t *testing.T) {
	server, count := localHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Error("expected HTTP/2")
		}
		w.Write([]byte("{}"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := newCreatorHTTPDialer(ctx, &httpTestDialer{})
	defer owner.client.CloseIdleConnections()
	owner.client.Transport = apiRedirect{owner.client.Transport, server.URL}
	request := func(d *creatorHTTPDialer) error {
		c, release := creatorAPI(d, "", TMConfig{})
		defer release()
		_, status, err := c.Do("GET", "/waiting-rooms/peers", nil)
		if err == nil && status != 200 {
			return errors.New("bad HTTP status")
		}
		return err
	}
	for i := 0; i < 100; i++ {
		if err := request(owner); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := request(owner); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	accepted, _ := count()
	if accepted != 1 {
		t.Fatalf("120 requests opened %d sockets", accepted)
	}
	if owner.client.Timeout != apiRequestTimeout {
		t.Fatal("missing request timeout")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	other := newCreatorHTTPDialer(ctx2, &httpTestDialer{})
	defer other.client.CloseIdleConnections()
	other.client.Transport = apiRedirect{other.client.Transport, server.URL}
	if err := request(other); err != nil {
		t.Fatal(err)
	}
	accepted, _ = count()
	if accepted != 2 {
		t.Fatalf("sessions must own separate pools: %d", accepted)
	}
	cancel()
	awaitLive(t, count, 1)
	if err := request(owner); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed session error=%v", err)
	}
	if err := request(other); err != nil {
		t.Fatal("other session affected:", err)
	}
	cancel2()
	awaitLive(t, count, 0)
}

func TestCreatorHTTPInFlightCancellationAndTimeout(t *testing.T) {
	started := make(chan struct{}, 2)
	server, count := localHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := newCreatorHTTPDialer(ctx, &httpTestDialer{})
	defer owner.client.CloseIdleConnections()
	owner.client.Transport = apiRedirect{owner.client.Transport, server.URL}
	c, release := creatorAPI(owner, "", TMConfig{})
	defer release()
	done := make(chan error, 1)
	go func() { _, _, err := c.Do("GET", "/slow", nil); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request not started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
	awaitLive(t, count, 0)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	other := newCreatorHTTPDialer(ctx2, &httpTestDialer{})
	defer other.client.CloseIdleConnections()
	other.client.Timeout = 50 * time.Millisecond
	other.client.Transport = apiRedirect{other.client.Transport, server.URL}
	c2, release2 := creatorAPI(other, "", TMConfig{})
	defer release2()
	_, _, err := c2.Do("GET", "/slow", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v", err)
	}
}
