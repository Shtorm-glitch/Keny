package call

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/call"
)

func readPSK(options option.CallCommonOptions) ([]byte, error) {
	if options.PSK != "" && options.PSKFile != "" {
		return nil, errors.New("call: psk and psk_file are mutually exclusive")
	}
	secret := options.PSK
	if options.PSKFile != "" {
		data, err := os.ReadFile(options.PSKFile)
		if err != nil {
			return nil, errors.New("call: cannot read PSK credential")
		}
		secret = strings.TrimSpace(string(data))
	}
	if secret != "" && len(secret) < 32 {
		return nil, errors.New("call: PSK must contain at least 32 bytes")
	}
	if options.PSKFile != "" && secret == "" {
		return nil, errors.New("call: empty PSK credential")
	}
	return []byte(secret), nil
}

func rotationConfig(options option.CallCommonOptions) *call.RotationConfig {
	if options.Rotation == nil {
		return nil
	}
	config := &call.RotationConfig{StateFile: options.Rotation.StateFile, Scheduled: options.Rotation.Scheduled,
		IntervalMin: time.Duration(options.Rotation.IntervalMin), IntervalMax: time.Duration(options.Rotation.IntervalMax),
		IdleTimeout: time.Duration(options.Rotation.IdleTimeout), MaxDefer: time.Duration(options.Rotation.MaxDefer)}
	for _, room := range options.Rotation.Rooms {
		config.Rooms = append(config.Rooms, call.RotationRoom{Name: room.Name, JoinLink: room.JoinLink})
	}
	return config
}
