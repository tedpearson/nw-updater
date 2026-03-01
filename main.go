/*
nw-updater syncs investment account balances into [YNAB] or [Actual Budget].

Two sync paths are supported:

 1. Institution login sync: fetch balances from supported institutions using [chromedp], then update YNAB.
 2. SimpleFin sync: fetch balances from [SimpleFin], map them to Actual Budget accounts, then create adjustment
    transactions through the Actual HTTP API.

Usage:

	nw-updater [flags] [command]

Flags:

	--config config_file
		Config file containing institutions, mappings, and destination settings. Defaults to config.yaml.

	--passphrase-file file
		File containing passphrase used to decrypt encrypted values in the config file.

	--headless
		Run Chrome in headless mode (Linux support via chromedp-undetected).

	--websocket url
		Use an existing Chrome DevTools instance.

Commands:

	security-code
		Complete MFA code entry for institution logins that require it.
		Args:
			--institution name  Institution key from config (required)
			--username value    Username from config
							    (optional, needed if there are multiple accounts at this institution)

	simplefin-auth
		Authenticat with SimpleFin and save the generated access URL.
		Args:
			--token value  SimpleFin setup token (required)

	setup
		Interactively map SimpleFin accounts to Actual Budget accounts and write mapping.yaml.
		Args:
			none

[YNAB]: https://www.ynab.com/
[Actual Budget]: https://actualbudget.org/
[SimpleFin]: https://beta-bridge.simplefin.org/
[chromedp]: https://github.com/chromedp/chromedp
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"maps"
	. "nw-updater/common"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"

	cu "github.com/Davincible/chromedp-undetected"

	"nw-updater/crypto"
	"nw-updater/institution"
)

// Config contains the fields read from the config.yaml file.
type Config struct {
	AccountMappings   map[string]string   `yaml:"account_mappings"`
	InstitutionConfig []InstitutionConfig `yaml:"institutions"`
	SimpleFin         *SimpleFin          `yaml:"simplefin"`
	YnabConfig        *YnabConfig         `yaml:"ynab"`
	ActualConfig      *ActualBudgetConfig `yaml:"actual"`
	EmailConfig       EmailConfig         `yaml:"email"`
}

// InstitutionConfig contains the configs for an account at an institution along with the mapping to a YNAB account.
type InstitutionConfig struct {
	Name string           // The name of the institution, for finding the correct instance to get balances
	Auth institution.Auth // The credentials to log in to the institution
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
	decryptor := crypto.NewOpenSslDecryptor(*passphraseFileFlag)
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "security-code":
			err = SecurityCodeMain(args[1:], ctx, config.InstitutionConfig, decryptor)
		case "simplefin-auth":
			err = SimpleFinAuthMain(args[1:], *config.SimpleFin)
		case "setup":
			err = SimpleFinSetupMain(config, *configFlag, decryptor)
		default:
			panic("unsupported command: " + args[0])
		}
	} else {
		err = StandardMain(config, ctx, decryptor)
	}
	if err != nil {
		err = Email(config.EmailConfig, decryptor, err)
		if err != nil {
			fmt.Printf("Error sending email with error: %s", err)
		}
	}

}

// StandardMain is the main function responsible for updating balances, fetching from either YNAB or Actual Budget,
// and updating either YNAB or Actual Budget.
func StandardMain(config Config, ctx context.Context, decryptor crypto.OpenSslDecryptor) error {
	balances := make(map[string]AccountBalance)
	if len(config.InstitutionConfig) > 0 {
		var err error
		balances, err = GetAllBalances(ctx, config.InstitutionConfig, config.AccountMappings, decryptor)
		if err != nil {
			err = Email(config.EmailConfig, decryptor, err)
			if err != nil {
				fmt.Printf("Error sending email with errors: %s", err)
			}
		}
	}
	if config.SimpleFin != nil {
		simpleFin := *config.SimpleFin
		simpleFinBalances, err := simpleFin.GetBalances(config.AccountMappings)
		if err != nil {
			return err
		}
		maps.Copy(balances, simpleFinBalances)
	}
	if config.ActualConfig != nil {
		actualBudget := NewActualBudget(*config.ActualConfig, decryptor)
		return actualBudget.UpdateBalances(balances)
	}
	if config.YnabConfig != nil {
		err := YnabUpdateBalances(balances, *config.YnabConfig, decryptor)
		if err != nil {
			return fmt.Errorf("error updating ynab balances: %w", err)
		}
		return nil
	}
	return fmt.Errorf("error, invalid config file")
}

// SimpleFinAuthMain authenticates with SimpleFin and saves the access URL to a file.
func SimpleFinAuthMain(args []string, sf SimpleFin) error {
	fs := flag.NewFlagSet("nw-updater simplefin-auth", flag.ExitOnError)
	token := fs.String("token", "", "The token to authenticate with")
	_ = fs.Parse(args)
	return sf.Authenticate(*token)
}

// SimpleFinSetupMain runs the interactive setup process for SimpleFin.
func SimpleFinSetupMain(config Config, configFile string, decryptor crypto.OpenSslDecryptor) error {
	sf := *config.SimpleFin
	ab := NewActualBudget(*config.ActualConfig, decryptor)
	return Setup(sf, ab, config, configFile)
}

// SecurityCodeMain handles the security code entry flow for institutions that require it.
// It finds the correct institution and account based on the provided args,
// then prompts the user to enter the code after requesting it from the institution.
func SecurityCodeMain(args []string, ctx context.Context, configs []InstitutionConfig, decryptor crypto.OpenSslDecryptor) error {
	// Parse additonal args
	fs := flag.NewFlagSet("nw-updater security-code", flag.ExitOnError)
	instString := fs.String("institution", "", "The institution to authenticate with")
	username := fs.String("username", "", "The username to use in the config file. Optional if there is only one for this institution.")
	_ = fs.Parse(args)

	sc, ok := institution.MustGet(*instString).(institution.SecurityCode)
	if !ok {
		panic(fmt.Sprintf("%s does not implement SecurityCode", *instString))
	}
	var instConfig *InstitutionConfig
	for _, inst := range configs {
		if inst.Name == *instString && (*username == "" || inst.Auth.Username == *username) {
			instConfig = &inst
			break
		}
	}
	if instConfig == nil {
		panic("unable to find matching institution in config")
	}
	ctx, cancel, err := sc.RequestCode(ctx, instConfig.Auth, decryptor)
	if err != nil {
		return err
	}
	defer cancel()
	code := institution.UserInput("Enter code: ")
	err = sc.EnterCode(ctx, code)
	if err != nil {
		return err
	}
	fmt.Println("Success!")
	return nil
}

// GetAllBalances gets the balances for each InstitutionConfig from the corresponding [institution.Institution]
// and returns all balances in a map where keys are the YNAB account name and values are in cents.
func GetAllBalances(ctx context.Context, config []InstitutionConfig, mappings map[string]string,
	decryptor crypto.OpenSslDecryptor) (map[string]AccountBalance, error) {

	balances := make(map[string]AccountBalance)
	errs := &institution.MultiError{}
	for _, ic := range config {
		fmt.Printf("Getting balances at %s for %s\n", ic.Name, ic.Auth.Username)
		inst := institution.MustGet(ic.Name)
		bs, err := inst.GetBalances(ctx, ic.Auth, decryptor, FilterMappings(mappings, ic.Name, ic.Auth.Username))
		fmt.Printf("Found %d matching balances\n", len(bs))
		if err != nil {
			newErr := fmt.Errorf("failed to get balances from %s: %w", ic.Name, err)
			errs.AddError(newErr)
			fmt.Println(newErr)
		}
		maps.Copy(balances, bs)
		// note: there is some strange issue occurring where the next new browser tab context fails to open without a sleep here.
		time.Sleep(1 * time.Second)
	}
	if errs.IsEmpty() {
		return balances, nil
	}
	return balances, errs
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
		panic(err)
	}
	return ctx, cancel
}

func FilterMappings(mappings map[string]string, institution, username string) map[string]string {
	prefix := institution + ":" + username + ":"
	for k, v := range mappings {
		if strings.Index(k, prefix) == 0 {
			mappings[k[len(prefix):]] = v
		}
	}
	return mappings
}
