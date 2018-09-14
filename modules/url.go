package modules

import (
	"database/sql"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/middleware"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/pg"
)

var baseUrl string

func init() {
	baseUrl = os.Getenv("BASE_URL")
	if baseUrl == "" {
		baseUrl = "localhost:5000/"
	}
}

const (
	base = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
)

func GetHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"baseUrl": baseUrl,
	})
}

func CreateURL(c *gin.Context) {
	db := middleware.GetDB(c)

	url := c.PostForm("URL")
	slug := c.PostForm("SLUG")
	log.WithFields(log.Fields{
		"url":  url,
		"slug": slug,
	}).Info("Got Post Form")

	var shortened string
	var error = "Oops. Something went wrong. Please try again."
	if url != "" {
		// URL sanitization
		url = strings.TrimSpace(url)
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}

		urlObj, err := pg.GetURL(db, url, "")
		if err != nil && err != sql.ErrNoRows {
			c.Error(err)
			c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
				"error": error,
			})
			return
		}

		if slug != "" {
			urlObjBySlug, err := pg.GetURL(db, "", slug)
			if err != nil && err != sql.ErrNoRows {
				c.Error(err)
				c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
					"error": error,
				})
				return
			}
			if urlObjBySlug != nil {
				// slug already exists so we generate new slug
				slug = slug + "-" + generateSlug(2)
			}
		} else {
			slug = generateSlug(6)
		}

		if urlObj == nil {
			// New URL
			urlObj = &models.URL{
				Url:     url,
				Slug:    slug,
				Created: time.Now(),
				IP:      c.ClientIP(),
			}
			err = pg.CreateURL(db, urlObj)
			if err != nil {
				c.Error(err)
				c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
					"error": error,
				})
				return
			}
		}
		shortened = baseUrl + urlObj.Slug
	}

	log.Info("Shortened URL generated: %v", shortened)
	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"url":     shortened,
		"baseUrl": baseUrl,
	})
}

func GetURL(c *gin.Context) {
	db := middleware.GetDB(c)

	slug := c.Param("slug")
	log.WithFields(log.Fields{
		"slug": slug,
	}).Info("Got SLUG")

	urlObj, err := pg.GetURL(db, "", slug)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
	}

	if urlObj != nil {

		urlObj.Counter += 1
		err = pg.UpdateURL(db, urlObj)
		if err != nil {
			c.Error(err)
		}

		c.Redirect(http.StatusFound, urlObj.Url)
		return
	}
	c.Redirect(http.StatusFound, baseUrl)
	return
}

func generateSlug(size int) string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	var slug string
	for i := 0; i < size; i++ {
		idx := r.Intn(len(base))
		slug = slug + string(base[idx])
	}
	return slug
}
