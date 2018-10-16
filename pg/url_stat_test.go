package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/modules/utils"
)

func TestURLStat(t *testing.T) {
	db := setup(t)

	slug := utils.GenerateSlug(6)
	urlStat := &models.URLStat{
		Slug:    slug,
		Counter: 1,
		Country: "United States",
		State:   "California",
	}

	// Test UpsertURLStat
	err := UpsertURLStat(db, urlStat)
	assert.Nil(t, err)

	// Test GetURLStats
	returnedUrlStats, err := GetURLStats(db, map[string]interface{}{
		"slug": slug,
	})
	assert.Nil(t, err)
	assert.Equal(t, returnedUrlStats[0].Slug, slug)
}
