package call

import (
	"context"
	"github.com/sagernet/sing/common/logger"
	"path/filepath"
	"testing"
	"time"
)

func TestTelemostRoomsCanonicalAndSafe(t *testing.T) {
	a, err := rotationRoomID("telemost", "https://telemost.yandex.ru/j/1001?source=test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := rotationRoomID("telemost", "https://telemost.yandex.com/j/1001/")
	if err != nil || a != b {
		t.Fatal("room aliases not recognized")
	}
	for _, link := range []string{"http://telemost.yandex.ru/j/1001", "https://elsewhere.test/j/1001", "https://telemost.yandex.ru/1001", "https://user:secret@telemost.yandex.ru/j/1001"} {
		if _, err := rotationRoomID("telemost", link); err == nil {
			t.Fatal("unsafe room accepted")
		}
	}
}

func TestTelemostTwoRoomPlannedRotationWithoutWBOwner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	broker := &roomBroker{owners: make(map[string]*brokerTunnel), joiners: make(map[string]*brokerTunnel), secondRotation: make(chan struct{}), t: t}
	rooms := []RotationRoom{{Name: "Interview1", JoinLink: "https://telemost.yandex.ru/j/1001"}, {Name: "Interview2", JoinLink: "https://telemost.yandex.ru/j/1002"}}
	enabled := true
	base := Config{Platform: "telemost", Logger: logger.NOP(), PSK: []byte("01234567890123456789012345678901"), connectCarrier: broker.connect}
	owner := base
	owner.Role = RoleCreator
	owner.Rotation = &RotationConfig{Rooms: rooms, StateFile: filepath.Join(t.TempDir(), "owner.json"), Scheduled: &enabled, IdleTimeout: time.Millisecond, interval: func() time.Duration { return 20 * time.Millisecond }, switchDelay: 3 * time.Millisecond}
	joiner := base
	joiner.Role = RoleJoiner
	joiner.Rotation = &RotationConfig{Rooms: rooms}
	server, err := Connect(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := Connect(ctx, joiner)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-broker.secondRotation:
	case <-ctx.Done():
		t.Fatal("Telemost did not complete two room transitions")
	}
}

func TestTelemostRotationRejectsIncompatibleModes(t *testing.T) {
	disabled := false
	base := Config{Platform: "telemost", Logger: logger.NOP(), PSK: []byte("01234567890123456789012345678901"), Rotation: &RotationConfig{Rooms: []RotationRoom{{Name: "A", JoinLink: "https://telemost.yandex.ru/j/1001"}, {Name: "B", JoinLink: "https://telemost.yandex.ru/j/1002"}}}}
	for _, c := range []Config{func() Config { c := base; c.Reliable = &disabled; return c }(), func() Config { c := base; c.DualTrack = true; return c }()} {
		if _, err := connectRotating(context.Background(), c); err == nil {
			t.Fatal("incompatible carrier accepted")
		}
	}
}
