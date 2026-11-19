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
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ayn2op/tview"
	"github.com/ayn2op/tview/cmd"
	"github.com/gdamore/tcell/v3"
	"github.com/gorilla/websocket"
	"github.com/seamanticus/discordo/internal/discordtest"
	dhttp "github.com/seamanticus/discordo/internal/http"
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
func run(t *testing.T, command tview.Cmd) tview.Msg {
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
		want          tview.Msg
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
		m := NewModel(dhttp.Build{})
		if msg := run(t, m.Init()); msg != (startMsg{}) {
			t.Fatalf("Init sent %#v", msg)
		}
		m, command := m.Update(startMsg{})
		if command == nil || !strings.Contains(m.status, "Connecting") {
			t.Fatalf("status = %q, command = %v", m.status, command)
		}
	})

	t.Run("hello schedules a heartbeat at its interval", func(t *testing.T) {
		m, command := NewModel(dhttp.Build{}).Update(helloMsg{heartbeatInterval: 41250})
		if m.heartbeatInterval.Milliseconds() != 41250 {
			t.Fatalf("interval = %v", m.heartbeatInterval)
		}
		if batch, ok := command.(cmd.Batch); !ok || len(batch) != 3 {
			t.Fatalf("command = %#v, want listen, heartbeat and key generation", command)
		}
	})

	t.Run("no heartbeat once the connection is gone", func(t *testing.T) {
		if _, command := NewModel(dhttp.Build{}).Update(heartbeatTickMsg{}); command != nil {
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
			if m, _ := NewModel(dhttp.Build{}).Update(tt.msg); !strings.HasSuffix(m.status, tt.want) {
				t.Fatalf("status = %q, want suffix %q", m.status, tt.want)
			}
		}
	})

	t.Run("pending login closes the socket and redeems", func(t *testing.T) {
		m, command := NewModel(dhttp.Build{}).Update(pendingLoginMsg{ticket: "tk"})
		if batch, ok := command.(cmd.Batch); !ok || len(batch) != 2 || m.status != "Authenticating..." {
			t.Fatalf("status = %q, command = %#v", m.status, command)
		}
	})

	for _, tt := range []struct {
		name string
		msg  tview.Msg
		want string
	}{
		{"cancel on the phone", cancelMsg{}, "Login canceled on mobile"},
		{"Esc", tview.KeyMsg(tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone)), "Canceled"},
		{"an error", errors.New("boom"), "boom"},
	} {
		t.Run(tt.name+" closes the connection", func(t *testing.T) {
			m, command := NewModel(dhttp.Build{}).Update(tt.msg)
			if m.status != tt.want {
				t.Fatalf("status = %q, want %q", m.status, tt.want)
			}
			if msg := run(t, command); msg != (connCloseMsg{}) {
				t.Fatalf("close sent %#v", msg)
			}
		})
	}

	t.Run("closing forgets the connection", func(t *testing.T) {
		m := NewModel(dhttp.Build{})
		m.conn = &gatewayConn{}
		if m, _ = m.Update(connCloseMsg{}); m.conn != nil {
			t.Fatal("connection kept")
		}
	})

	t.Run("keys other than Esc do nothing", func(t *testing.T) {
		m := NewModel(dhttp.Build{})
		if got, command := m.Update(tview.KeyMsg(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))); command != nil || got.status != m.status {
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
	t.Run("a connection the server closed is an error", func(t *testing.T) {
		client, server := pipe(t)
		_ = server.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4003, "timeout"))
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
