package institution

import (
	"context"
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

func init() {
	registerInstitution("fidelity", fidelity{})
}

func (f fidelity) GetBalances(parentCtx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	browserCtx, cancel := newContext(parentCtx, fidelityUrlPrefix)
	defer cancel()
	err := f.auth(browserCtx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(browserCtx, 1*time.Minute)
	defer cancel()
	var nodes []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes(".acct-selector__acct-content", &nodes, chromedp.ByQueryAll))
	if err != nil {
		return nil, screenshotError(browserCtx, err)
	}
	return getMultipleBalances(nodes, browserCtx, mapping,
		".acct-selector__acct-name", ".acct-selector__acct-balance span:not(.sr-only)")
}

func (f fidelity) auth(parentCtx context.Context, username, password string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	err := chromedp.Run(ctx,
		chromedp.Navigate(fidelityLoginUrl),
		chromedp.SetValue("#dom-username-input", username),
		chromedp.SetValue("#dom-pswd-input", password),
		chromedp.Click("#dom-login-button"),
		chromedp.WaitReady(".acct-selector__acct-list"))
	return screenshotError(parentCtx, err)
}
