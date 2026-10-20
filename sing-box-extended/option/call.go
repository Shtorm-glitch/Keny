package option

import (
	"github.com/sagernet/sing/common/json/badoption"
	"strings"
)

// CallCookie is a single cookie entry provided inline in the configuration as
// JSON, matching the browser-exported {name, value} format.
type CallCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CallCookieList is the list of cookies provided in the configuration.
type CallCookieList []CallCookie

// Header renders the cookie list into a single "name=value; name=value"
// header string. Entries with an empty name are skipped.
func (l CallCookieList) Header() string {
	if len(l) == 0 {
		return ""
	}
	parts := make([]string, 0, len(l))
	for _, c := range l {
		if c.Name == "" {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// ParseCookieHeader parses a "name=value; name=value" cookie header string into
// a CallCookieList.
func ParseCookieHeader(header string) CallCookieList {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}
	var list CallCookieList
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		list = append(list, CallCookie{Name: name, Value: strings.TrimSpace(value)})
	}
	return list
}

type CallCommonOptions struct {
	VP8FPS    int                  `json:"vp8_fps,omitempty"`
	VP8Batch  int                  `json:"vp8_batch,omitempty"`
	DualTrack bool                 `json:"dual_track,omitempty"`
	Reliable  *bool                `json:"reliable,omitempty"`
	Rotation  *CallRotationOptions `json:"rotation,omitempty"`
	// PSKFile may reference a systemd credential. PSK is for manually agreed client profiles.
	PSK               string `json:"psk,omitempty"`
	PSKFile           string `json:"psk_file,omitempty"`
	Platform          string `json:"platform,omitempty"`
	Mode              string `json:"mode,omitempty"`
	ReadBuffer        int    `json:"read_buffer,omitempty"`
	MaxBufferedAmount int    `json:"max_buffered_amount,omitempty"`
	MemoryLimit       int64  `json:"memory_limit,omitempty"`
}

type CallRotationOptions struct {
	Scheduled   *bool              `json:"scheduled,omitempty"`
	IntervalMin badoption.Duration `json:"interval_min,omitempty"`
	IntervalMax badoption.Duration `json:"interval_max,omitempty"`
	IdleTimeout badoption.Duration `json:"idle_timeout,omitempty"`
	MaxDefer    badoption.Duration `json:"max_defer,omitempty"`
	Rooms       []CallRotationRoom `json:"rooms"`
	StateFile   string             `json:"state_file,omitempty"`
}
type CallRotationRoom struct {
	Name     string `json:"name"`
	JoinLink string `json:"join_link"`
}

type CallInboundOptions struct {
	DialerOptions
	CallCommonOptions
	Cookies          CallCookieList `json:"cookies,omitempty"`
	JoinLink         string         `json:"join_link,omitempty"`
	SessionStateFile string         `json:"session_state_file,omitempty"`
	// Email and Password are used to re-authenticate with the dion.vc
	// platform when the refresh cookie is missing or rejected.
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

type CallOutboundOptions struct {
	DialerOptions
	CallCommonOptions
	JoinLink string         `json:"join_link"`
	Cookies  CallCookieList `json:"cookies,omitempty"`
}
