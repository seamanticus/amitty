package qr

import (
	"crypto/rsa"
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
	"github.com/seamanticus/discordo/internal/http"
	"github.com/seamanticus/discordo/internal/ui/login/keys"
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

func (Model) ShortHelp() []keybind.Keybind { return []keybind.Keybind{keys.Back("cancel")} }

func (m Model) FullHelp() [][]keybind.Keybind { return [][]keybind.Keybind{m.ShortHelp()} }

func (Model) Init() tview.Cmd {
	return cmd.Task(func() tview.Msg { return startMsg{} })
}

// Update returns m changed in response to msg and a command to run, or nil.
func (m Model) Update(msg tview.Msg) (Model, tview.Cmd) {
	switch msg := msg.(type) {
	case startMsg:
		m.status = "Connecting to Remote Auth Gateway..."
		return m, connect(m.build)
	case tview.KeyMsg:
		if keybind.Matches(msg, keys.Back("")) {
			m.status = "Canceled"
			return m, closeConn(m.conn)
		}
		return m, nil

	case connCreateMsg:
		m.conn = msg.conn
		m.status = "Connected. Handshaking..."
		return m, listen(m.conn)
	case connCloseMsg:
		m.conn = nil
		return m, nil

	case helloMsg:
		m.heartbeatInterval = time.Duration(msg.heartbeatInterval) * time.Millisecond
		return m, cmd.Batch{listen(m.conn), scheduleHeartbeat(m.heartbeatInterval), generatePrivateKey()}
	case privateKeyMsg:
		m.privateKey = msg.privateKey
		return m, sendInit(m.conn, m.privateKey)
	case nonceProofMsg:
		return m, cmd.Batch{listen(m.conn), sendNonceProof(m.conn, m.privateKey, msg.encryptedNonce)}
	case pendingRemoteInitMsg:
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
		return m, cmd.Batch{closeConn(m.conn), exchangeTicket(m.build, m.fingerprint, m.privateKey, msg.ticket)}
	case ignoredMsg:
		return m, listen(m.conn)
	case cancelMsg:
		m.status = "Login canceled on mobile"
		return m, closeConn(m.conn)

	case heartbeatTickMsg:
		if m.conn == nil {
			return m, nil
		}
		return m, cmd.Batch{scheduleHeartbeat(m.heartbeatInterval), sendHeartbeat(m.conn)}

	case error:
		m.status = msg.Error()
		return m, closeConn(m.conn)
	}

	return m, nil
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

	return center.New(column.New(qrCode, text.New(m.status)).
		Spacing(statusSpacing).
		AlignX(tview.HorizontalAlignmentCenter).
		Width(layout.Shrink).
		Height(layout.Shrink))
}
