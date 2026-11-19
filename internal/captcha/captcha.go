// Package captcha answers Discord's hCaptcha challenges in the user's browser, through a one-shot page on the loopback address.
package captcha

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// AnswerTimeout bounds how long a page waits for the user to answer, waited on or not.
const AnswerTimeout = 5 * time.Minute

// MaxTries bounds the challenges answered for one request; Discord answers a rejected answer with a new challenge.
const MaxTries = 3

// maxAnswerBytes bounds the answer the page posts back; an hCaptcha key is about 3 KiB.
const maxAnswerBytes = 64 << 10

//go:embed page.html
var pageHTML string

// Challenge is a captcha Discord asks for before it takes a request.
type Challenge struct {
	sitekey   string
	sessionID string
	rqdata    string
	rqtoken   string
	invisible bool
}

// FromError returns the hCaptcha challenge that err carries, if Discord answered with one.
func FromError(err error) (Challenge, bool) {
	httpErr, ok := errors.AsType[*httputil.HTTPError](err)
	if !ok || httpErr.Status != http.StatusBadRequest {
		return Challenge{}, false
	}
	var body struct {
		Service   string `json:"captcha_service"`
		Sitekey   string `json:"captcha_sitekey"`
		SessionID string `json:"captcha_session_id"`
		RQData    string `json:"captcha_rqdata"`
		RQToken   string `json:"captcha_rqtoken"`
		Invisible bool   `json:"should_serve_invisible"`
	}
	if json.Unmarshal(httpErr.Body, &body) != nil || body.Service != "hcaptcha" || body.Sitekey == "" {
		return Challenge{}, false
	}
	return Challenge{
		sitekey:   body.Sitekey,
		sessionID: body.SessionID,
		rqdata:    body.RQData,
		rqtoken:   body.RQToken,
		invisible: body.Invisible,
	}, true
}

// Solution is the answer to a challenge, sent again with the request Discord challenged; its zero value is no answer.
type Solution struct {
	key       string
	sessionID string
	rqtoken   string
}

// Headers returns the headers that carry s, or none for the zero Solution.
func (s Solution) Headers() http.Header {
	headers := make(http.Header)
	if s.key == "" {
		return headers
	}
	headers.Set("X-Captcha-Key", s.key)
	if s.sessionID != "" {
		headers.Set("X-Captcha-Session-Id", s.sessionID)
	}
	if s.rqtoken != "" {
		headers.Set("X-Captcha-Rqtoken", s.rqtoken)
	}
	return headers
}

// Option adds the headers of s to a request.
func (s Solution) Option() httputil.RequestOption { return httputil.WithHeaders(s.Headers()) }

// Page shows one challenge on http://127.0.0.1 and takes one answer. hCaptcha rejects "localhost" as a host, so the address is literal.
type Page struct {
	challenge Challenge
	url       string
	origin    string
	server    *http.Server
	served    sync.WaitGroup
	answers   chan string
	// lifetime ends after AnswerTimeout or with the context Serve got, closing the page.
	lifetime  context.Context
	end       context.CancelFunc
	closed    chan struct{}
	closeOnce sync.Once
}

// Serve starts a page for challenge on a free loopback port, under a random path only its URL knows. The page closes once ctx ends or AnswerTimeout passes.
func Serve(ctx context.Context, challenge Challenge) (*Page, error) {
	if challenge.sitekey == "" {
		return nil, errors.New("captcha challenge has no sitekey")
	}
	page, err := render(challenge)
	if err != nil {
		return nil, err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for the captcha page: %w", err)
	}
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return nil, fmt.Errorf("listen for the captcha page: unexpected address %v", listener.Addr())
	}

	path := "/" + rand.Text()
	p := &Page{
		challenge: challenge,
		origin:    fmt.Sprintf("http://127.0.0.1:%d", addr.Port),
		answers:   make(chan string, 1),
		closed:    make(chan struct{}),
	}
	p.url = p.origin + path
	p.lifetime, p.end = context.WithTimeout(ctx, AnswerTimeout)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("POST "+path+"/answer", p.takeAnswer)
	p.server = &http.Server{Handler: p.checkHost(mux), ReadHeaderTimeout: 10 * time.Second}
	p.served.Go(func() { _ = p.server.Serve(listener) })
	context.AfterFunc(p.lifetime, p.Close)
	return p, nil
}

// render returns the page with challenge embedded as JSON that cannot end its script element.
func render(challenge Challenge) ([]byte, error) {
	data, err := json.Marshal(struct {
		Sitekey   string `json:"sitekey"`
		RQData    string `json:"rqdata,omitempty"`
		Invisible bool   `json:"invisible"`
	}{challenge.sitekey, challenge.rqdata, challenge.invisible}, jsontext.EscapeForHTML(true))
	if err != nil {
		return nil, fmt.Errorf("render the captcha page: %w", err)
	}
	return []byte(strings.Replace(pageHTML, "CHALLENGE", string(data), 1)), nil
}

// checkHost refuses requests not addressed to the page's own host, so no other name can be rebound to it.
func (p *Page) checkHost(next http.Handler) http.Handler {
	host := strings.TrimPrefix(p.origin, "http://")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Error(w, "wrong host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// takeAnswer takes the key the page posts once hCaptcha is solved, only from the page itself.
func (p *Page) takeAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != p.origin {
		http.Error(w, "wrong origin", http.StatusForbidden)
		return
	}
	var answer struct {
		Key string `json:"key"`
	}
	if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, maxAnswerBytes), &answer); err != nil || answer.Key == "" {
		http.Error(w, "no answer", http.StatusBadRequest)
		return
	}
	select {
	case p.answers <- answer.Key:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "already answered", http.StatusConflict)
	}
}

// URL is where the browser opens the page.
func (p *Page) URL() string { return p.url }

// Prompt tells the user to answer the page, as try of MaxTries, and how by hand when no browser opened it because of openErr.
func (p *Page) Prompt(try int, openErr string) string {
	prompt := fmt.Sprintf("Solve the captcha in your browser (try %d of %d)", try, MaxTries)
	if openErr != "" {
		prompt += fmt.Sprintf("\nNo browser opened (%s). Open %s", openErr, p.url)
	}
	return prompt
}

// Wait returns the answer once the user gives one, or an error once ctx ends or Close is called. The page closes either way.
func (p *Page) Wait(ctx context.Context) (Solution, error) {
	defer p.Close()
	select {
	case key := <-p.answers:
		return Solution{key: key, sessionID: p.challenge.sessionID, rqtoken: p.challenge.rqtoken}, nil
	case <-ctx.Done():
		return Solution{}, fmt.Errorf("captcha: %w", ctx.Err())
	case <-p.closed:
		if errors.Is(p.lifetime.Err(), context.DeadlineExceeded) {
			return Solution{}, fmt.Errorf("captcha not answered within %v", AnswerTimeout)
		}
		return Solution{}, errors.New("captcha canceled")
	}
}

// Close stops the page, letting an answer in flight finish, and waits for its server. It is safe to call more than once.
func (p *Page) Close() {
	p.closeOnce.Do(func() {
		close(p.closed)
		p.end()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if p.server.Shutdown(ctx) != nil {
			_ = p.server.Close()
		}
		p.served.Wait()
	})
}
