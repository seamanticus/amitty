package password

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// FingerprintMsg carries the fingerprint that the login of final attempt is sent with.
type FingerprintMsg struct {
	fingerprint string
	final       bool
}

// fetchFingerprint gets the fingerprint that ties a login to its two-factor step, then makes the login page's other calls with it, as the web client does before it posts credentials.
func fetchFingerprint(build dhttp.Build, browser *dhttp.Browser, final bool) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		msg := requestFingerprint(ctx, authClient(browser.Client(""), build, ""), browser)
		if fp, ok := msg.(FingerprintMsg); ok {
			visitLoginPage(ctx, authClient(browser.Client(""), build, fp.fingerprint))
			fp.final = final
			return fp
		}
		return countdownOr(msg, final)
	})
}

// visitLoginPage makes with client the calls the web client's login page makes on loading, in its order. Their answers do not matter to the login, so a failure is only logged.
func visitLoginPage(ctx context.Context, client *api.Client) {
	client = client.WithContext(ctx)
	if err := client.LocationMetadata(); err != nil {
		slog.Warn("failed to get the login location metadata", "err", err)
	}
	if err := client.StartConditional(); err != nil {
		slog.Warn("failed to start the passkey login", "err", err)
	}
}

// requestFingerprint asks with client, which must send no fingerprint or token, for a new one and returns a FingerprintMsg or a FailedMsg. The installation ID in the answer goes to browser, as the web client keeps it.
func requestFingerprint(ctx context.Context, client *api.Client, browser *dhttp.Browser) tui.Msg {
	resp, err := client.WithContext(ctx).RequestJSON[struct {
		Fingerprint  string `json:"fingerprint"`
		Installation string `json:"installation"`
	}](http.MethodGet, api.Endpoint+"experiments")
	if err != nil {
		return FailedMsg{err}
	}
	browser.SetInstallation(resp.Installation)
	if resp.Fingerprint == "" {
		return FailedMsg{errors.New("experiments response did not include a fingerprint")}
	}
	return FingerprintMsg{fingerprint: resp.Fingerprint}
}
