package institution

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

const igoeLoginUrl = "https://goigoe.wealthcareportal.com/Authentication/Handshake"
const igoeUrlPrefix = "https://www.schwabplan.com"

func init() {
	registerInstitution("igoe", igoe{})
}

type igoe struct {
}

func (i igoe) GetBalances(ctx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	ctx, cancel := newContext(ctx, igoeUrlPrefix)
	defer cancel()
	err := i.auth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if err != nil {
		return nil, err
	}

	return getMultipleBalances(func(nodes *[]*cdp.Node) error {
		return chromedp.Run(ctx,
			chromedp.Nodes(".b-dashboard-accounts-item", nodes, chromedp.ByQueryAll))
	}, ctx, mapping, ".b-dashboard-accounts-name", ".b-dashboard-accounts-balance .currency-span")
}

func (i igoe) auth(ctx context.Context, username, password string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(igoeLoginUrl),
		chromedp.SetValue("#un", username, chromedp.ByQuery),
		chromedp.SetValue("#password", password, chromedp.ByQuery),
		chromedp.Click(".button-subm", chromedp.ByQuery),
		chromedp.WaitVisible("accounts-summary-mini", chromedp.ByQuery),
	)
}
