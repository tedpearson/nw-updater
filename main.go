/*
nw-updater updates account balances in [ynab], getting current account balances from various institutions.
Current account balances are retrieved using Chrome DevTools Protocol via [chromedp].
YNAB is updated using the YNAB API via [ynab.go].

Usage:

nw-updater [flags]

The flags are:

	--config config_file

		The config file to parse accounts and authentication information from. Defaults to config.yaml

	--passphrase-file file

		The file containing the passphrase to use to decrypt passwords in the config file.

	--headless

		If specified, Chrome instance will be created without a window.

	--websocket url

		Optional websocket url to an existing Chrome DevTools instance.

[ynab]: https://www.ynab.com/
[chromedp]: https://github.com/chromedp/chromedp
[ynab.go]: https://github.com/brunomvsouza/ynab.go
*/
package main

import (
	"context"
	"flag"
	"log"
	"maps"
	"os"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"

	cu "github.com/Davincible/chromedp-undetected"

	"nw-updater/decrypt"
	"nw-updater/institution"
)

// Config contains the fields read from the config.yaml file.
type Config struct {
	InstitutionConfig []InstitutionConfig `yaml:"institutions"`
	YnabConfig        YnabConfig          `yaml:"ynab"`
}

// InstitutionConfig contains the configs for an account at an institution along with the mapping to a YNAB account.
type InstitutionConfig struct {
	Name            string                       // The name of the institution, for finding the correct instance to get balances
	Auth            institution.Auth             // The credentials to log in to the institution
	AccountMappings []institution.AccountMapping `yaml:"accounts"` // The mapping from name in the institution to name in YNAB
}

func main() {
	configFlag := flag.String("config", "config.yaml", "Config file")
	passphraseFileFlag := flag.String("passphrase-file", ".passphrase",
		"File containing passphrase to decrypt passwords in config file")
	// See https://github.com/Davincible/chromedp-undetected, only works headless in Linux.
	headlessFlag := flag.Bool("headless", false, "Runs chrome in headless mode (Linux only currently)")
	websocketFlag := flag.String("websocket", "",
		"Use existing chrome instance via websocket url (launch chrome with --remote-debugging-port=9222)")
	flag.Parse()
	// read config
	file, err := os.ReadFile(*configFlag)
	if err != nil {
		log.Panic(err)
	}
	var config Config
	err = yaml.Unmarshal(file, &config)
	file = nil
	if err != nil {
		log.Panic(err)
	}

	ctx, cancel := GetContext(*headlessFlag, *websocketFlag)
	defer cancel()
	decryptor := decrypt.NewDecryptor(*passphraseFileFlag)
	balances := GetAllBalances(ctx, config.InstitutionConfig, decryptor)
	log.Printf("%v\n", balances)
	err = YnabUpdateBalances(balances, config.YnabConfig)
	if err != nil {
		log.Printf("Error updating ynab balances: %v\n", err)
	}
}

// GetAllBalances gets the balances for each InstitutionConfig from the corresponding [institution.Institution]
// and returns all balances in a map where keys are the YNAB account name and values are in cents.
func GetAllBalances(ctx context.Context, config []InstitutionConfig, decryptor decrypt.Decryptor) map[string]int64 {
	balances := make(map[string]int64)
	for _, ic := range config {
		bs, err := institution.MustGet(ic.Name).GetBalances(ctx, ic.Auth, decryptor, ic.AccountMappings)
		if err != nil {
			log.Printf("Failed to get balances from %s: %v", ic.Name, err)
		}
		maps.Copy(balances, bs)
	}
	return balances
}

// GetContext creates a new context for [chromedp]. If websocket is given, it creates a context connected to an
// existing Chrome instance. Otherwise it creates a context that starts a new Chrome instance, and if headless
// is true, will be run without a window (headless only works on linux currently per https://github.com/Davincible/chromedp-undetected).
func GetContext(headless bool, websocket string) (context.Context, context.CancelFunc) {
	if len(websocket) > 0 {
		allocatorContext, cancel1 := chromedp.NewRemoteAllocator(context.Background(), websocket)
		ctx, cancel2 := chromedp.NewContext(allocatorContext)
		return ctx, func() {
			cancel2()
			cancel1()
		}
	}
	cuConfig := cu.NewConfig()
	if headless {
		cuConfig.Headless = true
	}
	ctx, cancel, err := cu.New(cuConfig)
	if err != nil {
		log.Panic(err)
	}
	return ctx, func() {
		cancel()
	}
}
