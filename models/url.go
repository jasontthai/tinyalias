package models

import (
	"time"

	"github.com/guregu/null"
)

type URL struct {
	Url       string    `json:"url" db:"url"`
	Slug      string    `json:"slug" db:"slug"`
	IP        string    `json:"ip" db:"ip"'`
	Counter   int       `json:"counter" db"counter"`
	Created   time.Time `json:"created" db:"created"`
	Updated   null.Time `json:"updated" db:"updated"`
	AccessIPs []string  `json:"access_ips" db:"access_ips"`
}
