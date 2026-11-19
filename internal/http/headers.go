package http

import (
	"encoding/base64"
	json "encoding/json/v2"
	"log/slog"
	stdHttp "net/http"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/pkg/tzlocal"
)

// apiHeaderOrder returns the order Chromium sends a fetch to Discord's API in, from a NetLog of the web client in Brave 157 (Chromium 157), Brave's sec-gpc left out. The x-captcha-* places were not captured and are a guess. Headers not listed go last, sorted.
func apiHeaderOrder() []string {
	return []string{
		"content-length",
		"sec-ch-ua-platform", "authorization", "x-installation-id", "x-debug-options",
		"sec-ch-ua", "sec-ch-ua-mobile", "x-discord-timezone", "x-context-properties", "x-super-properties", "x-discord-locale",
		"x-captcha-key", "x-captcha-session-id", "x-captcha-rqtoken", "x-fingerprint",
		"user-agent", "content-type", "accept", "origin",
		"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "referer",
		"accept-encoding", "accept-language", "cookie", "priority",
	}
}

// imageHeaderOrder returns the order Chromium sends an image request to Discord's CDN in, from the same NetLog.
func imageHeaderOrder() []string {
	return []string{
		"sec-ch-ua-platform", "user-agent", "sec-ch-ua", "sec-ch-ua-mobile", "accept",
		"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "sec-fetch-storage-access", "referer",
		"accept-encoding", "accept-language", "cookie", "priority",
	}
}

// orderHeaders makes header go out in order rather than sorted.
func orderHeaders(header fhttp.Header, order []string) {
	header[fhttp.HeaderOrderKey] = order
}

// imageHeaders returns the headers Chromium sends when the page loads an image from the CDN, Brave's sec-gpc and language left out.
func imageHeaders() map[string]string {
	return map[string]string{
		"Sec-Ch-Ua-Platform":       `"` + osName + `"`,
		"User-Agent":               BrowserUserAgent(),
		"Sec-Ch-Ua":                browserBrands(),
		"Sec-Ch-Ua-Mobile":         "?0",
		"Accept":                   "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
		"Sec-Fetch-Site":           "cross-site",
		"Sec-Fetch-Mode":           "no-cors",
		"Sec-Fetch-Dest":           "image",
		"Sec-Fetch-Storage-Access": "active",
		"Referer":                  api.BaseEndpoint + "/",
		"Accept-Language":          "en-US,en;q=0.9",
		"Priority":                 "i",
	}
}

// Headers returns the headers the web client of build sends with its API requests.
func Headers(build Build) stdHttp.Header {
	headers := make(stdHttp.Header)
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers
	headers.Set("Accept", "*/*")
	headers.Set("Accept-Language", "en-US,en;q=0.9")
	headers.Set("Origin", api.BaseEndpoint)
	headers.Set("Priority", "u=1, i")
	headers.Set("Referer", "https://discord.com/channels/@me")

	headers.Set("Sec-Fetch-Dest", "empty")
	headers.Set("Sec-Fetch-Mode", "cors")
	headers.Set("Sec-Fetch-Site", "same-origin")

	headers.Set("Sec-Ch-Ua", browserBrands())
	headers.Set("Sec-Ch-Ua-Mobile", "?0")
	headers.Set("Sec-Ch-Ua-Platform", `"`+osName+`"`)

	headers.Set("X-Debug-Options", "bugReporterEnabled")
	headers.Set("X-Discord-Locale", string(locale))
	if timezone, err := tzlocal.Name(); err != nil {
		slog.Error("failed to get the local time zone", "err", err)
	} else {
		headers.Set("X-Discord-Timezone", timezone)
	}

	superProps, err := json.Marshal(xSuperProperties(build))
	if err != nil {
		slog.Error("failed to marshal super props", "err", err)
	} else {
		headers.Set("X-Super-Properties", base64.StdEncoding.EncodeToString(superProps))
	}

	return headers
}
