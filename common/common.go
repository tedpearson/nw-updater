package common

import "time"

type AccountBalance struct {
	Balance     int64
	BalanceDate time.Time
	Id          string
	Name        string
}
