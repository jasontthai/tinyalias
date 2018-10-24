package models

import (
	"fmt"
	"time"

	"github.com/guregu/null"
	"golang.org/x/crypto/bcrypt"
)

const (
	Active = "active"
	Pending = "pending"
	Expired = "expired"
)

type URL struct {
	Url      string    `json:"url" db:"url"`
	Slug     string    `json:"slug" db:"slug"`
	IP       string    `json:"ip" db:"ip"`
	Counter  int       `json:"counter" db:"counter"`
	Created  time.Time `json:"created" db:"created"`
	Updated  null.Time `json:"updated" db:"updated"`
	Status   string    `json:"status" db:"status"`
	Password string    `json:"password" db:"password"`
	Expired  null.Time `json:"expired" db:"expired"`
	Mindful  bool      `json:"mindful" db:"mindful"`
}

func TransformPassword(val string) (string, error) {
	pwbytes, err := bcrypt.GenerateFromPassword([]byte(val), bcrypt.DefaultCost)
	if err != nil {
		return val, fmt.Errorf("password encryption error: %s\n", err.Error())
	}
	return string(pwbytes), nil
}

func VerifyPassword(hashedPassword string, val string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(val))
}
