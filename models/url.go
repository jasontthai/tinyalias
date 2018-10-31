package models

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/guregu/null"
	"golang.org/x/crypto/bcrypt"
)

const (
	base    = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	Active  = "active"
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
	Password string    `json:"-" db:"password"`
	Expired  null.Time `json:"expired" db:"expired"`
	Mindful  bool      `json:"mindful" db:"mindful"`
	Username string    `json:"username" db:"username"'`
}

func TransformPassword(val string) (string, error) {
	pwbytes, err := bcrypt.GenerateFromPassword([]byte(val), bcrypt.DefaultCost)
	if err != nil {
		return val, fmt.Errorf("password encryption error: %s", err.Error())
	}
	return string(pwbytes), nil
}

func VerifyPassword(hashedPassword string, val string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(val))
}

func GenerateSlug(size int) string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	var slug string
	for i := 0; i < size; i++ {
		idx := r.Intn(len(base))
		slug = slug + string(base[idx])
	}
	return slug
}
