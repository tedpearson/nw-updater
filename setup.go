package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/yarlson/tap"
	"gopkg.in/yaml.v3"
)

// Setup configures the mapping between SimpleFin accounts and Actual Budget accounts, and writes
// that mapping to a file as well as to stdout.
func Setup(sf SimpleFin, a ActualBudget, mappingFile string) error {
	if !sf.IsAuthenticated() {
		return fmt.Errorf("error getting access url, try `nw-updater simplefin-auth`")
	}
	mappings, err := ReadMappingFile(mappingFile)
	if err != nil {
		return err
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
	accountMapping := make(map[string]string)
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
		accountMapping[sfAccounts[sfAccountIndex].Id] = filteredAccounts[abAccountIndex].Name
	}
	f, err := os.Create(mappingFile)
	if err != nil {
		return fmt.Errorf("error creating mapping file: %w", err)
	}
	defer f.Close()
	err = yaml.NewEncoder(f).Encode(accountMapping)
	if err != nil {
		return fmt.Errorf("error writing mapping file: %w", err)
	}
	fmt.Printf("\n\n")
	err = yaml.NewEncoder(os.Stdout).Encode(accountMapping)
	if err != nil {
		return err
	}
	fmt.Printf("\nCopy the above config, if needed, for saving mapping.yaml in ansible, etc.\n")
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
