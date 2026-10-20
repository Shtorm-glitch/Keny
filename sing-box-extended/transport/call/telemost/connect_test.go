package telemost

import (
	"encoding/json"
	"testing"
)

func TestJoinerTuningParams(t *testing.T) {
	disabled := false
	for _, tc := range []struct {
		name     string
		options  JoinerOptions
		reliable bool
	}{
		{"default preserves reliability", JoinerOptions{}, true},
		{"explicit false survives serialization", JoinerOptions{VP8FPS: 24, VP8Batch: 24, DualTrack: true, Reliable: &disabled}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var parsed struct {
				JoinLink    string `json:"joinLink"`
				DisplayName string `json:"displayName"`
				VP8FPS      int    `json:"vp8Fps"`
				VP8Batch    int    `json:"vp8Batch"`
				DualTrack   bool   `json:"dualTrack"`
				Reliable    bool   `json:"reliable"`
			}
			if err := json.Unmarshal([]byte(joinerParams("test-link", "quoted \"name\"", tc.options)), &parsed); err != nil {
				t.Fatal(err)
			}
			if parsed.JoinLink != "test-link" || parsed.DisplayName != "quoted \"name\"" || parsed.Reliable != tc.reliable || parsed.VP8FPS != tc.options.VP8FPS || parsed.VP8Batch != tc.options.VP8Batch || parsed.DualTrack != tc.options.DualTrack {
				t.Fatalf("tuning parameters not preserved: %+v", parsed)
			}
		})
	}
}
