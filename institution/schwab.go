package institution

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func (s schwab) RequestCode(ctx context.Context, auth Auth, d decrypt.Decryptor) (context.Context, context.CancelFunc, error) {
	// begin login process
	doCancel := true
	ctx, cancel := newContext(ctx, schwabUrlPrefix)
	defer func() {
		if doCancel {
			cancel()
		}
	}()
	result, screenshot := s.startAuth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	// ensure it asks for security code
	switch result {
	case LoginError:
		return nil, nil, screenshot
	case LoginOk:
		return nil, nil, errors.New("login successful, no security code needed")
	case CodeRequired:
		// trigger security code send
		numbers, nodeIds, err := s.getNumbers(ctx)
		if err != nil {
			return nil, nil, err
		}
		fmt.Println("Found SMS numbers:")
		for i, number := range numbers {
			fmt.Printf("%d: %s\n", i+1, number)
		}
		chosen := UserInput("Choose one: ")
		index, err := strconv.ParseUint(chosen, 10, 8)
		if err != nil {
			return nil, nil, err
		}
		if int(index-1) > len(numbers) {
			return nil, nil, errors.New("index out of range")
		}
		err = s.sendCode(ctx, nodeIds[index-1])
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

func (s schwab) EnterCode(parentCtx context.Context, code string) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	// enter code into existing browser
	err := chromedp.Run(ctx,
		chromedp.SetValue("#securityCode", code),
		chromedp.Click("#checkbox-remember-device"),
		chromedp.Click("#continueButton", chromedp.ByID),
		chromedp.WaitVisible("#retirement-widget-container"))
	return screenshotError(parentCtx, err)
}

func (s schwab) GetBalances(ctx context.Context, auth Auth, d decrypt.Decryptor, mapping []AccountMapping) (map[string]int64, error) {
	ctx, cancel := newContext(ctx, schwabUrlPrefix)
	defer cancel()
	result, screenshot := s.startAuth(ctx, auth.Username, d.Decrypt(auth.EncryptedPassword))
	if result != LoginOk {
		return nil, screenshot
	}
	var balance string
	err := chromedp.Run(ctx,
		chromedp.TextContent("#vestedAmount", &balance))
	if err != nil {
		return nil, screenshotError(ctx, err)
	}
	balanceNum, err := parseCents(balance)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		mapping[0].Mapping: balanceNum,
	}, nil
}

func (s schwab) startAuth(parentCtx context.Context, username, password string) (LoginResult, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cancel()
	errs := &MultiError{}
	var iframes []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Navigate(schwabLoginUrl),
		chromedp.Sleep(1*time.Second))
	errs.AddError(screenshotError(parentCtx, errors.New("screenshot1")))
	if err != nil {
		errs.AddError(err)
		return LoginError, errs
	}
	err = chromedp.Run(ctx, chromedp.Nodes("div.bcn-panel__body iframe", &iframes, chromedp.ByQueryAll))
	errs.AddError(screenshotError(parentCtx, errors.New("screenshot2")))
	if err != nil {
		errs.AddError(err)
		return LoginError, errs
	}
	var sms []*cdp.Node
	err = chromedp.Run(ctx,
		chromedp.SetValue("#loginIdInput", username, chromedp.ByQuery, chromedp.FromNode(iframes[0])),
		chromedp.SetValue("#passwordInput", password, chromedp.ByQuery, chromedp.FromNode(iframes[0])))
	errs.AddError(screenshotError(parentCtx, errors.New("screenshot3")))
	if err != nil {
		errs.AddError(err)
		return LoginError, errs
	}
	err = chromedp.Run(ctx, chromedp.Click("#btnLogin", chromedp.ByQuery, chromedp.FromNode(iframes[0])),
		chromedp.WaitVisible("#retirement-widget-container,#otp_sms", chromedp.ByQuery),
		chromedp.Nodes("#otp_sms", &sms, chromedp.AtLeast(0)))
	if err != nil {
		return LoginError, screenshotError(parentCtx, err)
	}
	if len(sms) == 0 {
		return LoginOk, errors.New("login ok")
	}
	return CodeRequired, screenshotError(parentCtx, errors.New("code required"))
}

func (s schwab) getNumbers(parentCtx context.Context) ([]string, []cdp.NodeID, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	var radios []*cdp.Node
	err := chromedp.Run(ctx,
		chromedp.Click("#otp_sms"),
		chromedp.Nodes("#targetInputs label", &radios, chromedp.NodeVisible))
	if err != nil {
		return nil, nil, screenshotError(parentCtx, err)
	}
	numbers := make([]string, len(radios))
	ids := make([]cdp.NodeID, len(radios))
	for i, node := range radios {
		var number string
		if err = chromedp.Run(ctx, chromedp.TextContent([]cdp.NodeID{node.NodeID}, &number, chromedp.ByNodeID)); err != nil {
			return nil, nil, screenshotError(parentCtx, err)
		}
		numbers[i] = number
		ids[i] = node.NodeID
	}
	return numbers, ids, nil
}

func (s schwab) sendCode(parentCtx context.Context, nodeId cdp.NodeID) error {
	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()
	err := chromedp.Run(ctx,
		chromedp.Click([]cdp.NodeID{nodeId}, chromedp.ByNodeID),
		chromedp.Click("#btnContinue", chromedp.ByID))
	return screenshotError(parentCtx, err)
}
