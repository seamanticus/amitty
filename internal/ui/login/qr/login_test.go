package qr

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gorilla/websocket"
	"github.com/seamanticus/amitty/internal/discordtest"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
	dhttp "github.com/seamanticus/amitty/internal/http"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
	"github.com/seamanticus/amitty/internal/ui/login/keys"
)

// testKey returns a fresh RSA-2048 key, as the login makes one per attempt.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// encrypt is what Discord does with the public key the client sent.
func encrypt(t *testing.T, key *rsa.PrivateKey, plain string) string {
	t.Helper()
	sealed, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, []byte(plain), nil)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sealed)
}

// run runs a task command and returns its message.
func run(t *testing.T, command tui.Cmd) tui.Msg {
	t.Helper()
	task, ok := command.(cmd.Task)
	if !ok {
		t.Fatalf("command = %#v, want a task", command)
	}
	return task()
}

// pipe returns a client connection whose server side the test reads from.
func pipe(t *testing.T) (*gatewayConn, *websocket.Conn) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		serverConn <- conn
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	conn := <-serverConn
	t.Cleanup(func() { _ = conn.Close() })
	return &gatewayConn{ws: client}, conn
}

func TestHandshake(t *testing.T) {
	key := testKey(t)

	t.Run("init sends the public key as base64 SPKI", func(t *testing.T) {
		client, server := pipe(t)
		if msg := run(t, sendInit(client, key)); msg != nil {
			t.Fatalf("sendInit returned %#v", msg)
		}
		var got struct {
			Op               string `json:"op"`
			EncodedPublicKey string `json:"encoded_public_key"`
		}
		if err := server.ReadJSON(&got); err != nil {
			t.Fatal(err)
		}
		der, err := base64.StdEncoding.DecodeString(got.EncodedPublicKey)
		if err != nil {
			t.Fatal(err)
		}
		pub, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			t.Fatal(err)
		}
		if got.Op != "init" || !key.PublicKey.Equal(pub) {
			t.Fatalf("op = %q, key matches = %v", got.Op, key.PublicKey.Equal(pub))
		}
	})

	t.Run("init without a key fails before writing", func(t *testing.T) {
		if _, ok := run(t, sendInit(nil, nil)).(error); !ok {
			t.Fatal("want an error")
		}
	})

	t.Run("nonce proof is the decrypted nonce, base64url without padding", func(t *testing.T) {
		client, server := pipe(t)
		// Two bytes encode with padding in base64, so a padded encoding would show.
		nonce := "\xfb\xff"
		if msg := run(t, sendNonceProof(client, key, encrypt(t, key, nonce))); msg != nil {
			t.Fatalf("sendNonceProof returned %#v", msg)
		}
		var got struct{ Op, Nonce string }
		if err := server.ReadJSON(&got); err != nil {
			t.Fatal(err)
		}
		if want := base64.RawURLEncoding.EncodeToString([]byte(nonce)); got.Op != "nonce_proof" || got.Nonce != want {
			t.Fatalf("got %+v, want nonce %q", got, want)
		}
	})

	for _, tt := range []struct{ name, encrypted string }{
		{"nonce that is not base64", "!!"},
		{"nonce encrypted to another key", ""},
		{"nonce that is not ciphertext", base64.StdEncoding.EncodeToString([]byte("plain"))},
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			encrypted := tt.encrypted
			if encrypted == "" {
				encrypted = encrypt(t, testKey(t), "nonce")
			}
			if _, ok := run(t, sendNonceProof(nil, key, encrypted)).(error); !ok {
				t.Fatal("want an error")
			}
		})
	}
}

