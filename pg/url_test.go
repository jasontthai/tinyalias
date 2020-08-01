package pg

import (
	"testing"

	"github.com/jasontthai/tinyalias/models"
	"github.com/jasontthai/tinyalias/test"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T) *sqlx.DB {
	db, err := sqlx.Open("postgres", test.GetTestPgURL())
	assert.Nil(t, err)
	return db
}

func TestURL(t *testing.T) {
	db := setup(t)

	slug := models.GenerateSlug(6)
	url := &models.URL{
		Url:  "https://example.com",
		Slug: slug,
	}

	// Test CreateURL
	err := CreateURL(db, url)
	assert.Nil(t, err)

	// Test GetURL
	returnedUrl, err := GetURL(db, slug)
	assert.Nil(t, err)
	assert.Equal(t, url.Url, returnedUrl.Url)

	// Test GetURLs
	returnedUrls, err := GetURLs(db, map[string]interface{}{
		"slug": slug,
	})
	assert.Nil(t, err)
	assert.Equal(t, returnedUrls[0].Url, url.Url)

	// Test UpdateURL
	url.Status = "inactive"
	err = UpdateURL(db, url)
	assert.Nil(t, err)
}
