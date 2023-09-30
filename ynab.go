package main

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/brunomvsouza/ynab.go"
	"github.com/brunomvsouza/ynab.go/api"
	"github.com/brunomvsouza/ynab.go/api/account"
	"github.com/brunomvsouza/ynab.go/api/budget"
	"github.com/brunomvsouza/ynab.go/api/transaction"

	"nw-updater/decrypt"
)

// YnabConfig contains the information needed to update YNAB accounts.
type YnabConfig struct {
	EncryptedAccessToken string `yaml:"encrypted_access_token"`
	BudgetName           string `yaml:"budget_name"`
}

// YnabUpdateBalances takes a map of YNAB account names to balances in cents as well as a config, and creates
// adjustment transactions in those accounts to make the account balances match.
func YnabUpdateBalances(balances map[string]int64, config YnabConfig, decryptor decrypt.Decryptor) error {
	c := ynab.NewClient(decryptor.Decrypt(config.EncryptedAccessToken))
	budgets, err := c.Budget().GetBudgets()
	if err != nil {
		return fmt.Errorf("unable to get budget: %w", err)
	}
	bIdx := slices.IndexFunc(budgets, func(summary *budget.Summary) bool {
		return summary.Name == config.BudgetName
	})
	if bIdx == -1 {
		return errors.New("unable to find budget")
	}
	bId := budgets[bIdx].ID
	results, err := c.Account().GetAccounts(bId, nil)
	if err != nil {
		return fmt.Errorf("unable to get accounts: %w", err)
	}
	for accountName, balance := range balances {
		err := updateBalance(c, bId, accountName, balance, results.Accounts)
		if err != nil {
			fmt.Printf("Unable to update balance for '%s': %v\n", accountName, err)
		}
	}
	return nil
}

// updateBalance Updates the balance in an individual YNAB account by creating an adjustment transaction
func updateBalance(c ynab.ClientServicer, budgetId, accountName string, newBalance int64, accounts []*account.Account) error {
	accountIdx := slices.IndexFunc(accounts, func(a *account.Account) bool {
		return a.Name == accountName
	})
	if accountIdx == -1 {
		return fmt.Errorf("unable to find account '%s'", accountName)
	}
	acct := accounts[accountIdx]
	err := validateAccount(acct, newBalance)
	if err != nil {
		return err
	}
	payee := "Market Changes"
	memo := "Entered automatically by nw-updater"
	difference := (newBalance * 10) - acct.Balance
	_, err = c.Transaction().CreateTransaction(budgetId, transaction.PayloadTransaction{
		AccountID: acct.ID,
		Date:      api.Date{Time: time.Now()},
		Amount:    difference,
		Cleared:   transaction.ClearingStatusReconciled,
		Approved:  true,
		PayeeName: &payee,
		Memo:      &memo,
	})
	if err != nil {
		return fmt.Errorf("unable to create adjustment transaction on account '%s': %w", accountName, err)
	}
	sign := "+"
	if difference < 0 {
		sign = "-"
	}
	fmt.Printf("Updated '%s' to $%.2f ($%s%.2f)\n",
		accountName, float64(newBalance)/100.0, sign, math.Abs(float64(difference))/1000.0)
	return nil
}

// validateAccount checks that an account: is on budget, not deleted or closed,
// is an "other asset" account, is up-to-date with reconciliation, and has a balance that has changed.
func validateAccount(acct *account.Account, newBalance int64) error {
	if acct.OnBudget || acct.Deleted || acct.Closed || acct.Type != account.TypeOtherAsset {
		return fmt.Errorf("account does not pass checks: %+v", acct)
	}
	if acct.ClearedBalance != acct.Balance || acct.UnclearedBalance > 0 {
		return fmt.Errorf("account has an uncleared balance: %+v", acct)
	}
	if acct.Balance == newBalance*10 {
		return errors.New("account balance has not changed")
	}
	return nil
}
