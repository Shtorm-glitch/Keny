package telemost

import "testing"

func TestPageConfigLayouts(t *testing.T) {
	for _, tc := range []struct {
		page, version, bundle string
		modern                bool
	}{
		{`<script id="preloaded-state">{"config":{"appVersion":"1.2.3"}}</script><script src="https://telemost.yastatic.net/s3/telemost/_/main.web.abc123.js"></script>`, "1.2.3", "https://telemost.yastatic.net/s3/telemost/_/main.web.abc123.js", false},
		{`<script>var globalParams = {"version":"213.3.0","nested":{"a":1}}; other();</script><script src="//yastatic.net/s3/chat-static/telemessenger/_/213.3.0/web/app.js"></script>`, "213.3.0", "https://yastatic.net/s3/chat-static/telemessenger/_/213.3.0/web/app.js", true},
	} {
		cfg, bundle, modern, err := parsePageConfig([]byte(tc.page))
		if err != nil || cfg.AppVersion != tc.version || bundle != tc.bundle || modern != tc.modern {
			t.Fatalf("unexpected bootstrap result: %v %q %v %v", cfg, bundle, modern, err)
		}
	}
	for _, page := range []string{`<html>captcha</html>`, `var globalParams = {"version":"213.3.0"};`, `var globalParams = {}; //yastatic.net/s3/chat-static/telemessenger/_/213.3.0/web/app.js`} {
		if _, _, _, err := parsePageConfig([]byte(page)); err == nil {
			t.Fatal("accepted incomplete bootstrap")
		}
	}
}

func TestSDKVersionLayouts(t *testing.T) {
	for _, tc := range []struct {
		bundle, version string
		modern          bool
	}{
		{`goloom_sdk_version:"5.1.0"`, "5.1.0", false},
		{`const unrelated={version:"1.2.3"};goloom_publisher_pc;const sdk={version:"6.5.0",date:"2026-09-16T06:49:21.978Z"};`, "6.5.0", true},
	} {
		version, err := parseSDKVersion([]byte(tc.bundle), tc.modern)
		if err != nil || version != tc.version {
			t.Fatalf("SDK version %q: %v", version, err)
		}
	}
	for _, bundle := range []string{`version:"1.2.3"`, `version:"6.5.0",date:"2026-09-16T06:49:21.978Z"`, `goloom_publisher_pc;version:"6.5.0",date:"2026-09-16T06:49:21.978Z";version:"7.0.0",date:"2026-09-16T06:49:21.978Z"`} {
		if _, err := parseSDKVersion([]byte(bundle), true); err == nil {
			t.Fatal("accepted absent or ambiguous SDK metadata")
		}
	}
}
