package call

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/transport/call/common"
	"github.com/sagernet/sing-box/transport/call/wbstream"
)

type RotationRoom struct {
	Name     string `json:"name"`
	JoinLink string `json:"join_link"`
}
type RotationConfig struct {
	Scheduled                                       *bool
	IntervalMin, IntervalMax, IdleTimeout, MaxDefer time.Duration
	Rooms                                           []RotationRoom `json:"rooms"`
	StateFile                                       string         `json:"state_file,omitempty"`
	interval                                        func() time.Duration
	switchDelay                                     time.Duration
}

type rotationAnnouncement struct {
	Version  int       `json:"version"`
	Epoch    uint64    `json:"epoch"`
	Active   int       `json:"active"`
	Next     int       `json:"next"`
	SwitchAt time.Time `json:"switch_at"`
	MAC      string    `json:"mac"`
}

func (s rotationAnnouncement) signedBytes() []byte {
	s.MAC = ""
	data, _ := json.Marshal(s)
	return data
}
func (s *rotationAnnouncement) sign(key []byte) {
	mac := hmac.New(sha256.New, key)
	mac.Write(s.signedBytes())
	s.MAC = hex.EncodeToString(mac.Sum(nil))
}
func (s rotationAnnouncement) valid(key []byte, epoch uint64, now time.Time) bool {
	if s.Version != 1 || s.Epoch <= epoch || s.Active < 0 || s.Active > 1 || s.Next != 1-s.Active || s.SwitchAt.Before(now.Add(-15*time.Second)) || s.SwitchAt.After(now.Add(15*time.Second)) {
		return false
	}
	supplied, err := hex.DecodeString(s.MAC)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(s.signedBytes())
	return hmac.Equal(supplied, mac.Sum(nil))
}

type rotationSlot struct {
	bridge *Bridge
	cancel context.CancelFunc
	index  int
}

func (s *rotationSlot) close() {
	if s != nil {
		s.cancel()
		s.bridge.Close()
	}
}

type rotationEvent struct {
	slot  *rotationSlot
	state *rotationAnnouncement
	ready bool
}

func connectRotating(ctx context.Context, cfg Config) (*Bridge, error) {
	if (cfg.Platform != "wbstream" && cfg.Platform != "telemost") || len(cfg.Rotation.Rooms) != 2 || len(cfg.PSK) < 32 {
		return nil, errors.New("call: rotation requires wbstream or telemost, two rooms and independent PSK")
	}
	if err := cfg.Rotation.validate(); err != nil {
		return nil, err
	}
	if cfg.Platform == "telemost" && ((cfg.Reliable != nil && !*cfg.Reliable) || cfg.DualTrack) {
		return nil, errors.New("call: telemost rotation requires reliable single-track mode")
	}
	rooms := cfg.Rotation.Rooms
	first, firstErr := rotationRoomID(cfg.Platform, rooms[0].JoinLink)
	second, secondErr := rotationRoomID(cfg.Platform, rooms[1].JoinLink)
	if firstErr != nil || secondErr != nil || rooms[0].Name == "" || rooms[1].Name == "" || rooms[0].Name == rooms[1].Name || first == second {
		return nil, errors.New("call: rotation rooms must have distinct names and links")
	}
	if cfg.Platform == "wbstream" && cfg.Role == RoleCreator && cfg.OwnerSession == nil {
		return nil, errors.New("call: rotation requires shared owner session")
	}
	if cfg.Logger == nil {
		return nil, errors.New("call: rotation requires logger")
	}
	ctx, cancel := context.WithCancel(ctx)
	m := &managedBridge{ctx: ctx, cancel: cancel, changed: make(chan struct{}), done: make(chan struct{})}
	go runRotation(m, cfg)
	return &Bridge{managed: m}, nil
}

