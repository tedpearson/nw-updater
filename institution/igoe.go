package institution

import (
	"context"
	"fmt"

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
	err := i.auth(ctx, auth, d)
	if err != nil {
		return nil, err
	}

	return getMultipleBalances(func(nodes *[]*cdp.Node) error {
		return chromedp.Run(ctx,
			chromedp.Nodes(".b-dashboard-accounts-item", nodes, chromedp.ByQueryAll))
	}, ctx, mapping, ".b-dashboard-accounts-name", ".b-dashboard-accounts-balance .currency-span")
}

func (i igoe) auth(ctx context.Context, auth Auth, d decrypt.Decryptor) error {

	var questions []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(igoeLoginUrl),
		chromedp.SetValue("#un", auth.Username, chromedp.ByQuery),
		chromedp.SetValue("#password", d.Decrypt(auth.EncryptedPassword), chromedp.ByQuery),
		chromedp.Click(".button-subm", chromedp.ByQuery),
		chromedp.WaitVisible("accounts-summary-mini,div.secq-form", chromedp.ByQuery),
		chromedp.Nodes("div.secq-form div.field-label label", &questions, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	)
	if err != nil {
		return err
	}
	if len(questions) == 0 {
		return nil
	}
	for _, node := range questions {
		var question string
		err := chromedp.Run(ctx,
			chromedp.TextContent([]cdp.NodeID{node.NodeID}, &question, chromedp.ByNodeID))
		if err != nil {
			return err
		}
		answer, ok := auth.Questions[question]
		if !ok {
			return fmt.Errorf("did not have answer to question: %s", question)
		}
		id := node.AttributeValue("for")
		err = chromedp.Run(ctx,
			chromedp.SetValue(fmt.Sprintf("#%s", id), answer, chromedp.ByQuery))
		if err != nil {
			return err
		}
	}
	return chromedp.Run(ctx,
		chromedp.Click(".button-subm", chromedp.ByQuery),
		chromedp.WaitVisible("accounts-summary-mini", chromedp.ByQuery))
}
