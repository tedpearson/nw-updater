package institution

import (
	"context"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"nw-updater/decrypt"
)

type Auth struct {
	Username          string
	EncryptedPassword string `yaml:"encrypted_password"`
}

type AccountMapping struct {
	Name    string
	Mapping string
}

type Institution interface {
	GetBalances(context.Context, Auth, decrypt.Decryptor, []AccountMapping) (map[string]int64, error)
}

var institutions map[string]Institution = make(map[string]Institution)

func registerInstitution(name string, institution Institution) {
	institutions[name] = institution
}

func MustGet(name string) Institution {
	institution, ok := institutions[name]
	if !ok {
		panic("Invalid institution")
	}
	return institution
}

func newContext(ctx context.Context, urlPrefix string) (context.Context, context.CancelFunc) {
	// get the list of the targets
	infos, err := chromedp.Targets(ctx)
	if err != nil {
		log.Fatal(err)
	}
	i := slices.IndexFunc(infos, func(info *target.Info) bool {
		return strings.HasPrefix(info.URL, urlPrefix)
	})
	if i != -1 {
		ctx, _ = chromedp.NewContext(ctx, chromedp.WithTargetID(infos[i].TargetID))
	}
	// no targets or none match urlPrefix.
	ctx, _ = chromedp.NewContext(ctx)
	return context.WithTimeout(ctx, 1*time.Minute)
}

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
			log.Println(err)
			continue
		}
		trimmedName := strings.TrimSpace(name)
		mappingIndex := slices.IndexFunc(mapping, func(mapping AccountMapping) bool {
			return mapping.Name == trimmedName
		})
		if mappingIndex != -1 {
			balanceNum, err := parseCents(balance)
			if err != nil {
				log.Println(err)
				continue
			}
			balances[mapping[mappingIndex].Mapping] = balanceNum
		}
	}
	return balances, nil
}

var centsPattern = regexp.MustCompile(`\D`)

func parseCents(str string) (int64, error) {
	numsOnly := centsPattern.ReplaceAllString(str, "")
	return strconv.ParseInt(numsOnly, 10, 64)
}
