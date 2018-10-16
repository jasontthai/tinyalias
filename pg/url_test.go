package pg

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/modules/utils"
	"github.com/zirius/url-shortener/test"
)

func setup(t *testing.T) *sqlx.DB {
	db, err := sqlx.Open("postgres", test.GetTestPgURL())
	assert.Nil(t, err)
	return db
}

func TestURL(t *testing.T) {
	db := setup(t)

	slug := utils.GenerateSlug(6)
	url := &models.URL{
		Url:  "https://example.com",
		Slug: slug,
	}

	// Test CreateURL
	err := CreateURL(db, url)
	assert.Nil(t, err)

	// Test GetURL
	returnedUrl, err := GetURL(db, url.Url, "")
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
