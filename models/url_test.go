package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPassword(t *testing.T) {
	hashPassword, err := TransformPassword("abcdef")
	assert.Nil(t, err)

	err = VerifyPassword(hashPassword, "abcdef")
	assert.Nil(t, err)
}