// evidence: witnesses
func TestDecryptUserPayload(t *testing.T) {
	key := testKey(t)
	for _, tt := range []struct {
		name, payload string
		want          tui.Msg
	}{
		{"pomelo user", "1:0:avatar:name", userMsg{discriminator: "0", username: "name"}},
		{"legacy tag", "1:1234:avatar:name", userMsg{discriminator: "1234", username: "name"}},
		{"no avatar", "1:0::name", userMsg{discriminator: "0", username: "name"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, decryptUserPayload(key, encrypt(t, key, tt.payload))); got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
	for _, payload := range []string{"", "1:0:name", "1:0:a:b:c", "a colon:in:the:user:name"} {
		t.Run("rejects "+payload, func(t *testing.T) {
			if _, ok := run(t, decryptUserPayload(key, encrypt(t, key, payload))).(error); !ok {
				t.Fatalf("payload %q accepted", payload)
			}
		})
	}
	t.Run("rejects a payload that is not base64", func(t *testing.T) {
		if _, ok := run(t, decryptUserPayload(key, "%%")).(error); !ok {
			t.Fatal("accepted")
		}
	})
}

func TestRedeemTicket(t *testing.T) {
	key := testKey(t)
	const path = "/api/v9/users/@me/remote-auth/login"
	serve := func(t *testing.T, status int, body string, seen func(*http.Request, string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var b bytes.Buffer
			_, _ = b.ReadFrom(r.Body)
			if seen != nil {
				seen(r, b.String())
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
	}

	t.Run("posts the ticket and decrypts the token", func(t *testing.T) {
		var method, gotPath, ticket, fingerprint string
		reply, _ := json.Marshal(map[string]string{"encrypted_token": encrypt(t, key, "the-token")})
		client := ticketClient(discordtest.Client(t, serve(t, http.StatusOK, string(reply), func(r *http.Request, body string) {
			var b struct {
				Ticket string `json:"ticket"`
			}
			_ = json.Unmarshal([]byte(body), &b)
			method, gotPath, ticket, fingerprint = r.Method, r.URL.Path, b.Ticket, r.Header.Get("X-Fingerprint")
		})), dhttp.Build{}, "fp")
		if msg := redeemTicket(t.Context(), client, key, "tk"); msg != TokenMsg("the-token") {
			t.Fatalf("got %#v", msg)
		}
		if method != http.MethodPost || gotPath != path || ticket != "tk" || fingerprint != "fp" {
			t.Fatalf("request = %s %s ticket %q fingerprint %q", method, gotPath, ticket, fingerprint)
		}
	})

	t.Run("no fingerprint header without a fingerprint", func(t *testing.T) {
		present := true
		reply, _ := json.Marshal(map[string]string{"encrypted_token": encrypt(t, key, "t")})
		client := ticketClient(discordtest.Client(t, serve(t, http.StatusOK, string(reply), func(r *http.Request, _ string) {
			_, present = r.Header["X-Fingerprint"]
		})), dhttp.Build{}, "")
		redeemTicket(t.Context(), client, key, "tk")
		if present {
			t.Fatal("empty X-Fingerprint sent")
		}
	})

	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{"expired ticket", http.StatusBadRequest, `{"message":"Invalid ticket"}`},
		{"token not base64", http.StatusOK, `{"encrypted_token":"%%"}`},
		{"token for another key", http.StatusOK, ""},
		{"no token", http.StatusOK, `{}`},
	} {
		t.Run(tt.name+" fails", func(t *testing.T) {
			body := tt.body
			if body == "" {
				reply, _ := json.Marshal(map[string]string{"encrypted_token": encrypt(t, testKey(t), "t")})
				body = string(reply)
			}
			if _, ok := redeemTicket(t.Context(), discordtest.Client(t, serve(t, tt.status, body, nil)), key, "tk").(error); !ok {
				t.Fatal("want an error")
			}
		})
	}

	t.Run("a ticket is posted once, even on a 5xx", func(t *testing.T) {
		var calls atomic.Int32
		client := ticketClient(discordtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusBadGateway)
		})), dhttp.Build{}, "fp")
		redeemTicket(t.Context(), client, key, "tk")
		if n := calls.Load(); n != 1 {
			t.Fatalf("ticket posted %d times, want 1", n)
		}
	})

	t.Run("a canceled context fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err, ok := redeemTicket(ctx, discordtest.Client(t, serve(t, http.StatusOK, `{}`, nil)), key, "tk").(error)
		if !ok || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want context.Canceled", err)
		}
	})
}

