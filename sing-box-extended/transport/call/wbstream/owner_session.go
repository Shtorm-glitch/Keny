package wbstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sagernet/sing-box/transport/call/common"
)

// OwnerSession belongs to an account, not to a room or a carrier generation.
// A stale authentication failure cannot rotate a newer refresh cookie again.
type OwnerSession struct {
	mu                              sync.Mutex
	cookies, bearer, deviceID, path string
	generation                      uint64
	persistPending                  bool
	refresh                         func(*http.Client, string, string) (string, map[string]string, error)
}

func NewOwnerSession(cookies, path string) (*OwnerSession, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			var saved struct {
				Cookies string `json:"cookies"`
			}
			if json.Unmarshal(data, &saved) != nil {
				return nil, errors.New("wbstream: invalid session state")
			}
			cookies = saved.Cookies
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("wbstream: cannot read session state")
		}
	}
	device := common.CookieValue(cookies, "__wb_device_id")
	if device == "" {
		return nil, errors.New("wbstream: session missing device cookie")
	}
	return &OwnerSession{cookies: common.FilterCookies(cookies, WBStreamCookieAllowlist), deviceID: device, path: path, refresh: RefreshAccessToken}, nil
}

// Credentials refreshes only once for concurrent failures of the same generation.
// Refresh results remain in memory if disk persistence fails: replaying the old
// refresh cookie could invalidate the newly issued session.
func (s *OwnerSession) Credentials(ctx context.Context, client *http.Client, failedGeneration uint64) (string, string, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", "", 0, err
	}
	if s.bearer == "" || (failedGeneration != 0 && failedGeneration == s.generation) {
		bounded := *client
		bounded.Timeout = 30 * time.Second
		bounded.Transport = contextTransport{ctx: ctx, base: client.Transport}
		bearer, updates, err := s.refresh(&bounded, s.cookies, s.deviceID)
		if err != nil {
			// The slide may already have rotated its one-use cookie before confirm
			// failed. Preserve it instead of replaying the previous cookie.
			if len(updates) > 0 {
				s.cookies = common.FilterCookies(mergeCookies(s.cookies, updates), WBStreamCookieAllowlist)
				if s.path != "" {
					data, _ := json.Marshal(struct {
						Cookies string `json:"cookies"`
					}{mergeCookies(s.cookies, map[string]string{"__wb_device_id": s.deviceID})})
					s.persistPending = atomicPrivateWrite(s.path, data) != nil
				}
			}
			return "", "", 0, errors.New("wbstream: session refresh failed")
		}
		s.cookies = common.FilterCookies(mergeCookies(s.cookies, updates), WBStreamCookieAllowlist)
		s.bearer = bearer
		s.generation++
		s.persistPending = s.path != ""
	}
	if s.persistPending {
		data, _ := json.Marshal(struct {
			Cookies string `json:"cookies"`
		}{mergeCookies(s.cookies, map[string]string{"__wb_device_id": s.deviceID})})
		if err := atomicPrivateWrite(s.path, data); err != nil {
			return "", "", 0, errors.New("wbstream: session persistence failed")
		}
		s.persistPending = false
	}
	return s.cookies, s.bearer, s.generation, nil
}

type contextTransport struct {
	ctx  context.Context
	base http.RoundTripper
}

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req.Clone(t.ctx))
}

func atomicPrivateWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".keny-session-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

// WritePrivateState persists non-cookie controller state with the same atomic discipline.
func WritePrivateState(path string, data []byte) error { return atomicPrivateWrite(path, data) }
