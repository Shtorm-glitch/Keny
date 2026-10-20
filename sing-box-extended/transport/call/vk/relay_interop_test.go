package vk

import (
	"testing"
	"time"

	headless "github.com/kulikov0/headless-client"
	"github.com/kulikov0/headless-client/webrtc"
	"github.com/sagernet/sing-box/transport/call/headlessapi"
	"github.com/sagernet/sing/common/logger"
)

// Exercise the real creator factory against the profile used by VKJoiner.
// A successful SDP exchange alone does not establish DTLS/SCTP compatibility.
func TestCreatorInteroperatesWithJoinerProfile(t *testing.T) {
	creator := NewTunnelRelay(nil, logger.NOP())
	if err := creator.Init(nil); err != nil {
		t.Fatal(err)
	}
	defer creator.Close()
	api, err := headlessapi.WebRTCAPI(headlessapi.Options{Profile: headless.ChromeWindows.WithDTLS13Mimicry()})
	if err != nil {
		t.Fatal(err)
	}
	joiner, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer joiner.Close()
	delivered := make(chan struct{}, 1)
	joiner.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "interop-probe" {
			dc.OnMessage(func(m webrtc.DataChannelMessage) {
				if string(m.Data) == "creator-to-joiner" {
					select {
					case delivered <- struct{}{}:
					default:
					}
				}
			})
		}
	})
	dc, err := creator.pc.CreateDataChannel("interop-probe", nil)
	if err != nil {
		t.Fatal(err)
	}
	dc.OnOpen(func() { _ = dc.SendText("creator-to-joiner") })
	gather := webrtc.GatheringCompletePromise(creator.pc)
	if _, err = creator.CreateOffer(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gather:
	case <-time.After(5 * time.Second):
		t.Fatal("creator candidate gathering timed out")
	}
	if err = joiner.SetRemoteDescription(*creator.pc.LocalDescription()); err != nil {
		t.Fatal(err)
	}
	answer, err := joiner.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gather = webrtc.GatheringCompletePromise(joiner)
	if err = joiner.SetLocalDescription(answer); err != nil {
		t.Fatal(err)
	}
	select {
	case <-gather:
	case <-time.After(5 * time.Second):
		t.Fatal("joiner candidate gathering timed out")
	}
	if err = creator.SetRemoteDescription(webrtc.SDPTypeAnswer, joiner.LocalDescription().SDP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	case <-time.After(8 * time.Second):
		t.Fatalf("no payload delivered: creator=%s joiner=%s", creator.pc.ConnectionState(), joiner.ConnectionState())
	}
}