func TestModelUpdate(t *testing.T) {
	t.Run("a new model asks to connect", func(t *testing.T) {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		if msg := run(t, m.Init()); msg != (startMsg{}) {
			t.Fatalf("Init sent %#v", msg)
		}
		m, command := m.Update(startMsg{})
		if command == nil || !strings.Contains(m.status, "Connecting") {
			t.Fatalf("status = %q, command = %v", m.status, command)
		}
	})

	t.Run("hello schedules a heartbeat at its interval", func(t *testing.T) {
		m, command := NewModel(dhttp.Build{}, dhttp.Network{}).Update(helloMsg{heartbeatInterval: 41250})
		if m.heartbeatInterval.Milliseconds() != 41250 {
			t.Fatalf("interval = %v", m.heartbeatInterval)
		}
		if batch, ok := command.(cmd.Batch); !ok || len(batch) != 3 {
			t.Fatalf("command = %#v, want listen, heartbeat and key generation", command)
		}
	})

	t.Run("no heartbeat once the connection is gone", func(t *testing.T) {
		if _, command := NewModel(dhttp.Build{}, dhttp.Network{}).Update(heartbeatTickMsg{}); command != nil {
			t.Fatal("heartbeat scheduled without a connection")
		}
	})

	t.Run("the user shows with a tag only when legacy", func(t *testing.T) {
		for _, tt := range []struct {
			msg  userMsg
			want string
		}{
			{userMsg{discriminator: "0", username: "name"}, "Logging in as name"},
			{userMsg{discriminator: "1234", username: "name"}, "Logging in as name#1234"},
		} {
			if m, _ := NewModel(dhttp.Build{}, dhttp.Network{}).Update(tt.msg); !strings.HasSuffix(m.status, tt.want) {
				t.Fatalf("status = %q, want suffix %q", m.status, tt.want)
			}
		}
	})

	t.Run("pending login closes the socket and redeems", func(t *testing.T) {
		m, command := NewModel(dhttp.Build{}, dhttp.Network{}).Update(pendingLoginMsg{ticket: "tk"})
		batch, ok := command.(cmd.Batch)
		// The captcha's reset adds a nil when no page waits.
		if !ok || len(slices.DeleteFunc(slices.Clone(batch), func(c cmd.Cmd) bool { return c == nil })) != 2 || m.status != "Authenticating..." {
			t.Fatalf("status = %q, command = %#v", m.status, command)
		}
	})

	for _, tt := range []struct {
		name string
		msg  tui.Msg
		want string
	}{
		{"cancel on the phone", cancelMsg{}, "Login canceled on mobile"},
		{"Esc", tui.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)), "Canceled"},
		{"an error", errors.New("boom"), "boom"},
	} {
		t.Run(tt.name+" closes the connection", func(t *testing.T) {
			m, command := NewModel(dhttp.Build{}, dhttp.Network{}).Update(tt.msg)
			if m.status != tt.want {
				t.Fatalf("status = %q, want %q", m.status, tt.want)
			}
			batch, ok := command.(cmd.Batch)
			if !ok || len(batch) == 0 {
				t.Fatalf("command = %#v, want a batch that closes", command)
			}
			if msg := run(t, batch[0]); msg != (connCloseMsg{}) {
				t.Fatalf("close sent %#v", msg)
			}
		})
	}

	t.Run("closing forgets the connection", func(t *testing.T) {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		m.conn = &gatewayConn{}
		if m, _ = m.Update(connCloseMsg{m.conn}); m.conn != nil {
			t.Fatal("connection kept")
		}
	})

	t.Run("closing an old connection keeps the current one", func(t *testing.T) {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		m.conn = &gatewayConn{}
		if m, _ = m.Update(connCloseMsg{&gatewayConn{}}); m.conn == nil {
			t.Fatal("current connection forgotten")
		}
	})

	t.Run("keys other than Esc do nothing", func(t *testing.T) {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		if got, command := m.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))); command != nil || got.status != m.status {
			t.Fatalf("status = %q, command = %v", got.status, command)
		}
	})
}

