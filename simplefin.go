package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"nw-updater/crypto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SimpleFin interacts with the SimpleFin API to retrieve accounts and balances.
// It stores the SimpleFin access url in an encrypted file in the token directory.
type SimpleFin struct {
	Passphrase string `yaml:"passphrase"`
	TokenDir   string `yaml:"token_dir"`
}

// SFAccountSet is the JSON response from the SimpleFin accounts endpoint
type SFAccountSet struct {
	Accounts []SFAccountJson `json:"accounts"`
}

// SFAccountJson is the JSON representation of an account in SimpleFin
type SFAccountJson struct {
	Name             string `json:"name"`
	Balance          string `json:"balance"`
	AvailableBalance string `json:"available-balance"`
	BalanceDate      int64  `json:"balance-date"`
	Id               string `json:"id"`
}

// SFAccount is a SimpleFin account with idiomatic go types
type SFAccount struct {
	Name             string
	Balance          int64
	AvailableBalance int64
	BalanceDate      time.Time
	Id               string
}

const AccessUrlFilename = "access_url.txt"
const AccountsEndpoint = "accounts"

// GetBalances takes a map of SimpleFin account ids to Actual Budget account ids
// and returns a map of Actual Budget account ids to SimpleFin accounts with balances and balance dates.
func (sf SimpleFin) GetBalances(mappings map[string]string) (map[string]SFAccount, error) {
	accounts, err := sf.GetAllAccounts()
	if err != nil {
		return nil, err
	}
	balances := make(map[string]SFAccount)
	for _, account := range accounts {
		if mapping, ok := mappings[account.Name]; ok {
			balances[mapping] = account
		}
	}
	return balances, nil
}

// GetAllAccounts returns all SimpleFin accounts and their balances.
func (sf SimpleFin) GetAllAccounts() ([]SFAccount, error) {
	accessUrl, err := sf.GetAccessUrl()
	if err != nil {
		return nil, err
	}
	accountsUrl, err := url.JoinPath(accessUrl, AccountsEndpoint)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(accountsUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	accountSet := new(SFAccountSet)
	reader := resp.Body.(io.Reader)
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(accountSet)
	if err != nil {
		return nil, err
	}
	accounts := make([]SFAccount, len(accountSet.Accounts))
	for i, account := range accountSet.Accounts {
		balance, err := parseCurrency(account.Balance)
		if err != nil {
			return nil, err
		}
		availableBalance, err := parseCurrency(account.AvailableBalance)
		if err != nil {
			return nil, err
		}
		balanceDate := time.Unix(account.BalanceDate, 0)
		accounts[i] = SFAccount{
			Name:             account.Name,
			Balance:          balance,
			AvailableBalance: availableBalance,
			BalanceDate:      balanceDate,
			Id:               account.Id,
		}
	}
	return accounts, nil
}

// Authenticate takes a SimpleFin token, uses it to get the access url, and stores it in the token directory.
func (sf SimpleFin) Authenticate(token string) error {
	claimUrl, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return err
	}
	resp, err := http.Post(string(claimUrl), "", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	accessUrl, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return sf.storeAccessUrl(accessUrl, sf.Passphrase)
}

// storeAccessUrl encrypts and stores the access url in the token directory.
func (sf SimpleFin) storeAccessUrl(url []byte, passphrase string) error {
	encrypted, err := crypto.EncryptAES256GCM(url, passphrase)
	if err != nil {
		return err
	}
	encodedLen := base64.StdEncoding.EncodedLen(len(encrypted))
	encoded := make([]byte, encodedLen)
	base64.StdEncoding.Encode(encoded, encrypted)
	return os.WriteFile(filepath.Join(sf.TokenDir, AccessUrlFilename), encoded, 0600)
}

// IsAuthenticated returns true if the SimpleFin access url file exists in the token directory.
func (sf SimpleFin) IsAuthenticated() bool {
	_, err := os.Stat(filepath.Join(sf.TokenDir, AccessUrlFilename))
	return err == nil
}

// GetAccessUrl returns the SimpleFin access url from the encrypted file.
func (sf SimpleFin) GetAccessUrl() (string, error) {
	encoded, err := os.ReadFile(filepath.Join(sf.TokenDir, AccessUrlFilename))
	if err != nil {
		return "", err
	}
	decodedLen := base64.StdEncoding.DecodedLen(len(encoded))
	encrypted := make([]byte, decodedLen)
	_, err = base64.StdEncoding.Decode(encrypted, encoded)
	if err != nil {
		return "", err
	}
	return crypto.DecryptAES256GCM(encrypted, sf.Passphrase)
}

// parseCurrency removes the decimal point from a string representing a currency amount
// and converts it to an int64 representing the amount in cents.
func parseCurrency(s string) (int64, error) {
	noDots := strings.ReplaceAll(s, ".", "")
	return strconv.ParseInt(noDots, 10, 64)
}
