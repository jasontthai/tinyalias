package models

import (
	"time"

	"github.com/guregu/null"
)

type User struct {
	Username   string      `json:"username" db:"username"`
	Password   string      `json:"-" db:"password"`
	Status     string      `json:"status" db:"status"`
	Properties PropertyMap `json:"properties" db:"properties"`
	Created    time.Time   `json:"created" db:"created"`
	Updated    null.Time   `json:"updated" db:"updated"`
}
