package main

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/brunomvsouza/ynab.go"
	"github.com/brunomvsouza/ynab.go/api"
	"github.com/brunomvsouza/ynab.go/api/account"
	"github.com/brunomvsouza/ynab.go/api/budget"
	"github.com/brunomvsouza/ynab.go/api/transaction"
)

// YnabConfig contains the information needed to update YNAB accounts.
type YnabConfig struct {
	AccessToken string `yaml:"access_token"`
	BudgetName  string `yaml:"budget_name"`
}

// YnabUpdateBalances takes a map of YNAB account names to balances in cents as well as a config, and creates
// adjustment transactions in those accounts to make the account balances match.
func YnabUpdateBalances(balances map[string]int64, config YnabConfig) error {
	c := ynab.NewClient(config.AccessToken)
	budgets, err := c.Budget().GetBudgets()
	if err != nil {
		return err
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
		return err
	}
	for accountName, balance := range balances {
		err := updateBalance(c, bId, accountName, balance, results.Accounts)
		if err != nil {
			log.Println(err)
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
	// todo: run checks here!
	err := validateAccount(acct, newBalance)
	if err != nil {
		return err
	}
	payee := "Market Changes"
	memo := "Entered automatically by nw-updater"
	_, err = c.Transaction().CreateTransaction(budgetId, transaction.PayloadTransaction{
		AccountID: acct.ID,
		Date:      api.Date{Time: time.Now()},
		Amount:    (newBalance * 10) - acct.Balance,
		Cleared:   transaction.ClearingStatusReconciled,
		Approved:  true,
		PayeeName: &payee,
		Memo:      &memo,
	})
	return err
}

// validateAccount checks that an account: is on budget, not deleted or closed,
// is an "other asset" account, is up-to-date with reconciliation, and has a balance that has changed.
func validateAccount(acct *account.Account, newBalance int64) error {
	if acct.OnBudget || acct.Deleted || acct.Closed || acct.Type != account.TypeOtherAsset {
		return fmt.Errorf("account does not pass checks: %+v", acct)
	}
	if acct.ClearedBalance != acct.Balance || acct.UnclearedBalance > 0 {
		return fmt.Errorf("account has an uncleared balance %+v", acct)
	}
	if acct.Balance == newBalance*10 {
		return fmt.Errorf("account balance has not changed %+v", acct)
	}
	return nil
}
