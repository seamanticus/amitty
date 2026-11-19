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

// headerOrder returns the order Chromium sends an API request's headers in, from a capture of the official client on Chromium 152 (neucord #92: GET /users/@me, and content-length first on a POST). Where content-type, origin and x-captcha-* go was not captured; they are placed by guess. Headers not listed go last, sorted.
func headerOrder() []string {
	return []string{
		"content-length",
		"sec-ch-ua-platform", "authorization", "x-debug-options", "accept-language",
		"sec-ch-ua", "sec-ch-ua-mobile", "x-discord-timezone", "x-super-properties", "x-discord-locale",
		"x-captcha-key", "x-captcha-session-id", "x-captcha-rqtoken",
		"content-type", "user-agent", "x-fingerprint", "accept", "origin",
		"sec-fetch-site", "sec-fetch-mode", "sec-fetch-dest", "referer", "accept-encoding",
		"cookie", "priority",
	}
}

// orderHeaders makes header go out in Chromium's order rather than sorted.
func orderHeaders(header fhttp.Header) {
	header[fhttp.HeaderOrderKey] = headerOrder()
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
