/*
Package institution contains types that get balances for various institutions like banks, retirement programs, etc.
*/
package institution

import (
	"context"
	"fmt"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

// Auth contains authentication information for an institution.
type Auth struct {
	Username          string
	EncryptedPassword string `yaml:"encrypted_password"`
	Questions         map[string]string
}

// AccountMapping contains the account name in the institution and the mapping to the YNAB account name.
type AccountMapping struct {
	Name    string
	Mapping string
}

type LoginResult uint8

const (
	LoginError LoginResult = iota
	LoginOk
	CodeRequired
)

// An Institution gets the balances for the given slice of [AccountMapping].
type Institution interface {
	GetBalances(context.Context, Auth, decrypt.Decryptor, []AccountMapping) (map[string]int64, error)
}

type SecurityCode interface {
	RequestCode(ctx context.Context, auth Auth, d decrypt.Decryptor) (context.Context, context.CancelFunc, error)
	EnterCode(ctx context.Context, code string) error
	Institution
}

type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	return m.Errors[0].Error()
}

func (m *MultiError) AddError(e error) {
	m.Errors = append(m.Errors, e)
}

func (m *MultiError) IsEmpty() bool {
	return len(m.Errors) == 0
}

type Error struct {
	Wrapped    error
	Screenshot []byte
	Stacktrace []byte
}

func (e Error) Error() string {
	return e.Wrapped.Error()
}

func (e Error) Unwrap() error {
	return e.Wrapped
}

var institutions = make(map[string]Institution)

// Each Institution should register itself with this function in an init() method so that it can be looked up by name.
func registerInstitution(name string, institution Institution) {
	institutions[name] = institution
}

// MustGet looks up an institution by name, panicking if none can be found.
func MustGet(name string) Institution {
	institution, ok := institutions[name]
	if !ok {
		panic(fmt.Errorf("invalid institution '%s'", name))
	}
	return institution
}

// newContext creates a new chromedp context for each institution, using an existing tab if the urlPrefix matches
// an open one, which is only currently used when debugging with an existing chrome instance with a websocket.
func newContext(ctx context.Context, urlPrefix string) (context.Context, context.CancelFunc) {
	// get the list of the targets
	infos, err := chromedp.Targets(ctx)
	if err != nil {
		panic(err)
	}
	i := slices.IndexFunc(infos, func(info *target.Info) bool {
		return strings.HasPrefix(info.URL, urlPrefix)
	})
	var cancel1 context.CancelFunc
	if i != -1 {
		ctx, cancel1 = chromedp.NewContext(ctx, chromedp.WithTargetID(infos[i].TargetID))
	} else {
		ctx, cancel1 = chromedp.NewContext(ctx)
	}
	ctx, cancel2 := context.WithTimeout(ctx, 5*time.Minute)
	// create tab so we can take a screenshot later on this context.
	if err = chromedp.Run(ctx); err != nil {
		panic(err)
	}
	return ctx, func() {
		cancel2()
		cancel1()
	}
}

func screenshotError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var buf []byte
	fmt.Printf("Error: %s\n", err)
	stack := debug.Stack()
	fmt.Println(string(stack))
	err2 := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 100))
	if err2 != nil {
		fmt.Printf("Failed to take screenshot of error: %s\n", err2)
	}
	return Error{
		Wrapped:    err,
		Screenshot: buf,
		Stacktrace: stack,
	}
}

// getMultipleBalances is a utility function used by an Institution to retrieve multiple balances from
// a single page. The Institution provides the nodes containing the account name and balance,
// and selectors for the name and balance inside each node.
func getMultipleBalances(nodes []*cdp.Node, parentCtx context.Context, mapping []AccountMapping, nameSelector,
	balSelector string) (map[string]int64, error) {

	ctx, cancel := context.WithTimeout(parentCtx, 1*time.Minute)
	defer cancel()

	balances := make(map[string]int64)
	errs := &MultiError{}
	for _, node := range nodes {
		var name, balance string
		err := chromedp.Run(ctx,
			chromedp.TextContent(nameSelector, &name, chromedp.ByQuery, chromedp.FromNode(node)),
			chromedp.TextContent(balSelector, &balance, chromedp.ByQuery, chromedp.FromNode(node)))
		if err != nil {
			err = fmt.Errorf("failed to find account name and balance: %w", err)
			errs.AddError(screenshotError(parentCtx, err))
			continue
		}
		trimmedName := strings.TrimSpace(name)
		mappingIndex := slices.IndexFunc(mapping, func(mapping AccountMapping) bool {
			return mapping.Name == trimmedName
		})
		if mappingIndex != -1 {
			balanceNum, err := parseCents(balance)
			if err != nil {
				err = fmt.Errorf("failed to parse balance '%s': %w", balance, err)
				errs.AddError(screenshotError(parentCtx, err))
				continue
			}
			balances[mapping[mappingIndex].Mapping] = balanceNum
		}
	}
	if errs.IsEmpty() {
		return balances, nil
	}
	return balances, errs
}

var centsPattern = regexp.MustCompile(`\D`)

// parseCents parses a string for a balance, removing all non-numeric characters and parsing to int64.
func parseCents(str string) (int64, error) {
	numsOnly := centsPattern.ReplaceAllString(str, "")
	return strconv.ParseInt(numsOnly, 10, 64)
}

func UserInput(prompt string) string {
	fmt.Print(prompt)
	// ring bell
	fmt.Print("\a")
	var line string
	_, err := fmt.Scanln(&line)
	if err != nil {
		panic(err)
	}
	fmt.Println()
	return line
}
