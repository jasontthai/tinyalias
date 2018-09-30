package models

import (
	"time"

	"github.com/guregu/null"
)

type URLStat struct {
	Slug       string      `json:"slug" db:"slug"`
	Country    string      `json:"country" db:"country"`
	State      string      `json:"state" db:"state"`
	Counter    int         `json:"counter" db:"counter"`
	Properties PropertyMap `json:"properties" db:"properties"`
	Created    time.Time   `json:"created" db:"created"`
	Updated    null.Time   `json:"updated" db:"updated"`
}
