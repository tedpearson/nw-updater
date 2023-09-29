package institution

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

const (
	nbLoginUrl  = "https://nb.fidelity.com/public/nb/default/home"
	nbPrefixUrl = "https://workplaceservices.fidelity.com/"
)

type netbenefits struct {
}

func init() {
	registerInstitution("netbenefits", netbenefits{})
}

func (n netbenefits) GetBalances(ctx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	ctx, cancel := newContext(ctx, nbPrefixUrl)
	defer cancel()

	err := n.auth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if err != nil {
		return nil, err
	}
	return getMultipleBalances(func(nodes *[]*cdp.Node) error {
		return chromedp.Run(ctx,
			chromedp.Click(`div[context="client-employer"] a`, chromedp.ByQuery),
			chromedp.WaitVisible(".ui-model-popup", chromedp.ByQuery),
			chromedp.Nodes(".ui-model-popup .fidgrid--row", nodes, chromedp.ByQueryAll))
	}, ctx, mapping, ".ui-accounts-list--account-title", ".ui-accounts-list--account-balance--sm")
}

func (n netbenefits) auth(ctx context.Context, username, password string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(nbLoginUrl),
		chromedp.SetValue("#dom-username-input", username),
		chromedp.SetValue("#dom-pswd-input", password),
		chromedp.Click("#dom-login-button"))
}
