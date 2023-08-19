package institution

import (
	"context"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

const schwabLoginUrl = "https://workplace.schwab.com"
const schwabUrlPrefix = "https://www.schwabplan.com"

type schwab struct {
}

func init() {
	registerInstitution("schwab", schwab{})
}

func (s schwab) GetBalances(ctx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	ctx, cancel := newContext(ctx, schwabUrlPrefix)
	defer cancel()

	err := s.auth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if err != nil {
		return nil, err
	}

	var balance string
	err = chromedp.Run(ctx,
		chromedp.TextContent("#vestedAmount", &balance))
	if err != nil {
		return nil, err
	}
	balanceNum, err := parseCents(balance)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		mapping[0].Mapping: balanceNum,
	}, nil
}

func (s schwab) auth(ctx context.Context, username, password string) error {
	var iframes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(schwabLoginUrl),
		chromedp.Sleep(1*time.Second),
		chromedp.Nodes("div.bcn-panel__body iframe", &iframes, chromedp.ByQueryAll))
	if err != nil {
		return err
	}
	return chromedp.Run(ctx,
		chromedp.SetValue("#loginIdInput", username, chromedp.ByQuery, chromedp.FromNode(iframes[0])),
		chromedp.SetValue("#passwordInput", password, chromedp.ByQuery, chromedp.FromNode(iframes[0])),
		chromedp.Click("#btnLogin", chromedp.ByQuery, chromedp.FromNode(iframes[0])),
		chromedp.WaitVisible("#retirement-widget-container", chromedp.ByQuery))
}
