package password

import (
	json "encoding/json/v2"
	"errors"
	"log/slog"
	"slices"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// codeVerifyLocation is the field error of a login from a place Discord has not seen the account at: it emailed a link that allows it.
const codeVerifyLocation = "ACCOUNT_LOGIN_VERIFICATION_EMAIL"

// verifyLocationError is a login Discord held until the link it emailed is clicked; logging in again afterwards succeeds.
type verifyLocationError struct{ err error }

func (e verifyLocationError) Error() string {
	return "Discord does not know this location yet and emailed you a link to verify the login. Click it, then log in again."
}

func (e verifyLocationError) Unwrap() error { return e.err }

// LogValue keeps Discord's own answer in the log, which the message shown leaves out.
func (e verifyLocationError) LogValue() slog.Value {
	return slog.GroupValue(slog.String("message", e.Error()), slog.String("cause", e.err.Error()))
}

// isVerifyLocation reports whether err is that held login.
func isVerifyLocation(err error) bool {
	_, ok := errors.AsType[verifyLocationError](err)
	return ok
}

// fieldCodes returns the codes of the errors Discord gave field in an Invalid Form Body, or none when err is not one.
func fieldCodes(err error, field string) []string {
	httpErr, ok := errors.AsType[*httputil.HTTPError](err)
	if !ok || len(httpErr.Errors) == 0 {
		return nil
	}
	var fields map[string]struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"_errors"`
	}
	if json.Unmarshal(httpErr.Errors, &fields) != nil {
		return nil
	}
	var codes []string
	for _, e := range fields[field].Errors {
		codes = append(codes, e.Code)
	}
	return codes
}

// explainLogin returns err as the error the user acts on, when Discord said why it refused the login.
func explainLogin(err error) error {
	if slices.Contains(fieldCodes(err, "login"), codeVerifyLocation) {
		return verifyLocationError{err}
	}
	return err
}
