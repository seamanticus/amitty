package qr

import (
	"crypto/rsa"
	"errors"
	"fmt"
	stdimage "image"
	"time"

	"github.com/gdamore/tcell/v3/color"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/box"
	"github.com/seamanticus/amitty/internal/tui/center"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/tui/column"
	"github.com/seamanticus/amitty/internal/tui/help"
	"github.com/seamanticus/amitty/internal/tui/image"
	"github.com/seamanticus/amitty/internal/tui/keybind"
	"github.com/seamanticus/amitty/internal/tui/layout"
	"github.com/seamanticus/amitty/internal/tui/text"
	"github.com/seamanticus/amitty/internal/ui"
	"github.com/seamanticus/amitty/internal/ui/login/keys"
)

const (
	// quietZoneColor is the white of the margin around the code, as go-qrcode draws its light modules.
	quietZoneColor = color.IsRGB | color.IsValid | 0xffffff
	// quietZoneRows and quietZoneColumns make a margin of two modules on every side, as a row holds two.
	quietZoneRows, quietZoneColumns = 1, 2
	// statusSpacing is the blank rows between the code and the status.
	statusSpacing = 1
)

type Model struct {
	// build is what the gateway and ticket requests identify as.
	build http.Build
	// browser holds the cookies and installation every request shares, and how they connect.
	browser *http.Browser

	conn              *gatewayConn
	heartbeatInterval time.Duration
	privateKey        *rsa.PrivateKey
	fingerprint       string
	// expiresIn is the seconds left before the gateway ends the session, or 0 when none are counted.
	expiresIn int
	// ended is set once the login stopped without a token; Renew then starts a new one.
	ended bool

	// captcha answers the challenges of the ticket exchange.
	captcha ui.Captcha[string]

	// qrCode is the code at one pixel per module, made once as it arrives.
	qrCode stdimage.Image
	status string
}

// NewModel returns a QR login whose requests identify as build and go through browser.
func NewModel(build http.Build, browser *http.Browser) Model {
	return Model{build: build, browser: browser, status: "Connecting to Remote Auth Gateway..."}
}

func (Model) Label() string {
	return "QR"
}

var _ tui.Model[Model] = Model{}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	if m.ended {
		return []keybind.Keybind{keys.Renew()}
	}
	return []keybind.Keybind{keys.Back("cancel")}
}

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tui.Cmd {
	return cmd.Task(func() tui.Msg { return startMsg{} })
}

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tui.Msg) (Model, tui.Cmd) {
	switch msg := msg.(type) {
	case startMsg:
		next := NewModel(m.build, m.browser)
		next.status = "Connecting to Remote Auth Gateway..."
		_, closing := m.captcha.Stop()
		return next, cmd.Batch{closeConn(m.conn), closing, connect(m.browser)}
	case tui.KeyMsg:
		switch {
		case m.ended && keybind.Matches(msg, keys.Renew()):
			return m.Update(startMsg{})
		case !m.ended && keybind.Matches(msg, keys.Back("")):
			return m.end("Canceled")
		}
		return m, nil

	case connCreateMsg:
		if m.ended || m.conn != nil {
			// Canceled while connecting, or a second connect lost the race.
			return m, closeConn(msg.conn)
		}
		m.conn = msg.conn
		m.status = "Connected. Handshaking..."
		return m, listen(m.conn)
	case connCloseMsg:
		if msg.conn == m.conn {
			m.conn = nil
		}
		return m, nil

	case helloMsg:
		m.heartbeatInterval = time.Duration(msg.heartbeatInterval) * time.Millisecond
		commands := cmd.Batch{listen(m.conn), scheduleHeartbeat(m.conn, m.heartbeatInterval), generatePrivateKey()}
		// Whole seconds, rounded down, so the code is gone before the gateway ends the session.
		if m.expiresIn = msg.timeoutMS / 1000; m.expiresIn > 0 {
			commands = append(commands, tickExpiry(m.conn))
		}
		return m, commands
	case privateKeyMsg:
		m.privateKey = msg.privateKey
		return m, sendInit(m.conn, m.privateKey)
	case nonceProofMsg:
		return m, cmd.Batch{listen(m.conn), sendNonceProof(m.conn, m.privateKey, msg.encryptedNonce)}
	case pendingRemoteInitMsg:
		if !ownsFingerprint(m.privateKey, msg.fingerprint) {
			return m, cmd.Task(func() tui.Msg {
				return errors.New("remote auth gateway sent a fingerprint that is not of our key; the code is not shown")
			})
		}
		m.fingerprint = msg.fingerprint
		return m, cmd.Batch{listen(m.conn), generateQRCode(msg.fingerprint)}
	case qrCodeMsg:
		m.qrCode = msg.qrCode
		m.status = "Scan this with the Discord mobile app to log in instantly."
		return m, nil
	case pendingTicketMsg:
		return m, cmd.Batch{listen(m.conn), decryptUserPayload(m.privateKey, msg.encryptedUserPayload)}
	case userMsg:
		name := msg.username
		if msg.discriminator != "0" {
			name += "#" + msg.discriminator
		}
		m.status = "Check your phone! Logging in as " + name
		return m, nil
	case pendingLoginMsg:
		m.status = "Authenticating..."
		m.expiresIn = 0
		var closing tui.Cmd
		m.captcha, closing = m.captcha.Start(msg.ticket)
		return m, cmd.Batch{closeConn(m.conn), closing, exchangeTicket(m.build, m.browser, m.fingerprint, m.captcha.Solution(), m.privateKey, msg.ticket, false)}
	case retryMsg:
		if m.ended {
			return m, nil
		}
		if msg.left <= 0 {
			m.status = "Authenticating..."
			return m, exchangeTicket(m.build, m.browser, m.fingerprint, m.captcha.Solution(), m.privateKey, msg.ticket, true)
		}
		m.status = fmt.Sprintf("Connection failed, retrying once in %d s: %v", msg.left, msg.err)
		return m, tickRetry(msg)
	case captchaMsg, captchaStepMsg, captchaDoneMsg:
		return m.updateCaptcha(msg)
	case ignoredMsg:
		return m, listen(m.conn)
	case cancelMsg:
		return m.end("Login canceled on mobile")
	case expiredMsg:
		return m.end("Code expired")

	case heartbeatTickMsg:
		if msg.conn == nil || msg.conn != m.conn {
			return m, nil
		}
		return m, cmd.Batch{scheduleHeartbeat(m.conn, m.heartbeatInterval), sendHeartbeat(m.conn)}
	case expiryTickMsg:
		if msg.conn == nil || msg.conn != m.conn || m.ended || m.expiresIn <= 0 {
			return m, nil
		}
		if m.expiresIn--; m.expiresIn == 0 {
			return m.end("Code expired")
		}
		return m, tickExpiry(m.conn)

	case error:
		if m.ended {
			// A task of the login that already stopped.
			return m, nil
		}
		return m.end(msg.Error())
	}

	return m, nil
}