func runRotation(m *managedBridge, cfg Config) {
	defer close(m.done)
	ctx := m.ctx
	scheduled := cfg.Platform == "wbstream"
	if cfg.Rotation.Scheduled != nil {
		scheduled = *cfg.Rotation.Scheduled
	}
	interval := cfg.Rotation.interval
	if interval == nil {
		minimum, maximum := cfg.Rotation.IntervalMin, cfg.Rotation.IntervalMax
		if minimum == 0 {
			minimum = 2 * time.Hour
		}
		if maximum == 0 {
			maximum = 4 * time.Hour
		}
		interval = func() time.Duration {
			if minimum == maximum {
				return minimum
			}
			return minimum + time.Duration(rand.Int64N(int64(maximum-minimum)))
		}
	}
	switchDelay := cfg.Rotation.switchDelay
	if switchDelay == 0 {
		switchDelay = 3 * time.Second
	}
	events := make(chan rotationEvent, 16)
	index, epoch := 0, uint64(0)
	if cfg.Rotation.StateFile != "" {
		data, err := os.ReadFile(cfg.Rotation.StateFile)
		if err == nil {
			var saved struct {
				Index int
				Epoch uint64
			}
			if json.Unmarshal(data, &saved) != nil || saved.Index < 0 || saved.Index > 1 {
				cfg.Logger.Error("call: invalid rotation state")
				m.cancel()
				return
			}
			index, epoch = saved.Index, saved.Epoch
		} else if !errors.Is(err, os.ErrNotExist) {
			cfg.Logger.Error("call: cannot read rotation state")
			m.cancel()
			return
		}
	}
	var active *rotationSlot
	defer func() { active.close(); m.activate(nil) }()
	connect := func(index int) (*rotationSlot, error) {
		childCtx, cancel := context.WithCancel(ctx)
		childCfg := cfg
		childCfg.Rotation = nil
		childCfg.Rotating = true
		childCfg.JoinLink = cfg.Rotation.Rooms[index].JoinLink
		// Joining a room with no owner must not block alternate-room recovery.
		timeout := time.AfterFunc(90*time.Second, cancel)
		if cfg.Role == RoleJoiner || cfg.Platform == "telemost" {
			timeout.Stop()
			timeout = time.AfterFunc(25*time.Second, cancel)
		}
		factory := cfg.connectCarrier
		if factory == nil {
			factory = Connect
		}
		child, err := factory(childCtx, childCfg)
		timeout.Stop()
		if err != nil {
			cancel()
			return nil, err
		}
		if childCtx.Err() != nil {
			cancel()
			child.Close()
			return nil, childCtx.Err()
		}
		slot := &rotationSlot{bridge: child, cancel: cancel, index: index}
		child.relay.SetRotationHandler(func(data []byte) {
			if cfg.Role != RoleJoiner || len(data) > 1024 {
				return
			}
			var state rotationAnnouncement
			if json.Unmarshal(data, &state) == nil {
				select {
				case events <- rotationEvent{slot: slot, state: &state}:
				default:
				}
			}
		}, func() {
			select {
			case events <- rotationEvent{slot: slot}:
			default:
			}
		})
		child.relay.SetAvailableHandler(func() {
			select {
			case events <- rotationEvent{slot: slot, ready: true}:
			default:
			}
		})
		return slot, nil
	}
	persist := func(next int, epoch uint64) bool {
		if cfg.Rotation.StateFile == "" {
			return true
		}
		data, _ := json.Marshal(struct {
			Index int
			Epoch uint64
		}{next, epoch})
		if wbstream.WritePrivateState(cfg.Rotation.StateFile, data) != nil {
			cfg.Logger.Error("call: rotation persistence failed")
			return false
		}
		return true
	}
	wait := func(delay time.Duration) bool {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		}
	}
	for attempt := 0; active == nil; attempt++ {
		if ctx.Err() != nil {
			return
		}
		child, err := connect(index)
		if err == nil {
			if !persist(index, epoch) {
				child.close()
				if !wait(5 * time.Second) {
					return
				}
				continue
			}
			active = child
			m.activate(child.bridge)
			break
		}
		cfg.Logger.Warn("call: initial carrier unavailable, retrying")
		if cfg.Role == RoleJoiner || cfg.Platform == "telemost" {
			index = 1 - index
		}
		if !wait(common.BackoffWithJitter(attempt, time.Second, time.Minute)) {
			return
		}
	}
	rotationTimer := time.NewTimer(interval())
	defer rotationTimer.Stop()
	var timerC <-chan time.Time
	healthTicker := time.NewTicker(10 * time.Second)
	defer healthTicker.Stop()
	if cfg.Role == RoleCreator && scheduled {
		timerC = rotationTimer.C
	}
	var deferredAt time.Time
	for {
		var next int
		var nextEpoch uint64
		var unavailable bool
		select {
		case <-ctx.Done():
			return
		case now := <-healthTicker.C:
			active.bridge.relay.ProbePeer()
			if (cfg.Role == RoleJoiner || cfg.Platform == "wbstream") && !active.bridge.relay.PeerAlive(now) {
				active.bridge.relay.NotifyUnavailable()
			}
			continue
		case <-timerC:
			if cfg.Platform == "telemost" {
				idle, maximum := cfg.Rotation.IdleTimeout, cfg.Rotation.MaxDefer
				if idle == 0 {
					idle = 30 * time.Second
				}
				if maximum == 0 {
					maximum = 10 * time.Minute
				}
				if !active.bridge.relay.IdleFor(time.Now(), idle) {
					if deferredAt.IsZero() {
						deferredAt = time.Now()
					}
					if time.Since(deferredAt) >= maximum {
						deferredAt = time.Time{}
						rotationTimer.Reset(interval())
					} else {
						rotationTimer.Reset(idle)
					}
					continue
				}
				deferredAt = time.Time{}
			}
			next, nextEpoch = 1-index, epoch+1
			state := rotationAnnouncement{Version: 1, Epoch: nextEpoch, Active: index, Next: next, SwitchAt: time.Now().Add(switchDelay)}
			state.sign(cfg.PSK)
			data, _ := json.Marshal(state)
			for range 3 {
				active.bridge.relay.SendRotation(data)
				if !wait(switchDelay / 3) {
					return
				}
			}
		case event := <-events:
			if event.slot != active {
				continue
			}
			if event.ready {
				if cfg.Role == RoleCreator && scheduled {
					rotationTimer.Reset(interval())
					timerC = rotationTimer.C
				}
				continue
			}
			if event.state != nil {
				if !event.state.valid(cfg.PSK, epoch, time.Now()) || event.state.Active != index {
					continue
				}
				next, nextEpoch = event.state.Next, event.state.Epoch
				// Prepare the next joiner immediately; old carrier remains alive until ready.
			} else {
				if cfg.Role == RoleCreator && cfg.Platform == "wbstream" {
					rotationTimer.Stop()
					timerC = nil
					continue
				} // WB creator's reconnect supervisor owns recovery.
				m.activate(nil)
				// An unavailable Telemost joiner must leave before searching.
				// Otherwise fallback to its room creates two peer epochs and
				// repeatedly resets the owner's KCP stream.
				unavailable = true
				next, nextEpoch = 1-index, epoch
			}
		}
		if !persist(next, nextEpoch) {
			if !wait(5 * time.Second) {
				return
			}
			if cfg.Role == RoleCreator && scheduled {
				rotationTimer.Reset(5 * time.Second)
				timerC = rotationTimer.C
			}
			continue
		}
		if cfg.Role == RoleCreator || (unavailable && cfg.Platform == "telemost" && cfg.Role == RoleJoiner) {
			m.activate(nil)
			active.close()
			active = nil
		}
		for attempt := 0; ; attempt++ {
			if ctx.Err() != nil {
				return
			}
			// Planned handover may retain the old room for a short grace
			// period, but a failed handover must not rejoin that same room
			// while its old joiner is still publishing.
			if cfg.Platform == "telemost" && cfg.Role == RoleJoiner && active != nil && next == active.index {
				m.activate(nil)
				active.close()
				active = nil
			}
			child, err := connect(next)
			if err == nil {
				if !persist(next, nextEpoch) {
					child.close()
					if !wait(5 * time.Second) {
						return
					}
					continue
				}
				old := active
				active = child
				index, epoch = next, nextEpoch
				m.activate(child.bridge)
				// The old client carrier can finish its outstanding streams for 15 seconds.
				if old != nil {
					go func() { defer old.close(); wait(15 * time.Second) }()
				}
				if cfg.Role == RoleCreator && scheduled {
					rotationTimer.Reset(interval())
					timerC = rotationTimer.C
				}
				cfg.Logger.Info("call: rotation carrier ready")
				break
			}
			cfg.Logger.Warn("call: next carrier unavailable, retrying")
			if cfg.Role == RoleJoiner || cfg.Platform == "telemost" {
				next = 1 - next
			}
			if !wait(common.BackoffWithJitter(attempt, 5*time.Second, time.Minute)) {
				return
			}
		}
	}
}

func rotationRoomID(platform, link string) (string, error) {
	if platform == "wbstream" {
		if strings.TrimSpace(link) == "" {
			return "", errors.New("empty room")
		}
		return wbstream.ParseRoomID(link), nil
	}
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return "", errors.New("invalid telemost room link")
	}
	switch strings.ToLower(u.Hostname()) {
	case "telemost.yandex.ru", "telemost.yandex.com", "telemost.360.yandex.ru":
	default:
		return "", errors.New("invalid telemost room host")
	}
	id := strings.TrimPrefix(strings.TrimSuffix(u.Path, "/"), "/j/")
	if id == "" || strings.Contains(id, "/") {
		return "", errors.New("invalid telemost room path")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", errors.New("invalid telemost room id")
		}
	}
	return id, nil
}

func (r *RotationConfig) validate() error {
	min, max := r.IntervalMin, r.IntervalMax
	if min == 0 {
		min = 2 * time.Hour
	}
	if max == 0 {
		max = 4 * time.Hour
	}
	if min < 0 || max < min || r.IdleTimeout < 0 || r.MaxDefer < 0 {
		return errors.New("call: invalid rotation timing")
	}
	return nil
}
