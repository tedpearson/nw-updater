package main

import (
	"context"
	"flag"
	"fmt"
	"maps"
	"os"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"

	cu "github.com/Davincible/chromedp-undetected"

	"nw-updater/decrypt"
	"nw-updater/institution"
)

type Config struct {
	InstitutionConfig []InstitutionConfig `yaml:"institutions"`
	YnabConfig        YnabConfig          `yaml:"ynab"`
	decryptor         decrypt.Decryptor
}

type InstitutionConfig struct {
	Name            string
	Auth            institution.Auth
	AccountMappings []institution.AccountMapping `yaml:"accounts"`
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
		panic(err)
	}
	var config Config
	err = yaml.Unmarshal(file, &config)
	file = nil
	if err != nil {
		panic(err)
	}

	ctx, cancel := GetContext(*headlessFlag, *websocketFlag)
	defer cancel()
	decryptor := decrypt.NewDecryptor(*passphraseFileFlag)
	balances := GetAllBalances(ctx, config.InstitutionConfig, decryptor)
	fmt.Printf("%v\n", balances)
	err = YnabUpdateBalances(balances, config.YnabConfig)
}

func GetAllBalances(ctx context.Context, config []InstitutionConfig, decryptor decrypt.Decryptor) map[string]int64 {
	balances := make(map[string]int64)
	for _, ic := range config {
		bs, err := GetBalances(ctx, ic, decryptor)
		if err != nil {
			fmt.Printf("Failed to get balances from %s", ic.Name)
		}
		maps.Copy(balances, bs)
	}
	return balances
}

func GetBalances(ctx context.Context, ic InstitutionConfig, decryptor decrypt.Decryptor) (map[string]int64, error) {
	return institution.MustGet(ic.Name).GetBalances(ctx, ic.Auth, decryptor, ic.AccountMappings)
}

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
		panic(err)
	}
	return ctx, func() {
		cancel()
	}
}
