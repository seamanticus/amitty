package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	fhttp "github.com/bogdanfinn/fhttp"
)

// Build is the web client build that requests and the gateway identify as. Its zero value is the build generated into the source.
type Build struct {
	// number is the fetched build number, or 0 for the generated one.
	number int
}

// Number returns the build number to send.
func (b Build) Number() int {
	if b.number == 0 {
		return clientBuildNumber
	}
	return b.number
}

const (
	loginPageURL = "https://discord.com/login"
	// maxLoginPage bounds what is read of the login page, which is a few KiB of HTML.
	maxLoginPage = 1 << 20
)

var buildNumberRegex = regexp.MustCompile(`"BUILD_NUMBER":\s*"(\d+)"`)

// FetchBuild reads the current build number from the login page of the web client, through network and the same TLS fingerprint as every other request.
func FetchBuild(ctx context.Context, network Network) (Build, error) {
	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodGet, loginPageURL, nil)
	if err != nil {
		return Build{}, fmt.Errorf("fetch build: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", BrowserUserAgent())
	orderHeaders(req.Header)

	resp, err := network.TLSClient().Do(req)
	if err != nil {
		return Build{}, fmt.Errorf("fetch build: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != fhttp.StatusOK {
		return Build{}, fmt.Errorf("fetch build: login page status %s", resp.Status)
	}
	return readBuild(resp.Body)
}

// readBuild finds the build number in a login page.
func readBuild(page io.Reader) (Build, error) {
	body, err := io.ReadAll(io.LimitReader(page, maxLoginPage))
	if err != nil {
		return Build{}, fmt.Errorf("read login page: %w", err)
	}
	match := buildNumberRegex.FindSubmatch(body)
	if match == nil {
		return Build{}, errors.New("login page has no build number")
	}
	number, err := strconv.Atoi(string(match[1]))
	if err != nil || number <= 0 {
		return Build{}, fmt.Errorf("login page has an invalid build number %q", match[1])
	}
	return Build{number: number}, nil
}
