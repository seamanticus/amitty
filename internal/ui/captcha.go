package ui

import (
	"context"
	"errors"
	"fmt"

	"github.com/seamanticus/amitty/internal/captcha"
	"github.com/seamanticus/amitty/internal/tui"
	"github.com/seamanticus/amitty/internal/tui/cmd"
)

// Captcha answers the hCaptcha challenges Discord sends for one request at a time, each on a page opened in the browser. K names the request, so the steps of an ended or another request are dropped. The zero value has no request.
type Captcha[K comparable] struct {
	active bool
	key    K
	// tries counts the challenges served for the request.
	tries int
	// page waits for an answer, or is nil; openErr is why no browser opened it.
	page    *captcha.Page
	openErr string
	// solution is the last answer, sent with the request until Discord takes or challenges it again.
	solution captcha.Solution
}

// captchaStep is what a CaptchaMsg reports. The zero message serves no page, so it is dropped.
type captchaStep uint8

const (
	captchaServed captchaStep = iota
	captchaChallenged
	captchaOpenFailed
	captchaAnswered
	captchaUnanswered
	captchaStepCount
)

// CaptchaMsg is a step of the captcha of a request named by a K.
type CaptchaMsg[K comparable] struct {
	key       K
	step      captchaStep
	challenge captcha.Challenge
	page      *captcha.Page
	solution  captcha.Solution
	err       error
}

// CaptchaDoneMsg ends the captcha of request Key. A nil Err means the user answered: send the request again with Captcha.Solution.
type CaptchaDoneMsg[K comparable] struct {
	Key K
	Err error
}

// Challenged says Discord sent challenge in answer to request key.
func Challenged[K comparable](key K, challenge captcha.Challenge) CaptchaMsg[K] {
	return CaptchaMsg[K]{key: key, step: captchaChallenged, challenge: challenge}
}

// captchaRefusedError says every try was answered and Discord still challenged the request.
type captchaRefusedError struct{}

func (captchaRefusedError) Error() string {
	return fmt.Sprintf("Discord did not accept the captcha after %d tries", captcha.MaxTries)
}

// IsCaptchaRefused reports whether err ended a captcha because Discord challenged its request after the last try.
func IsCaptchaRefused(err error) bool {
	_, ok := errors.AsType[captchaRefusedError](err)
	return ok
}

// Start begins the captcha of request key with no tries and no answer, closing the page of any earlier request.
func (c Captcha[K]) Start(key K) (Captcha[K], tui.Cmd) {
	return Captcha[K]{active: true, key: key}, closePage(c.page)
}

// Stop ends the request, closing its page.
func (c Captcha[K]) Stop() (Captcha[K], tui.Cmd) {
	return Captcha[K]{}, closePage(c.page)
}

// Cancel closes the waiting page; the wait for it then fails, which ends the captcha with an error.
func (c Captcha[K]) Cancel() tui.Cmd { return closePage(c.page) }

// Waiting reports whether a page waits for an answer.
func (c Captcha[K]) Waiting() bool { return c.page != nil }

// Solution is the last answer to send the request with; its zero value is no answer.
func (c Captcha[K]) Solution() captcha.Solution { return c.solution }

// Prompt tells the user how to answer the waiting page, or is empty when none waits.
func (c Captcha[K]) Prompt() string {
	if c.page == nil {
		return ""
	}
	return c.page.Prompt(c.tries, c.openErr)
}

// Update steps the captcha with msg. Steps of an ended or another request are dropped and their page closed.
func (c Captcha[K]) Update(msg CaptchaMsg[K]) (Captcha[K], tui.Cmd) {
	if !c.active || msg.key != c.key {
		return c, closePage(msg.page)
	}
	switch msg.step {
	case captchaServed:
		if msg.page == nil || c.page != nil {
			return c, closePage(msg.page)
		}
		c.page, c.openErr = msg.page, ""
		return c, cmd.Batch{openCaptcha(c.key, msg.page), waitCaptcha(c.key, msg.page)}
	case captchaChallenged:
		if c.tries >= captcha.MaxTries {
			key := c.key
			stopped, closing := c.Stop()
			return stopped, cmd.Batch{closing, captchaDone(key, captchaRefusedError{})}
		}
		c.tries++
		return c, serveCaptcha(c.key, msg.challenge)
	case captchaOpenFailed:
		if msg.page == c.page && c.page != nil {
			c.openErr = msg.err.Error()
		}
		return c, nil
	case captchaAnswered:
		if msg.page != c.page {
			return c, nil
		}
		c.page, c.openErr, c.solution = nil, "", msg.solution
		return c, captchaDone[K](c.key, nil)
	case captchaUnanswered:
		if msg.page != c.page {
			return c, nil
		}
		return Captcha[K]{}, captchaDone(c.key, msg.err)
	}
	panic("invariant: unknown captcha step")
}

// serveCaptcha puts challenge on a page; a page that cannot be served ends the captcha.
func serveCaptcha[K comparable](key K, challenge captcha.Challenge) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		page, err := captcha.Serve(context.Background(), challenge)
		if err != nil {
			return CaptchaMsg[K]{key: key, step: captchaUnanswered, err: err}
		}
		return CaptchaMsg[K]{key: key, step: captchaServed, page: page}
	})
}

// openCaptcha opens page in the browser. It can block until a browser it started exits, so it runs apart from waitCaptcha.
func openCaptcha[K comparable](key K, page *captcha.Page) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		if err := OpenExternal(context.Background(), page.URL()); err != nil {
			return CaptchaMsg[K]{key: key, step: captchaOpenFailed, page: page, err: err}
		}
		return nil
	})
}

// waitCaptcha waits for the answer to page, which closes itself after captcha.AnswerTimeout.
func waitCaptcha[K comparable](key K, page *captcha.Page) tui.Cmd {
	return cmd.Task(func() tui.Msg {
		solution, err := page.Wait(context.Background())
		if err != nil {
			return CaptchaMsg[K]{key: key, step: captchaUnanswered, page: page, err: err}
		}
		return CaptchaMsg[K]{key: key, step: captchaAnswered, page: page, solution: solution}
	})
}

// closePage closes page, if any, which ends its waitCaptcha with an error.
func closePage(page *captcha.Page) tui.Cmd {
	if page == nil {
		return nil
	}
	return cmd.Task(func() tui.Msg {
		page.Close()
		return nil
	})
}

func captchaDone[K comparable](key K, err error) tui.Cmd {
	return cmd.Task(func() tui.Msg { return CaptchaDoneMsg[K]{Key: key, Err: err} })
}