func TestListenClosed(t *testing.T) {
	t.Run("no connection reads nothing", func(t *testing.T) {
		if msg := run(t, listen(nil)); msg != nil {
			t.Fatalf("got %#v", msg)
		}
	})
	t.Run("a connection this client closed ends quietly", func(t *testing.T) {
		client, _ := pipe(t)
		_ = client.ws.Close()
		if msg := run(t, listen(client)); msg != nil {
			t.Fatalf("got %#v, want nil so the error case does not close again", msg)
		}
	})
	t.Run("a session the server timed out has expired", func(t *testing.T) {
		client, server := pipe(t)
		_ = server.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4003, "timeout"))
		if msg := run(t, listen(client)); msg != (expiredMsg{}) {
			t.Fatalf("got %#v, want expiredMsg", msg)
		}
	})
	t.Run("a connection the server closed otherwise is an error", func(t *testing.T) {
		client, server := pipe(t)
		_ = server.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4002, "handshake"))
		if _, ok := run(t, listen(client)).(error); !ok {
			t.Fatal("want an error")
		}
	})
	t.Run("a malformed frame is an error", func(t *testing.T) {
		client, server := pipe(t)
		_ = server.WriteMessage(websocket.TextMessage, []byte("{"))
		if _, ok := run(t, listen(client)).(error); !ok {
			t.Fatal("want an error")
		}
	})
}

func TestGenerateQRCode(t *testing.T) {
	msg, ok := run(t, generateQRCode("UZ0-kOVzXDZTFVV5_QlpURSO2BQHrtkKWHNpIGoDI0k")).(qrCodeMsg)
	if !ok || msg.qrCode == nil || msg.qrCode.Bounds().Empty() {
		t.Fatalf("got %#v", msg)
	}
}

// evidence: witnesses
func TestOwnsFingerprint(t *testing.T) {
	key := testKey(t)
	spki, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(spki)
	own := base64.RawURLEncoding.EncodeToString(sum[:])
	for _, tt := range []struct {
		name        string
		key         *rsa.PrivateKey
		fingerprint string
		want        bool
	}{
		{"the key's own", key, own, true},
		{"padded base64url", key, base64.URLEncoding.EncodeToString(sum[:]), false},
		{"standard base64", key, base64.StdEncoding.EncodeToString(sum[:]), false},
		{"another key's", testKey(t), own, false},
		{"empty", key, "", false},
		{"no key", nil, own, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownsFingerprint(tt.key, tt.fingerprint); got != tt.want {
				t.Fatalf("ownsFingerprint = %v, want %v", got, tt.want)
			}
		})
	}
}

// evidence: witnesses
func TestClosedMsg(t *testing.T) {
	if msg := closedMsg(4003); msg != (expiredMsg{}) {
		t.Fatalf("4003 = %#v, want expiredMsg", msg)
	}
	for _, code := range []int{1000, 4000, 4001, 4002, 4004} {
		err, ok := closedMsg(code).(error)
		if !ok || !strings.Contains(err.Error(), strconv.Itoa(code)) {
			t.Fatalf("%d = %v, want an error naming the code", code, err)
		}
	}
}

// evidence: witnesses
func TestRetryOr(t *testing.T) {
	refused := &httputil.HTTPError{Status: http.StatusBadGateway}
	for _, tt := range []struct {
		name  string
		msg   tui.Msg
		final bool
		retry bool
	}{
		{"a first failed connection", refused, false, true},
		{"a final failed connection", refused, true, false},
		{"a rejected ticket", &httputil.HTTPError{Status: http.StatusBadRequest}, false, false},
		{"a token", TokenMsg("t"), false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := retryOr(tt.msg, "tk", tt.final)
			retry, ok := got.(retryMsg)
			if ok != tt.retry {
				t.Fatalf("got %#v, retry want %v", got, tt.retry)
			}
			if ok && (retry.ticket != "tk" || retry.left != dhttp.RetryDelay) {
				t.Fatalf("got %#v", retry)
			}
			if !ok && got != tt.msg {
				t.Fatalf("got %#v, want %#v unchanged", got, tt.msg)
			}
		})
	}
}

