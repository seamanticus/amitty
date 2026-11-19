package qr

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	"github.com/seamanticus/amitty/internal/gateway"
	"github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/skip2/go-qrcode"
)

type TokenMsg string

const remoteAuthGatewayURL = "wss://remote-auth-gateway.discord.gg/?v=2"

// gatewayConn is a Remote Auth Gateway connection whose writes are serialized, since a websocket.Conn takes one writer at a time.
type gatewayConn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
}

// writeJSON writes v, doing nothing once closeConn closed the connection, as a heartbeat may still be in flight.
func (c *gatewayConn) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.ws.WriteJSON(v); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

type connCreateMsg struct {
	conn *gatewayConn
}

// connCloseMsg says conn is closed; the model forgets it only if it is still the current one.
type connCloseMsg struct{ conn *gatewayConn }

// connect opens the Remote Auth Gateway through network with the handshake of the web client's login page, which carries no API headers.
func connect(network http.Network) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		dialer := gateway.NewDialer(network)
		conn, _, err := dialer.Dial(remoteAuthGatewayURL, gateway.HandshakeHeaders())
		if err != nil {
			return err
		}
		return connCreateMsg{conn: &gatewayConn{ws: conn}}
	})
}

func closeConn(conn *gatewayConn) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if conn == nil {
			return connCloseMsg{conn}
		}

		// An error here must not reach the error case, which would close the connection again.
		if err := conn.ws.Close(); err != nil {
			slog.Error("failed to close remote auth gateway connection", "err", err)
		}

		return connCloseMsg{conn}
	})
}

type helloMsg struct {
	heartbeatInterval int
	timeoutMS         int
}

type nonceProofMsg struct {
	encryptedNonce string
}

type pendingRemoteInitMsg struct {
	fingerprint string
}

type pendingTicketMsg struct {
	encryptedUserPayload string
}

type pendingLoginMsg struct {
	ticket string
}

type cancelMsg struct{}

// expiredMsg says the gateway ended the session because its time ran out.
type expiredMsg struct{}

// ignoredMsg is a gateway message the login does not act on, such as heartbeat_ack.
type ignoredMsg struct{}

func listen(conn *gatewayConn) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if conn == nil {
			return nil
		}

		_, data, err := conn.ws.ReadMessage()
		if errors.Is(err, net.ErrClosed) {
			// closeConn closed the connection.
			return nil
		}
		if closeErr, ok := errors.AsType[*websocket.CloseError](err); ok {
			return closedMsg(closeErr.Code)
		}
		if err != nil {
			return err
		}

		return decodeMessage(data)
	})
}

// closedMsg is the message for a gateway close code: expiry for 4003, else an error naming the cause.
func closedMsg(code int) tui.Msg {
	var cause string
	switch code {
	case 4003:
		return expiredMsg{}
	case 4000:
		cause = "invalid version"
	case 4001:
		cause = "invalid payload"
	case 4002:
		cause = "handshake failed"
	default:
		cause = "session closed"
	}
	return fmt.Errorf("remote auth gateway: %s (close %d)", cause, code)
}

