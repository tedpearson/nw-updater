package institution

import (
	"context"

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

func (f fidelity) GetBalances(ctx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	ctx, cancel := newContext(ctx, fidelityUrlPrefix)
	defer cancel()
	_ = cancel
	err := f.auth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if err != nil {
		return nil, err
	}

	return getMultipleBalances(func(nodes *[]*cdp.Node) error {
		return chromedp.Run(ctx,
			chromedp.Nodes(".acct-selector__acct-content", nodes, chromedp.ByQueryAll))
	}, ctx, mapping, ".acct-selector__acct-name", ".acct-selector__acct-balance span:not(.sr-only)")
}

func (f fidelity) auth(ctx context.Context, username, password string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(fidelityLoginUrl),
		chromedp.SetValue("#userId-input", username),
		chromedp.SetValue("#password", password),
		chromedp.Click("#fs-login-button"),
		chromedp.WaitVisible(".acct-selector__acct-list"))
}
