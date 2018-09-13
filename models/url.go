package models

import (
	"time"

	"github.com/guregu/null"
)

type URL struct {
	Url     string    `json:"url" db:"url"`
	Slug    string    `json:"slug" db:"slug"`
	IP      string    `json:"ip" db:"ip"'`
	Created time.Time `json:"created" db:"created"`
	Updated null.Time `json:"updated" db:"updated"`
}
