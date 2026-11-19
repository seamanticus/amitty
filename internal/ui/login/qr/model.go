package qr

import (
	"crypto/rsa"
	"errors"
	"fmt"
	stdimage "image"
	"time"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/box"
	"github.com/ayn2op/tview/center"
	"github.com/ayn2op/tview/cmd"
	"github.com/ayn2op/tview/column"
	"github.com/ayn2op/tview/help"
	"github.com/ayn2op/tview/image"
	"github.com/ayn2op/tview/keybind"
	"github.com/ayn2op/tview/layout"
	"github.com/ayn2op/tview/text"
	"github.com/gdamore/tcell/v3/color"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/http"
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

	conn              *gatewayConn
	heartbeatInterval time.Duration
	privateKey        *rsa.PrivateKey
	fingerprint       string
	// expiresIn is the seconds left before the gateway ends the session, or 0 when none are counted.
	expiresIn int
	// ended is set once the login stopped without a token; Renew then starts a new one.
	ended bool

	// captchaTries counts the challenges answered for the ticket exchange.
	captchaTries int
	// captcha is the page waiting for an answer, or nil; captchaOpenErr is why no browser opened it.
	captcha        *captcha.Page
	captchaOpenErr string
	// solution is the last answer, sent with the exchange until Discord takes or challenges it again.
	solution captcha.Solution

	// qrCode is the code at one pixel per module, made once as it arrives.
	qrCode stdimage.Image
	status string
}

// NewModel returns a QR login whose requests identify as build.
func NewModel(build http.Build) Model {
	m := Model{build: build}
	m.status = "Press Ctrl+N to open QR login"
	return m
}

func (Model) Label() string {
	return "QR"
}

var _ tview.Model[Model] = Model{}

var _ help.KeyMap = Model{}

func (m Model) ShortHelp() []keybind.Keybind {
	if m.ended {
		return []keybind.Keybind{keys.Renew()}
	}
	return []keybind.Keybind{keys.Back("cancel")}
}

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tview.Cmd {
	return cmd.Task(func() tview.Msg { return startMsg{} })
}

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case startMsg:
		next := NewModel(m.build)
		next.status = "Connecting to Remote Auth Gateway..."
		return next, cmd.Batch{closeConn(m.conn), closeCaptcha(m.captcha), connect(m.build)}
	case tview.KeyMsg:
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
			return m, cmd.Task(func() tview.Msg {
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
		return m, cmd.Batch{closeConn(m.conn), exchangeTicket(m.build, m.fingerprint, m.solution, m.privateKey, msg.ticket, false)}
	case retryMsg:
		if m.ended {
			return m, nil
		}
		if msg.left <= 0 {
			m.status = "Authenticating..."
			return m, exchangeTicket(m.build, m.fingerprint, m.solution, m.privateKey, msg.ticket, true)
		}
		m.status = fmt.Sprintf("Connection failed, retrying once in %d s: %v", msg.left, msg.err)
		return m, tickRetry(msg)
	case captchaMsg, captchaPageMsg, captchaOpenFailedMsg, captchaSolvedMsg:
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

// end stops the login with status, hiding the code and closing the connection and any captcha page.
func (m Model) end(status string) (Model, tview.Cmd) {
	page := m.captcha
	m.status = status
	m.ended = true
	m.qrCode = nil
	m.expiresIn = 0
	m.captcha, m.captchaOpenErr = nil, ""
	return m, cmd.Batch{closeConn(m.conn), closeCaptcha(page)}
}

// View centers the code above the status.
func (m Model) View() tview.Widget {
	var qrCode tview.Widget
	if m.qrCode != nil {
		// The white margin around the code, two modules wide, is what scanners find it by against a dark terminal.
		qrCode = box.New(image.New(m.qrCode)).
			Background(quietZoneColor).
			Padding(quietZoneRows, quietZoneRows, quietZoneColumns, quietZoneColumns).
			Width(layout.Shrink).
			Height(layout.Shrink)
	}

	var note tview.Widget
	switch {
	case m.ended:
		note = text.New("Press " + keys.Renew().Help().Key + " for a new code")
	case m.qrCode != nil && m.expiresIn > 0:
		note = text.New(fmt.Sprintf("Code expires in %d:%02d", m.expiresIn/60, m.expiresIn%60))
	}

	return center.New(column.New(qrCode, text.New(m.status), note).
		Spacing(statusSpacing).
		AlignX(tview.HorizontalAlignmentCenter).
		Width(layout.Shrink).
		Height(layout.Shrink))
}
