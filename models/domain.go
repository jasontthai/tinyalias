package models

import (
	"time"

	"github.com/guregu/null"
)

type Domain struct {
	Host       string      `json:"host" db:"host"`
	Blacklist  bool        `json:"blacklist" db:"blacklist"`
	Properties PropertyMap `json:"properties" db:"properties"`
	Created    time.Time   `json:"created" db:"created"`
	Updated    null.Time   `json:"updated" db:"updated"`
}