// Leave ends a login in progress, closing its connection, as its tab is no longer shown; Init starts a new one.
func (m Model) Leave() (Model, tui.Cmd) {
	if m.ended {
		return m, nil
	}
	m, closing := m.end("Canceled")
	// Forgotten now, as the hidden tab does not get the close message.
	m.conn = nil
	return m, closing
}

// Settle handles msg while the tab is hidden: a connection that opens after Leave is closed, and anything else is dropped.
func (m Model) Settle(msg tui.Msg) (Model, tui.Cmd) {
	if msg, ok := msg.(connCreateMsg); ok {
		return m, closeConn(msg.conn)
	}
	return m, nil
}

// end stops the login with status, hiding the code and closing the connection and any captcha page.
func (m Model) end(status string) (Model, tui.Cmd) {
	m.status = status
	m.ended = true
	m.qrCode = nil
	m.expiresIn = 0
	var closing tui.Cmd
	m.captcha, closing = m.captcha.Stop()
	return m, cmd.Batch{closeConn(m.conn), closing}
}

// View centers the code above the status.
func (m Model) View() tui.Widget {
	var qrCode tui.Widget
	if m.qrCode != nil {
		// The white margin around the code, two modules wide, is what scanners find it by against a dark terminal.
		qrCode = box.New(image.New(m.qrCode)).
			Background(quietZoneColor).
			Padding(quietZoneRows, quietZoneRows, quietZoneColumns, quietZoneColumns).
			Width(layout.Shrink).
			Height(layout.Shrink)
	}

	var note tui.Widget
	switch {
	case m.ended:
		note = text.New("Press " + keys.Renew().Help().Key + " for a new code")
	case m.qrCode != nil && m.expiresIn > 0:
		note = text.New(fmt.Sprintf("Code expires in %d:%02d", m.expiresIn/60, m.expiresIn%60))
	}

	status := m.status
	if m.captcha.Waiting() {
		status = m.captcha.Prompt()
	}
	return center.New(column.New(qrCode, text.New(status), note).
		Spacing(statusSpacing).
		AlignX(tui.HorizontalAlignmentCenter).
		Width(layout.Shrink).
		Height(layout.Shrink))
}