func decodeMessage(data []byte) tui.Msg {
	var payload struct {
		Op                   string `json:"op"`
		HeartbeatInterval    int    `json:"heartbeat_interval"`
		TimeoutMS            int    `json:"timeout_ms"`
		EncryptedNonce       string `json:"encrypted_nonce"`
		Fingerprint          string `json:"fingerprint"`
		EncryptedUserPayload string `json:"encrypted_user_payload"`
		Ticket               string `json:"ticket"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	switch payload.Op {
	case "hello":
		return helloMsg{heartbeatInterval: payload.HeartbeatInterval, timeoutMS: payload.TimeoutMS}
	case "nonce_proof":
		return nonceProofMsg{encryptedNonce: payload.EncryptedNonce}
	case "pending_remote_init":
		return pendingRemoteInitMsg{fingerprint: payload.Fingerprint}
	case "pending_ticket":
		return pendingTicketMsg{encryptedUserPayload: payload.EncryptedUserPayload}
	case "cancel":
		return cancelMsg{}
	case "pending_login":
		return pendingLoginMsg{ticket: payload.Ticket}
	default:
		return ignoredMsg{}
	}
}

// heartbeatTickMsg is due on conn; the model drops it once conn is no longer current.
type heartbeatTickMsg struct{ conn *gatewayConn }

func scheduleHeartbeat(conn *gatewayConn, interval time.Duration) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		time.Sleep(interval)
		return heartbeatTickMsg{conn}
	})
}

// expiryTickMsg counts down one second of the time left on conn's code.
type expiryTickMsg struct{ conn *gatewayConn }

func tickExpiry(conn *gatewayConn) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		time.Sleep(time.Second)
		return expiryTickMsg{conn}
	})
}

func sendHeartbeat(conn *gatewayConn) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if conn == nil {
			return nil
		}
		data := struct {
			Op string `json:"op"`
		}{"heartbeat"}
		if err := conn.writeJSON(data); err != nil {
			return err
		}
		return nil
	})
}

type privateKeyMsg struct {
	privateKey *rsa.PrivateKey
}

func generatePrivateKey() tui.Cmd {
	return cmd.Task(func() tui.Msg {
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return err
		}
		return privateKeyMsg{privateKey: privateKey}
	})
}

func sendInit(conn *gatewayConn, privateKey *rsa.PrivateKey) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if privateKey == nil {
			return errors.New("missing private key")
		}
		spki, err := x509.MarshalPKIXPublicKey(privateKey.Public())
		if err != nil {
			return err
		}
		encodedPublicKey := base64.StdEncoding.EncodeToString(spki)
		data := struct {
			Op               string `json:"op"`
			EncodedPublicKey string `json:"encoded_public_key"`
		}{"init", encodedPublicKey}
		if err := conn.writeJSON(data); err != nil {
			return err
		}
		return nil
	})
}

// ownsFingerprint reports whether fingerprint is the base64url SHA-256 of key's SPKI, so the code links the phone to this key and no other.
func ownsFingerprint(key *rsa.PrivateKey, fingerprint string) bool {
	if key == nil {
		return false
	}
	spki, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return false
	}
	sum := sha256.Sum256(spki)
	return base64.RawURLEncoding.EncodeToString(sum[:]) == fingerprint
}

func sendNonceProof(conn *gatewayConn, privateKey *rsa.PrivateKey, encryptedNonce string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		decodedNonce, err := base64.StdEncoding.DecodeString(encryptedNonce)
		if err != nil {
			return err
		}

		decryptedNonce, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, decodedNonce, nil)
		if err != nil {
			return err
		}

		encodedNonce := base64.RawURLEncoding.EncodeToString(decryptedNonce)
		data := struct {
			Op    string `json:"op"`
			Nonce string `json:"nonce"`
		}{"nonce_proof", encodedNonce}
		if err := conn.writeJSON(data); err != nil {
			return err
		}
		return nil
	})
}

type qrCodeMsg struct {
	qrCode image.Image
}

func generateQRCode(fingerprint string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		content := "https://discord.com/ra/" + fingerprint
		qrCode, err := qrcode.New(content, qrcode.Low)
		if err != nil {
			return err
		}
		qrCode.DisableBorder = true
		return qrCodeMsg{qrCode: qrCode.Image(-1)}
	})
}

type userMsg struct {
	discriminator string
	username      string
}

func decryptUserPayload(privateKey *rsa.PrivateKey, encryptedPayload string) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		decodedPayload, err := base64.StdEncoding.DecodeString(encryptedPayload)
		if err != nil {
			return err
		}

		decryptedPayload, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, decodedPayload, nil)
		if err != nil {
			return err
		}

		parts := strings.Split(string(decryptedPayload), ":")
		if len(parts) != 4 {
			return errors.New("invalid user payload")
		}

		return userMsg{discriminator: parts[1], username: parts[3]}
	})
}

// retryMsg counts down, with left seconds to go, to the one retry of the exchange of ticket whose connection failed.
type retryMsg struct {
	ticket string
	left   int
	err    error
}

// exchangeTicket redeems ticket once, with the captcha solution if set. A first attempt whose connection failed starts the countdown to a retry; a final one fails.
func exchangeTicket(build http.Build, network http.Network, fingerprint string, solution captcha.Solution, privateKey *rsa.PrivateKey, ticket string, final bool) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), http.AuthTimeout)
		defer cancel()
		client := ticketClient(http.NewClient(network, ""), build, fingerprint)
		client.OnRequest = append(client.OnRequest, solution.Option())
		return retryOr(redeemTicket(ctx, client, privateKey, ticket), ticket, final)
	})
}

// retryOr returns msg, or the countdown to a retry when msg is the failure of a first exchange of ticket whose connection failed.
func retryOr(msg tui.Msg, ticket string, final bool) tui.Msg {
	if err, ok := msg.(error); ok && !final && http.Retryable(err) {
		return retryMsg{ticket: ticket, left: http.RetryDelay, err: err}
	}
	return msg
}

// tickRetry sends the countdown one second later.
func tickRetry(msg retryMsg) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		time.Sleep(time.Second)
		return retryMsg{ticket: msg.ticket, left: msg.left - 1, err: msg.err}
	})
}

// ticketClient adds to client the login page headers and, if set, the fingerprint of the remote auth session.
func ticketClient(client *api.Client, build http.Build, fingerprint string) *api.Client {
	headers := http.Headers(build)
	headers.Set("Referer", "https://discord.com/login")
	if fingerprint != "" {
		headers.Set("X-Fingerprint", fingerprint)
	}
	client.OnRequest = append(client.OnRequest, httputil.WithHeaders(headers))
	// A ticket is redeemed once; 0 would retry forever.
	client.Retries = 1
	return client
}

// redeemTicket exchanges ticket for the token encrypted to privateKey and returns a TokenMsg, a captchaMsg or an error.
func redeemTicket(ctx context.Context, client *api.Client, privateKey *rsa.PrivateKey, ticket string) tui.Msg {
	encryptedToken, err := client.WithContext(ctx).ExchangeRemoteAuthTicket(ticket)
	if challenge, ok := captcha.FromError(err); ok {
		return captchaMsg{ticket: ticket, challenge: challenge}
	}
	if err != nil {
		return err
	}

	decodedToken, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return err
	}

	decryptedToken, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, decodedToken, nil)
	if err != nil {
		return err
	}
	return TokenMsg(string(decryptedToken))
}

// startMsg starts connecting to the Remote Auth Gateway.
type startMsg struct{}
