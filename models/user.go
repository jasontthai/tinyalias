package models

import (
	"time"

	"github.com/guregu/null"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	Username   string      `json:"username" db:"username"`
	Role       string      `json:"role" db:"role"`
	Password   string      `json:"-" db:"password"`
	Status     string      `json:"status" db:"status"`
	Properties PropertyMap `json:"properties" db:"properties"`
	Created    time.Time   `json:"created" db:"created"`
	Updated    null.Time   `json:"updated" db:"updated"`
}
