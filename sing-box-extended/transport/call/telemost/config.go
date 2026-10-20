package telemost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/sagernet/sing/common/logger"
	N "github.com/sagernet/sing/common/network"
)

type TMConfig struct {
	AppVersion string
	SDKVersion string
}

func FetchConfig(dialer N.Dialer, logger logger.ContextLogger) (TMConfig, error) {
	var cfg TMConfig
	page, err := creatorGET(dialer, "https://telemost.yandex.ru/")
	if err != nil {
		return cfg, fmt.Errorf("failed to fetch telemost.yandex.ru: %w", err)
	}
	cfg, bundleURL, modern, err := parsePageConfig(page)
	if err != nil {
		return cfg, err
	}
	bundle, err := creatorGET(dialer, bundleURL)
	if err != nil {
		return cfg, fmt.Errorf("failed to fetch bundle: %w", err)
	}
	cfg.SDKVersion, err = parseSDKVersion(bundle, modern)
	if err != nil {
		return cfg, err
	}
	logger.Debug(fmt.Sprintf("[config] app=%s sdk=%s", cfg.AppVersion, cfg.SDKVersion))
	return cfg, nil
}

func parsePageConfig(page []byte) (TMConfig, string, bool, error) {
	var cfg TMConfig
	stateRe := regexp.MustCompile(`<script[^>]*id="preloaded-state"[^>]*>([\s\S]*?)</script>`)
	stateMatch := stateRe.FindSubmatch(page)
	if stateMatch == nil {
		assignment := regexp.MustCompile(`(?:var|let|const)\s+globalParams\s*=\s*`).FindIndex(page)
		if assignment == nil {
			return cfg, "", false, fmt.Errorf("telemost bootstrap not found in page")
		}
		var params struct {
			Version string `json:"version"`
		}
		if json.NewDecoder(bytes.NewReader(page[assignment[1]:])).Decode(&params) != nil || params.Version == "" {
			return cfg, "", true, fmt.Errorf("invalid telemost globalParams version")
		}
		cfg.AppVersion = params.Version
		bundle := regexp.MustCompile(`(?:https:)?//yastatic\.net/s3/chat-static/telemessenger/_/[0-9.]+/web/app\.js`).Find(page)
		if len(bundle) == 0 {
			return cfg, "", true, fmt.Errorf("telemessenger bundle URL not found in page")
		}
		bundleURL := string(bundle)
		if bytes.HasPrefix(bundle, []byte("//")) {
			bundleURL = "https:" + bundleURL
		}
		return cfg, bundleURL, true, nil
	}
	var state struct {
		Config struct {
			AppVersion string `json:"appVersion"`
		} `json:"config"`
		AppVersion string `json:"appVersion"`
	}
	if err := json.Unmarshal(stateMatch[1], &state); err != nil {
		return cfg, "", false, fmt.Errorf("failed to parse preloaded-state: %w", err)
	}
	cfg.AppVersion = state.Config.AppVersion
	if cfg.AppVersion == "" {
		cfg.AppVersion = state.AppVersion
	}
	if cfg.AppVersion == "" {
		return cfg, "", false, fmt.Errorf("appVersion not found in preloaded-state")
	}
	bundleRe := regexp.MustCompile(`https://telemost\.yastatic\.net/s3/telemost/_/main\.\w+\.[a-f0-9]+\.js`)
	bundleURL := bundleRe.FindString(string(page))
	if bundleURL == "" {
		return cfg, "", false, fmt.Errorf("main bundle URL not found in page")
	}
	return cfg, bundleURL, false, nil
}

func parseSDKVersion(bundle []byte, modern bool) (string, error) {
	sdkVerPatterns := []*regexp.Regexp{
		regexp.MustCompile(`goloom_sdk_version:"(\d+\.\d+\.\d+)"`),
		regexp.MustCompile(`"@yandex-video-platform/goloom-sdk":"(\d+\.\d+\.\d+)"`),
		regexp.MustCompile(`goloom-sdk\.(\d+\.\d+\.\d+)\.js`),
	}
	for _, re := range sdkVerPatterns {
		if m := re.FindSubmatch(bundle); m != nil {
			return string(m[1]), nil
		}
	}
	if modern {
		// The embedded Goloom build exports a version/date pair. Require a
		// unique version; do not guess from unrelated libraries' versions.
		matches := regexp.MustCompile(`version:"(\d+\.\d+\.\d+)",date:"\d{4}-\d{2}-\d{2}T[0-9:.]+Z"`).FindAllSubmatch(bundle, -1)
		version := ""
		for _, match := range matches {
			if version != "" && version != string(match[1]) {
				return "", fmt.Errorf("ambiguous Goloom build version")
			}
			version = string(match[1])
		}
		if version != "" && bytes.Contains(bundle, []byte("goloom_publisher_pc")) {
			return version, nil
		}
	}
	return "", fmt.Errorf("goloom SDK version not found in bundle")
}
