// Package session abstracts around the REST API and the Gateway, managing both
// at once. It offers a handler interface similar to that in discordgo for
// Gateway events.
package session

import (
	"context"
	"errors"
	"sync"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/gateway"
	"github.com/seamanticus/amitty/internal/dmwire/utils/handler"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws"
	"github.com/seamanticus/amitty/internal/dmwire/utils/ws/ophandler"
)

// ErrMFA is returned if the account requires a 2FA code to log in.
var ErrMFA = errors.New("account has 2FA enabled")

// ErrClosed is returned if the Session is closed, either because it's already
// closed (and Close is being called again) or it was never started.
var ErrClosed = errors.New("Session is closed")

// Session manages both the API and Gateway. As such, Session inherits all of
// API's methods, as well has the Handler used for Gateway.
type Session struct {
	*api.Client
	*handler.Handler

	// internal state to not be copied around.
	state *sessionState

	// DontWaitForReady makes Open return after the first event (usually HELLO).
	DontWaitForReady bool // false
}

type sessionState struct {
	sync.Mutex
	id      gateway.Identifier
	gateway *gateway.Gateway

	ctx    context.Context
	cancel context.CancelFunc
	doneCh <-chan struct{}
}

// NewWithGateway constructs a bare Session from the given UNOPENED gateway and the API client of the same account.
func NewWithGateway(g *gateway.Gateway, client *api.Client, h *handler.Handler) *Session {
	return newCustom(g.State().Identifier, client, h, g)
}

// NewCustom constructs a bare Session from the given parameters.
func NewCustom(id gateway.Identifier, cl *api.Client, h *handler.Handler) *Session {
	return newCustom(id, cl, h, nil)
}

func newCustom(
	id gateway.Identifier,
	cl *api.Client,
	h *handler.Handler,
	g *gateway.Gateway) *Session {

	s := &Session{
		Client:  cl,
		Handler: h,
		state: &sessionState{
			gateway: g,
			id:      id,
		},
	}
	h.Subscribe(func(ev ws.Event) {
		if ready, ok := ev.(*gateway.ReadyEvent); ok && ready.AuthToken != "" {
			s.Client.Session.Token = ready.AuthToken
		}
	})
	return s
}

// AddIntents adds the given intents into the gateway. Calling it after Open has
// already been called will result in a panic.
func (s *Session) AddIntents(intents gateway.Intents) {
	s.state.Lock()

	s.state.id.AddIntents(intents)

	if s.state.gateway != nil {
		s.state.gateway.AddIntents(intents)
	}

	s.state.Unlock()
}

// HasIntents reports if the Gateway has the passed Intents.
//
// If no intents are set, e.g. if using a user account, HasIntents will always
// return true.
func (s *Session) HasIntents(intents gateway.Intents) bool {
	return s.state.id.HasIntents(intents)
}

// Gateway returns the current session's gateway. If Open has never been called
// or Session was never constructed with a gateway, then nil is returned.
func (s *Session) Gateway() *gateway.Gateway {
	s.state.Lock()
	defer s.state.Unlock()

	return s.state.gateway
}

// GatewayOpts returns a copy of the current session's gateway options. If Open
// has never been called or Session was never constructed with a gateway, then
// the default gateway options are returned.
func (s *Session) GatewayOpts() *ws.GatewayOpts {
	s.state.Lock()
	defer s.state.Unlock()

	opts := &gateway.DefaultGatewayOpts
	if s.state.gateway != nil {
		opts = s.state.gateway.Opts()
	}

	return opts
}

// GatewayError returns the gateway's error if the gateway is dead. If it's not
// dead, then nil is always returned. The check is done with GatewayIsAlive().
// If the gateway has never been started, nil will be returned (even though
// GatewayIsAlive would've returned true).
//
// This method would return what Close() would've returned if a fatal gateway
// error was found.
func (s *Session) GatewayError() error {
	s.state.Lock()
	defer s.state.Unlock()

	if !s.gatewayIsAlive() && s.state.gateway != nil {
		return s.state.gateway.LastError()
	}

	return nil
}

// GatewayIsAlive returns true if the gateway is still alive, that is, it is
// either connected or is trying to reconnect after an interruption. In other
// words, false is returned if the gateway isn't open or it has exited after
// seeing a fatal error code (and therefore cannot recover).
func (s *Session) GatewayIsAlive() bool {
	s.state.Lock()
	defer s.state.Unlock()

	return s.gatewayIsAlive()
}

func (s *Session) gatewayIsAlive() bool {
	if s.state.gateway == nil || s.state.doneCh == nil {
		return false
	}

	select {
	case <-s.state.doneCh:
		return false
	default:
		return true
	}
}