func TestModelLifecycle(t *testing.T) {
	press := func(m Model, key string) (Model, tui.Cmd) {
		return m.Update(tui.KeyMsg(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone)))
	}
	live := func() Model {
		m := NewModel(dhttp.Build{}, dhttp.Network{})
		m.conn = &gatewayConn{}
		return m
	}

	t.Run("hello starts the expiry countdown in whole seconds", func(t *testing.T) {
		m, command := live().Update(helloMsg{heartbeatInterval: 41250, timeoutMS: 142637})
		if m.expiresIn != 142 {
			t.Fatalf("expiresIn = %d, want 142", m.expiresIn)
		}
		if batch, ok := command.(cmd.Batch); !ok || len(batch) != 4 {
			t.Fatalf("command = %#v, want listen, heartbeat, key generation and expiry tick", command)
		}
	})

	t.Run("the countdown ends the login at zero", func(t *testing.T) {
		m := live()
		m.expiresIn = 2
		m, command := m.Update(expiryTickMsg{m.conn})
		if m.expiresIn != 1 || m.ended || command == nil {
			t.Fatalf("expiresIn = %d, ended = %v, command = %v", m.expiresIn, m.ended, command)
		}
		if m, _ = m.Update(expiryTickMsg{m.conn}); !m.ended || m.status != "Code expired" || m.qrCode != nil {
			t.Fatalf("ended = %v, status = %q", m.ended, m.status)
		}
	})

	t.Run("a tick of an old connection is dropped", func(t *testing.T) {
		m := live()
		m.expiresIn = 5
		for _, msg := range []tui.Msg{expiryTickMsg{&gatewayConn{}}, heartbeatTickMsg{&gatewayConn{}}} {
			if got, command := m.Update(msg); command != nil || got.expiresIn != 5 {
				t.Fatalf("%T: expiresIn = %d, command = %v", msg, got.expiresIn, command)
			}
		}
	})

	t.Run("the gateway's timeout ends the login", func(t *testing.T) {
		if m, _ := live().Update(expiredMsg{}); !m.ended || m.status != "Code expired" {
			t.Fatalf("ended = %v, status = %q", m.ended, m.status)
		}
	})

	t.Run("a fingerprint of another key is an error and shows no code", func(t *testing.T) {
		m := live()
		m.privateKey = testKey(t)
		m, command := m.Update(pendingRemoteInitMsg{fingerprint: "not-ours"})
		if _, ok := run(t, command).(error); !ok || m.fingerprint != "" {
			t.Fatalf("fingerprint = %q, command = %#v", m.fingerprint, command)
		}
	})

	t.Run("r renews only an ended login", func(t *testing.T) {
		m := live()
		if _, command := press(m, "r"); command != nil {
			t.Fatal("r restarted a running login")
		}
		m, _ = m.end("Code expired")
		if help := m.ShortHelp(); len(help) != 1 || help[0].Help() != keys.Renew().Help() {
			t.Fatalf("help = %v, want only renew", help)
		}
		m, command := press(m, "r")
		if m.ended || m.conn != nil || !strings.Contains(m.status, "Connecting") || command == nil {
			t.Fatalf("ended = %v, status = %q, command = %v", m.ended, m.status, command)
		}
	})

	t.Run("an ended login closes a connection that arrives late", func(t *testing.T) {
		m, _ := NewModel(dhttp.Build{}, dhttp.Network{}).end("Canceled")
		late := &gatewayConn{}
		m, command := m.Update(connCreateMsg{conn: late})
		if m.conn != nil || command == nil {
			t.Fatalf("conn = %v, command = %v", m.conn, command)
		}
	})

	t.Run("an ended login ignores late errors and retries", func(t *testing.T) {
		m, _ := NewModel(dhttp.Build{}, dhttp.Network{}).end("Code expired")
		for _, msg := range []tui.Msg{errors.New("late"), retryMsg{ticket: "tk", left: 1}} {
			if got, command := m.Update(msg); command != nil || got.status != "Code expired" {
				t.Fatalf("%T: status = %q, command = %v", msg, got.status, command)
			}
		}
	})

	t.Run("a retry counts down, then redeems once more", func(t *testing.T) {
		m, command := NewModel(dhttp.Build{}, dhttp.Network{}).Update(retryMsg{ticket: "tk", left: 3, err: errors.New("refused")})
		if !strings.Contains(m.status, "retrying once in 3 s: refused") || command == nil {
			t.Fatalf("status = %q, command = %v", m.status, command)
		}
		if m, command = m.Update(retryMsg{ticket: "tk"}); m.status != "Authenticating..." || command == nil {
			t.Fatalf("status = %q, command = %v", m.status, command)
		}
	})
}
