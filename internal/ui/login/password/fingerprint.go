package password

import (
	"context"
	"errors"
	"net/http"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	dhttp "github.com/seamanticus/amitty/internal/http"
)

// FingerprintMsg carries the fingerprint that the login of final attempt is sent with.
type FingerprintMsg struct {
	fingerprint string
	final       bool
}

// fetchFingerprint gets the fingerprint that ties a login to its two-factor step, as the web client does before it posts credentials.
func fetchFingerprint(build dhttp.Build, network dhttp.Network, final bool) tview.Cmd {
	return cmd.Task(func() tview.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), dhttp.AuthTimeout)
		defer cancel()
		msg := requestFingerprint(ctx, authClient(dhttp.NewClient(network, ""), build, ""))
		if fp, ok := msg.(FingerprintMsg); ok {
			fp.final = final
			return fp
		}
		return countdownOr(msg, final)
	})
}

// requestFingerprint asks with client, which must send no fingerprint or token, for a new one and returns a FingerprintMsg or a FailedMsg.
func requestFingerprint(ctx context.Context, client *api.Client) tview.Msg {
	resp, err := client.WithContext(ctx).RequestJSON[struct {
		Fingerprint string `json:"fingerprint"`
	}](http.MethodGet, api.Endpoint+"experiments")
	if err != nil {
		return FailedMsg{err}
	}
	if resp.Fingerprint == "" {
		return FailedMsg{errors.New("experiments response did not include a fingerprint")}
	}
	return FingerprintMsg{fingerprint: resp.Fingerprint}
}