// Connect opens the Discord gateway and waits until an unrecoverable error
// occurs. Always prefer this method over Open. Note that Connect will return
// when ctx is done or when s.Close is called.
//
// As an odd case, when ctx is done and if the gateway is already finished
// connecting, then a nil error will be returned (unless the gateway has an
// error). This is contrary to the common behavior of a ctx function returning
// ctx.Err().
func (s *Session) Connect(ctx context.Context) error {
	opts := s.GatewayOpts()

	for {
		if err := s.Open(ctx); err != nil {
			if opts.ErrorIsFatalClose(err) || ctx.Err() != nil {
				// Fatal error or context is done, return.
				return err
			}
			// Non-fatal error, retry.
			continue
		}

		if err := s.Wait(ctx); err != nil {
			if opts.ErrorIsFatalClose(err) {
				// Gateway returned a fatal error, so we can't recover.
				return err
			}
			if ctx.Err() != nil {
				// Context was done, so we can't recover. Exit with no error,
				// since we're just waiting.
				return nil
			}
			// Non-fatal error, retry.
		}
	}
}

// Open opens the Discord gateway and its handler, then waits until either the
// Ready or Resumed event gets through. Prefer using Connect instead of Open.
func (s *Session) Open(ctx context.Context) error {
	evCh := make(chan ws.Event)

	s.state.Lock()
	defer s.state.Unlock()

	if s.state.cancel != nil {
		if err := s.close(); err != nil {
			return err
		}
	}

	// The gateway is the caller's, so it connects as the caller chose.
	if s.state.gateway == nil {
		return errors.New("session has no gateway to open")
	}

	// Make a context that's stored in state so this can be used throughout.
	s.state.ctx, s.state.cancel = context.WithCancel(context.Background())

	// Events reach evCh until Open returns; sends still pending then are dropped.
	waiting, stopWaiting := context.WithCancel(ctx)
	defer stopWaiting()
	unsubscribe := s.Subscribe(handler.Send(waiting, evCh))
	defer unsubscribe()

	opCh := s.state.gateway.Connect(s.state.ctx)
	s.state.doneCh = ophandler.Loop(opCh, s.Handler)

	for {
		select {
		case <-ctx.Done():
			s.close()
			return ctx.Err()

		case <-s.state.ctx.Done():
			s.close()
			return s.state.ctx.Err()

		case <-s.state.doneCh:
			// Event loop died.
			return s.state.gateway.LastError()

		case ev := <-evCh:
			if s.DontWaitForReady {
				return nil
			}

			switch ev.(type) {
			case *gateway.ReadyEvent:
				if s.state.id.Capabilities&gateway.PrioritizedReadyPayload == 0 {
					return nil
				}
			case *gateway.ReadySupplementalEvent, *gateway.ResumedEvent:
				return nil
			}
		}
	}
}

// Wait blocks until either ctx is done or the gateway stumbles on an
// unrecoverable error.
func (s *Session) Wait(ctx context.Context) error {
	s.state.Lock()
	doneCh := s.state.doneCh
	s.state.Unlock()

	if doneCh == nil {
		return ErrClosed
	}

	for {
		select {
		case <-ctx.Done():
			s.Close()
			// Prefer gateway errors over context errors.
			if err := s.GatewayError(); err != nil {
				return err
			}
			return ctx.Err()

		case <-doneCh:
			// Event loop died.
			return s.GatewayError()
		}
	}
}

// WithContext returns a shallow copy of Session with the context replaced in
// the API client. All methods called on the returned Session will use this
// given context.
//
// This method is thread-safe only after Open and before Close are called. Open
// and Close should not be called on the returned Session.
func (s *Session) WithContext(ctx context.Context) *Session {
	cpy := *s
	cpy.Client = s.Client.WithContext(ctx)
	return &cpy
}

// SendGateway is a helper to send messages over the gateway. It will check
// if the gateway is open and available, then send the message.
func (s *Session) SendGateway(ctx context.Context, m ws.Event) error {
	// The only necessary check here is checking if gateway is nil, however
	// this will save us a bit of work in serialization.
	if !s.GatewayIsAlive() {
		return ErrClosed
	}

	return s.Gateway().Send(ctx, m)
}

// Close closes the underlying Websocket connection, invalidating the session
// ID. It will send a closing frame before ending the connection, closing it
// gracefully. This will cause the bot to appear as offline instantly. To
// prevent this behavior, change Gateway.AlwaysCloseGracefully.
func (s *Session) Close() error {
	s.state.Lock()
	defer s.state.Unlock()

	return s.close()
}

func (s *Session) close() error {
	if s.state.cancel == nil {
		return ErrClosed
	}

	s.state.cancel()
	s.state.cancel = nil
	s.state.ctx = nil

	<-s.state.doneCh
	s.state.doneCh = nil

	return s.state.gateway.LastError()
}
