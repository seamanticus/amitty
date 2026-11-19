package http

import (
	"fmt"
	"strconv"
	"strings"
	"uuid"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/tls"
)

//go:generate go run generator.go

const (
	osName = "Linux"
	// The web client reads the OS version from the user agent, which on Linux has none.
	osVersion = ""
	locale    = discord.EnglishUS
)

// BrowserUserAgent is Chrome's reduced user agent on Linux, which names x86_64 on every architecture.
func BrowserUserAgent() string {
	return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) " + tls.BrowserName() + "/" + tls.BrowserVersion() + " Safari/537.36"
}

func browserBrands() string {
	return brands(tls.BrowserName(), tls.BrowserMajorVersion())
}

// brands returns the Sec-Ch-Ua of browser name at major version, whose GREASE brand and order Chromium derives from the version.
func brands(name, version string) string {
	seed, _ := strconv.Atoi(version)
	// Reference: https://github.com/chromium/chromium/blob/dfb6413cdd961f33d080d48d851dfe8ac66715f5/components/embedder_support/user_agent_utils.cc#L535
	chars := []string{" ", "(", ":", "-", ".", "/", ")", ";", "=", "?", "_"}
	list := []string{
		fmt.Sprintf(`"Not%sA%sBrand";v="%s"`, chars[seed%11], chars[(seed+1)%11], []string{"8", "99", "24"}[seed%3]),
		fmt.Sprintf(`"Chromium";v="%s"`, version),
		fmt.Sprintf(`"Google %s";v="%s"`, name, version),
	}

	// Reference: https://github.com/chromium/chromium/blob/dfb6413cdd961f33d080d48d851dfe8ac66715f5/components/embedder_support/user_agent_utils.cc#L242
	order := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}[seed%6]
	shuffled := make([]string, len(list))
	for i, at := range order {
		shuffled[at] = list[i]
	}
	return strings.Join(shuffled, ", ")
}

func commonProperties(build Build) gateway.IdentifyProperties {
	return gateway.IdentifyProperties{
		gateway.IdentifyDevice: "",

		gateway.IdentifyOS: osName,
		"os_version":       osVersion,

		gateway.IdentifyBrowser: tls.BrowserName(),
		"browser_version":       tls.BrowserVersion(),
		"browser_user_agent":    BrowserUserAgent(),

		"client_build_number": build.Number(),
		"client_event_source": nil,
		"client_launch_id":    uuid.NewV4().String(),

		"system_locale":   locale,
		"release_channel": "stable",
		"has_client_mods": false,

		"referrer":                 "",
		"referrer_current":         "",
		"referring_domain":         "",
		"referring_domain_current": "",
	}
}

func IdentifyProperties(build Build) gateway.IdentifyProperties {
	props := commonProperties(build)
	props["is_fast_connect"] = true
	return props
}

func xSuperProperties(build Build) gateway.IdentifyProperties {
	props := commonProperties(build)
	props["client_app_state"] = "focused"
	props["client_heartbeat_session_id"] = uuid.NewV4().String()
	props["launch_signature"] = generateLaunchSignature()
	return props
}

func generateLaunchSignature() string {
	// Discord uses specific UUID bits to detect client modifications.
	// This mask clears detection bits to avoid identification.
	// Reference: https://docs.discord.food/reference#launch-signature
	//
	// Required version and variant bits for UUIDv4 validity are set by uuid.NewV4.
	// Reference: https://pkg.go.dev/uuid#NewV4
	mask := [16]byte{
		0b11111111, 0b01111111, 0b11101111, 0b11101111,
		0b11110111, 0b11101111, 0b11110111, 0b11111111,
		0b11011111, 0b01111110, 0b11111111, 0b10111111,
		0b11111110, 0b11111111, 0b11110111, 0b11111111,
	}
	id := uuid.NewV4()
	for i := range mask {
		id[i] &= mask[i]
	}
	return id.String()
}
