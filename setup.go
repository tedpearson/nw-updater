package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/yarlson/tap"
	"gopkg.in/yaml.v3"
)

const SFMappingPattern = "ACT-[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}"

// Setup configures the mapping between SimpleFin accounts and Actual Budget accounts, and writes
// that mapping to a file as well as to stdout.
func Setup(sf SimpleFin, a ActualBudget, config Config, configFile string) error {
	if !sf.IsAuthenticated() {
		return fmt.Errorf("error getting access url, try `nw-updater simplefin-auth`")
	}
	sfAccounts, err := sf.GetAllAccounts()
	if err != nil {
		return err
	}
	// build items for select
	sfNames := make([]string, len(sfAccounts))
	for i, sfAccount := range sfAccounts {
		sfNames[i] = fmt.Sprintf("%s: $%.2f as of %s", sfAccount.Name, float64(sfAccount.Balance)/100.0, sfAccount.BalanceDate.Format(time.RFC850))
	}
	mappings := config.AccountMappings
	sfSelected := make([]int, 0, len(mappings))
	for i, sfAccount := range sfAccounts {
		if _, ok := mappings[sfAccount.Id]; ok {
			sfSelected = append(sfSelected, i)
		}
	}
	sfAccountIndexes := MultiSelect("Select accounts to sync", sfNames, sfSelected)
	// get names of accounts in actual budget
	abAccounts, err := a.GetAccounts()
	if err != nil {
		return err
	}
	slices.SortFunc(abAccounts, func(a1, a2 ABAccount) int {
		return strings.Compare(a1.Name, a2.Name)
	})
	filteredAccounts := make([]ABAccount, 0)
	for _, account := range abAccounts {
		if account.OffBudget && !account.Closed {
			filteredAccounts = append(filteredAccounts, account)
		}
	}
	abNames := make([]string, 0, len(abAccounts))
	for _, account := range filteredAccounts {
		abNames = append(abNames, account.Name)
	}
	updatedMappings := make(map[string]string)
	r := regexp.MustCompile(SFMappingPattern)
	// keep mappings that don't match the SimpleFin pattern
	for key, value := range mappings {
		if !r.MatchString(key) {
			updatedMappings[key] = value
		}
	}
	for i, sfAccountIndex := range sfAccountIndexes {
		var selected *int
		if mapping, ok := mappings[sfAccounts[sfAccountIndex].Id]; ok {
			selected = new(slices.IndexFunc(filteredAccounts, func(abAccount ABAccount) bool {
				return abAccount.Name == mapping
			}))
			if *selected == -1 {
				selected = nil
			}
		}
		message := fmt.Sprintf("[%d/%d] Select account to sync '%s' to", i, len(sfAccountIndexes), sfNames[sfAccountIndex])
		abAccountIndex := SingleSelect(message, abNames, selected)
		updatedMappings[sfAccounts[sfAccountIndex].Id] = filteredAccounts[abAccountIndex].Name
	}
	f, err := os.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("error opening config file: %w", err)
	}
	defer f.Close()
	config.AccountMappings = updatedMappings
	err = yaml.NewEncoder(f).Encode(config)
	if err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}
	return nil
}

// MultiSelect displays a multi-select prompt with a given message,
// options, and preselected indices, and returns selected indices.
func MultiSelect(message string, options []string, selected []int) []int {
	selectOptions := make([]tap.SelectOption[int], len(options))
	for i, option := range options {
		selectOptions[i] = tap.SelectOption[int]{Value: i, Label: option}
	}
	return tap.MultiSelect[int](context.Background(), tap.MultiSelectOptions[int]{
		Message:       message,
		Options:       selectOptions,
		InitialValues: selected,
	})
}

// SingleSelect displays a single-select prompt with a given message,
// options, and preselected index, and returns the selected index.
func SingleSelect(message string, options []string, selected *int) int {
	selectOptions := make([]tap.SelectOption[int], len(options))
	for i, option := range options {
		selectOptions[i] = tap.SelectOption[int]{Value: i, Label: option}
	}
	return tap.Select[int](context.Background(), tap.SelectOptions[int]{
		Message:      message,
		Options:      selectOptions,
		InitialValue: selected,
	})
}
