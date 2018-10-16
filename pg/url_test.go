package pg

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/modules/utils"
)

func setup(t *testing.T) *sqlx.DB {
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://localhost:12345/postgres?sslmode=disable"
	}
	db, err := sqlx.Open("postgres", database)
	assert.Nil(t, err)
	return db
}

func TestCreateURL(t *testing.T) {
	db := setup(t)
	url := &models.URL{
		Url:  "https://example.com",
		Slug: utils.GenerateSlug(6),
	}
	err := CreateURL(db, url)
	assert.Nil(t, err)
}
