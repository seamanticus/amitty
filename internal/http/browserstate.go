package http

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

// BrowserFileName is the file, next to the token, that keeps the browser's state between runs.
const BrowserFileName = "browser.json"

// browserState is the browser as kept in its file.
type browserState struct {
	Installation string        `json:"installation,omitzero"`
	Cookies      []savedCookie `json:"cookies"`
}

// savedCookie is a cookie and the host key of the jar it was set in.
type savedCookie struct {
	Host     string    `json:"host"`
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitzero"`
	Path     string    `json:"path,omitzero"`
	Expires  time.Time `json:"expires,omitzero"`
	Secure   bool      `json:"secure,omitzero"`
	HTTPOnly bool      `json:"http_only,omitzero"`
}

// OpenBrowser returns the browser kept at path, connecting through network, and keeps it there on every change. A missing file is a new browser; one that does not read is logged and replaced.
func OpenBrowser(network Network, path string) *Browser {
	b := NewBrowser(network)
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		slog.Warn("failed to read the browser state; starting a new one", "path", path, "err", err)
	default:
		if err := b.restore(data, time.Now()); err != nil {
			slog.Warn("failed to read the browser state; starting a new one", "path", path, "err", err)
		}
	}
	b.path = path
	b.jar = keptJar{CookieJar: b.jar, save: b.save}
	return b
}

// restore loads the state in data into b, leaving out cookies expired at now.
func (b *Browser) restore(data []byte, now time.Time) error {
	var state browserState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	hosts := map[string][]*fhttp.Cookie{}
	for _, c := range state.Cookies {
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		hosts[c.Host] = append(hosts[c.Host], &fhttp.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HttpOnly: c.HTTPOnly})
	}
	for _, host := range slices.Sorted(maps.Keys(hosts)) {
		b.jar.SetCookies(&url.URL{Scheme: "https", Host: host, Path: "/"}, hosts[host])
	}
	b.installation = state.Installation
	return nil
}

// state returns b as kept in its file, hosts sorted, leaving out cookies expired or deleted at now.
func (b *Browser) state(now time.Time) browserState {
	state := browserState{Installation: b.Installation(), Cookies: []savedCookie{}}
	all := b.jar.GetAllCookies()
	for _, host := range slices.Sorted(maps.Keys(all)) {
		for _, c := range all[host] {
			if !live(c, now) {
				continue
			}
			state.Cookies = append(state.Cookies, savedCookie{Host: host, Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, Secure: c.Secure, HTTPOnly: c.HttpOnly})
		}
	}
	return state
}

// save writes b to its file, readable by its owner alone; a failure is logged, as the next change tries again.
func (b *Browser) save() {
	b.saving.Lock()
	defer b.saving.Unlock()
	if err := writeState(b.path, b.state(time.Now())); err != nil {
		slog.Warn("failed to keep the browser state", "path", b.path, "err", err)
	}
}

func writeState(path string, state browserState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// CreateTemp makes the file 0600.
	tmp, err := os.CreateTemp(dir, ".browser-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", path, errors.Join(err, os.Remove(tmp.Name())))
	}
	return nil
}

// live reports whether c is still sent at now: not deleted by a Max-Age of 0 or less, and not past its expiry.
func live(c *fhttp.Cookie, now time.Time) bool {
	return c.MaxAge >= 0 && (c.Expires.IsZero() || c.Expires.After(now))
}

// keptJar is a jar that sends no expired cookie, stores Max-Age as an expiry, and calls save after each change.
type keptJar struct {
	tls_client.CookieJar
	save func()
}

func (j keptJar) SetCookies(u *url.URL, cookies []*fhttp.Cookie) {
	now := time.Now()
	for _, c := range cookies {
		// Max-Age wins over Expires, and is relative to now, which a later run cannot know.
		if c.MaxAge > 0 {
			c.Expires, c.MaxAge = now.Add(time.Duration(c.MaxAge)*time.Second).UTC(), 0
		}
	}
	j.CookieJar.SetCookies(u, cookies)
	j.save()
}

func (j keptJar) Cookies(u *url.URL) []*fhttp.Cookie {
	now := time.Now()
	return slices.DeleteFunc(j.CookieJar.Cookies(u), func(c *fhttp.Cookie) bool { return !live(c, now) })
}
