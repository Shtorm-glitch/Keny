package wbstream

import (
	"context"
	"errors"
	"github.com/sagernet/sing-box/transport/call/common"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestOwnerSessionConcurrentStaleFailure(t *testing.T) {
	s, err := NewOwnerSession("__wb_device_id=test-device; wbx-refresh=old", "")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s.refresh = func(_ *http.Client, cookies, device string) (string, map[string]string, error) {
		calls.Add(1)
		return "test-bearer", map[string]string{"wbx-refresh": "rotated"}, nil
	}
	_, _, generation, err := s.Credentials(context.Background(), &http.Client{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, _, _, err := s.Credentials(context.Background(), &http.Client{}, generation); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 2 {
		t.Fatalf("refresh calls=%d, want 2", calls.Load())
	}
}

func TestOwnerSessionRetainsCookieWhenConfirmFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s, _ := NewOwnerSession("__wb_device_id=test-device; wbx-refresh=old", path)
	s.refresh = func(_ *http.Client, _, _ string) (string, map[string]string, error) {
		return "", map[string]string{"wbx-refresh": "rotated"}, errors.New("confirm failed")
	}
	if _, _, _, err := s.Credentials(context.Background(), &http.Client{}, 0); err == nil {
		t.Fatal("expected failed confirmation")
	}
	restored, err := NewOwnerSession("", path)
	if err != nil {
		t.Fatal(err)
	}
	if common.CookieValue(restored.cookies, "wbx-refresh") != "rotated" {
		t.Fatal("lost already rotated cookie")
	}
}

func TestOwnerSessionRestartUsesRotatedCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s, _ := NewOwnerSession("__wb_device_id=test-device; wbx-refresh=old", path)
	s.refresh = func(_ *http.Client, _, _ string) (string, map[string]string, error) {
		return "bearer", map[string]string{"wbx-refresh": "new"}, nil
	}
	if _, _, _, err := s.Credentials(context.Background(), &http.Client{}, 0); err != nil {
		t.Fatal(err)
	}
	restored, err := NewOwnerSession("", path)
	if err != nil {
		t.Fatal(err)
	}
	if restored.cookies != "wbx-refresh=new" {
		t.Fatalf("unexpected restored cookie names or values")
	}
}

func TestOwnerSessionPersistenceFailureDoesNotRepeatRefresh(t *testing.T) {
	s, _ := NewOwnerSession("__wb_device_id=test-device; wbx-refresh=old", filepath.Join(t.TempDir(), "missing", "session.json"))
	var calls int
	s.refresh = func(_ *http.Client, _, _ string) (string, map[string]string, error) {
		calls++
		return "bearer", map[string]string{"wbx-refresh": "new"}, nil
	}
	for range 2 {
		if _, _, _, err := s.Credentials(context.Background(), &http.Client{}, 0); err == nil {
			t.Fatal("expected persistence failure")
		}
	}
	if calls != 1 {
		t.Fatalf("replayed refresh after persistence failure: %d", calls)
	}
}
