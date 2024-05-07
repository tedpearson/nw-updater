package institution

import (
	"context"
	"errors"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

const (
	fidelityLoginUrl  = "https://digital.fidelity.com/prgw/digital/login/full-page"
	fidelityUrlPrefix = "https://digital.fidelity.com"
)

type fidelity struct {
}

func (f fidelity) RequestCode(ctx context.Context, auth Auth, d decrypt.Decryptor) (context.Context, context.CancelFunc, error) {
	// begin login process
	doCancel := true
	ctx, cancel := newContext(ctx, fidelityUrlPrefix)
	defer func() {
		if doCancel {
			cancel()
		}
	}()
	result, err := f.startAuth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	// ensure it asks for security code
	switch result {
	case LoginError:
		return nil, nil, err
	case LoginOk:
		return nil, nil, errors.New("login successful, no security code needed")
	case CodeRequired:
		err := f.sendCode(ctx)
		if err != nil {
			return nil, nil, err
		}
		// keep browser open! don't close context
		doCancel = false
		return ctx, cancel, nil
	default:
		return nil, nil, errors.New("unhandled auth result")
	}
}

func (f fidelity) EnterCode(parentCtx context.Context, code string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	// enter code into existing browser
	err := chromedp.Run(ctx,
		chromedp.SetValue("#dom-otp-code-input", code),
		chromedp.Click("#dom-trust-device-checkbox"),
		chromedp.Click("#dom-otp-code-submit-button"),
		chromedp.WaitVisible(".acct-selector__acct-list"))
	return screenshotError(parentCtx, err)
}

func init() {
	registerInstitution("fidelity", fidelity{})
}

func (f fidelity) GetBalances(parentCtx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	browserCtx, cancel := newContext(parentCtx, fidelityUrlPrefix)
	defer cancel()
	result, screenshot := f.startAuth(browserCtx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if result != LoginOk {
		return nil, screenshot
	}
	ctx, cancel := context.WithTimeout(browserCtx, 1*time.Minute)
	defer cancel()
	var nodes []*cdp.Node
	err := chromedp.Run(ctx, chromedp.Nodes(".acct-selector__acct-content", &nodes, chromedp.ByQueryAll))
	if err != nil {
		return nil, screenshotError(browserCtx, err)
	}
	return getMultipleBalances(nodes, browserCtx, mapping,
		".acct-selector__acct-name", ".acct-selector__acct-balance span:not(.sr-only)")
}

func (f fidelity) startAuth(parentCtx context.Context, username, password string) (LoginResult, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	var accountNodes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(fidelityLoginUrl),
		chromedp.SetValue("#dom-username-input", username),
		chromedp.SetValue("#dom-pswd-input", password),
		chromedp.Click("#dom-login-button"),
		chromedp.WaitVisible("//*[contains(@class,\"acct-selector__acct-list\")] | //h1[contains(.,\"To verify it's you\")]"),
		chromedp.Nodes(".acct-selector__acct-list", &accountNodes, chromedp.AtLeast(0)))
	if err != nil {
		return LoginError, screenshotError(parentCtx, err)
	}
	if len(accountNodes) == 0 {
		return CodeRequired, screenshotError(parentCtx, errors.New("code required"))
	}
	return LoginOk, errors.New("login ok")
}

func (f fidelity) sendCode(parentCtx context.Context) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	err := chromedp.Run(ctx,
		chromedp.Click("button.pvd-button--primary"))
	return screenshotError(parentCtx, err)
}
