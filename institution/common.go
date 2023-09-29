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
}

// AccountMapping contains the account name in the institution and the mapping to the YNAB account name.
type AccountMapping struct {
	Name    string
	Mapping string
}

// An Institution gets the balances for the given slice of [AccountMapping].
type Institution interface {
	GetBalances(context.Context, Auth, decrypt.Decryptor, []AccountMapping) (map[string]int64, error)
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
	}
	// no targets or none match urlPrefix.
	ctx, cancel1 = chromedp.NewContext(ctx)
	ctx, cancel2 := context.WithTimeout(ctx, 1*time.Minute)
	return ctx, func() {
		cancel2()
		cancel1()
	}
}

// getMultipleBalances is a utility function used by an Institution to retrieve multiple balances from
// a single page. The Institution provides a function to get the nodes containing the account name and balance,
// and selectors for the name and balance inside each node.
func getMultipleBalances(getNodes func(*[]*cdp.Node) error, ctx context.Context, mapping []AccountMapping,
	nameSelector, balSelector string) (map[string]int64, error) {
	var nodes []*cdp.Node
	if err := getNodes(&nodes); err != nil {
		return nil, err
	}
	balances := make(map[string]int64)
	for _, node := range nodes {
		var name, balance string
		err := chromedp.Run(ctx,
			chromedp.TextContent(nameSelector, &name, chromedp.ByQuery, chromedp.FromNode(node)),
			chromedp.TextContent(balSelector, &balance, chromedp.ByQuery, chromedp.FromNode(node)))
		if err != nil {
			fmt.Printf("Failed to find account name and balance: %v\n", err)
			debug.PrintStack()
			continue
		}
		trimmedName := strings.TrimSpace(name)
		mappingIndex := slices.IndexFunc(mapping, func(mapping AccountMapping) bool {
			return mapping.Name == trimmedName
		})
		if mappingIndex != -1 {
			balanceNum, err := parseCents(balance)
			if err != nil {
				fmt.Printf("Failed to parse balance '%s': %v\n", balance, err)
				continue
			}
			balances[mapping[mappingIndex].Mapping] = balanceNum
		}
	}
	return balances, nil
}

var centsPattern = regexp.MustCompile(`\D`)

// parseCents parses a string for a balance, removing all non-numeric characters and parsing to int64.
func parseCents(str string) (int64, error) {
	numsOnly := centsPattern.ReplaceAllString(str, "")
	return strconv.ParseInt(numsOnly, 10, 64)
}
