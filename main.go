package main

import (
	"database/sql"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/pg"
)

const (
	base = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://localhost:12345/postgres?sslmode=disable"
	}

	baseUrl := os.Getenv("BASE_URL")
	if baseUrl == "" {
		baseUrl = "localhost:5000/"
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	//router.Static("/static", "static")

	db, err := sqlx.Open("postgres", database)
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	router.GET("", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main.tmpl.html", gin.H{})
	})

	router.POST("", func(c *gin.Context) {

		url := c.PostForm("URL")
		var shortened string
		var error = "Oops. Something went wrong. Please try again."
		if url != "" {
			log.WithFields(log.Fields{
				"url": url,
			}).Info("Got URL")

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
			if urlObj == nil {
				// New URL
				urlObj = &models.URL{
					Url:     url,
					Slug:    generateSlug(),
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
		c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
			"url": shortened,
		})
	})

	router.GET("/:slug", func(c *gin.Context) {
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
	})

	router.Run(":" + port)
}

func generateSlug() string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	var slug string
	for i := 0; i < 6; i++ {
		idx := r.Intn(len(base))
		slug = slug + string(base[idx])
	}
	return slug
}
