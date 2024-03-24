package institution

import (
	"context"
	"fmt"
	"time"

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

func (i igoe) GetBalances(parentCtx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	browserCtx, cancel := newContext(parentCtx, igoeUrlPrefix)
	defer cancel()
	err := i.auth(browserCtx, auth, d)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(browserCtx, 1*time.Minute)
	defer cancel()
	var nodes []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes(".b-dashboard-accounts-item", &nodes, chromedp.ByQueryAll))
	if err != nil {
		return nil, screenshotError(browserCtx, err)
	}
	return getMultipleBalances(nodes, browserCtx, mapping,
		".b-dashboard-accounts-name", ".b-dashboard-accounts-balance .currency-span")
}

func (i igoe) auth(parentCtx context.Context, auth Auth, d decrypt.Decryptor) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	var questions []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(igoeLoginUrl),
		chromedp.SetValue("#un", auth.Username, chromedp.ByQuery),
		chromedp.SetValue("#password", d.Decrypt(auth.EncryptedPassword), chromedp.ByQuery),
		chromedp.Click(".button-subm", chromedp.ByQuery),
		chromedp.WaitVisible("accounts-summary-mini,div.secq-form", chromedp.ByQuery),
		chromedp.Nodes("div.secq-form div.field-label label", &questions, chromedp.ByQueryAll, chromedp.AtLeast(0)))
	if err != nil {
		return screenshotError(parentCtx, err)
	}
	if len(questions) == 0 {
		return nil
	}
	for _, node := range questions {
		var question string
		err := chromedp.Run(ctx,
			chromedp.TextContent([]cdp.NodeID{node.NodeID}, &question, chromedp.ByNodeID))
		if err != nil {
			return screenshotError(parentCtx, err)
		}
		answer, ok := auth.Questions[question]
		if !ok {
			return screenshotError(parentCtx, fmt.Errorf("did not have answer to question: %s", question))
		}
		id := node.AttributeValue("for")
		err = chromedp.Run(ctx,
			chromedp.SetValue(fmt.Sprintf("#%s", id), answer, chromedp.ByQuery))
		if err != nil {
			return screenshotError(parentCtx, err)
		}
	}
	err = chromedp.Run(ctx,
		chromedp.Click(".button-subm", chromedp.ByQuery),
		chromedp.WaitVisible("accounts-summary-mini", chromedp.ByQuery))
	return screenshotError(parentCtx, err)
}
