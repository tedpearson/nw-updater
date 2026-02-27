package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"
)

// ActualBudget is used to interact with the Actual Budget Http api.
// It contains the configuration for connecting to the Actual HTTP API.
type ActualBudget struct {
	ApiKey string `yaml:"api_key"`
	ApiUrl string `yaml:"api_url"`
	SyncId string `yaml:"sync_id"`
}

// ABAccounts holds the JSON response from the Actual Budget accounts endpoint
type ABAccounts struct {
	Data []ABAccount `json:"data"`
}

// ABAccount is the JSON representation of an Actual Budget account
type ABAccount struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	OffBudget bool   `json:"offbudget"`
	Closed    bool   `json:"closed"`
}

// ABBalance holds the JSON response from the Actual Budget account balance endpoint
type ABBalance struct {
	Data int64 `json:"data"`
}

// ABTransactionRequest is the JSON request body for creating a transaction in Actual Budget
type ABTransactionRequest struct {
	LearnCategories bool          `json:"learnCategories"`
	RunTransfers    bool          `json:"runTransfers"`
	Transaction     ABTransaction `json:"transaction"`
}

// ABTransaction is the JSON representation of a transaction in Actual Budget
type ABTransaction struct {
	Account   string `json:"account"`
	Category  string `json:"category"`
	Amount    int64  `json:"amount"`
	PayeeName string `json:"payee_name"`
	Date      string `json:"date"`
	Cleared   bool   `json:"cleared"`
	Notes     string `json:"notes"`
}

// UpdateBalances takes a map of Actual account ids to SimpleFin account structs and creates
// adjustment transactions in those accounts to make the account balances match.
func (a ActualBudget) UpdateBalances(balances map[string]SFAccount) error {
	accounts, err := a.GetAccounts()
	if err != nil {
		return err
	}
	// update balances
	for _, account := range accounts {
		if balance, ok := balances[account.Id]; ok {
			err = a.updateBalance(account, balance)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// updateBalance updates the balance of an Actual account to match the provided SimpleFin account balance
// as of the balance date.
func (a ActualBudget) updateBalance(account ABAccount, balance SFAccount) error {
	balanceUrl, err := url.JoinPath(a.ApiUrl, "budgets", a.SyncId, "accounts", account.Id, "balance")
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", balanceUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", a.ApiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	balanceJson := new(ABBalance)
	reader := resp.Body.(io.Reader)
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(balanceJson)
	if err != nil {
		return err
	}
	newBalance := balance.Balance
	if balanceJson.Data == newBalance {
		fmt.Printf("Account balance has not changed for '%s': ($%.2f)\n", account.Name, float64(newBalance)/100.0)
		return nil
	}
	// balances differ
	payee := "Market Changes"
	memo := "Entered automatically by nw-updater"
	difference := newBalance - balanceJson.Data
	transactionDate := balance.BalanceDate.Format(time.DateOnly)

	transaction := ABTransactionRequest{
		LearnCategories: false,
		RunTransfers:    false,
		Transaction: ABTransaction{
			Account:   account.Id,
			Amount:    difference,
			PayeeName: payee,
			Date:      transactionDate,
			Cleared:   true,
			Notes:     memo,
		},
	}
	buffer := new(bytes.Buffer)
	err = json.NewEncoder(buffer).Encode(transaction)
	if err != nil {
		return err
	}
	transactionUrl, err := url.JoinPath(a.ApiUrl, "budgets", a.SyncId, "accounts", account.Id, "transactions")
	if err != nil {
		return err
	}

	req, err = http.NewRequest("POST", transactionUrl, buffer)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", a.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	sign := "+"
	if difference < 0 {
		sign = "-"
	}
	fmt.Printf("Updated '%s' to $%.2f as of %s ($%s%.2f)\n",
		account.Name, float64(newBalance)/100.0, transactionDate, sign, math.Abs(float64(difference))/100.0)
	return nil
}

// GetAccounts returns a list of all accounts in the budget.
func (a ActualBudget) GetAccounts() ([]ABAccount, error) {
	// get accounts
	accountsUrl, err := url.JoinPath(a.ApiUrl, "budgets", a.SyncId, "accounts")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", accountsUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", a.ApiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	accounts := new(ABAccounts)
	reader := resp.Body.(io.Reader)
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(accounts)
	return accounts.Data, err
}
