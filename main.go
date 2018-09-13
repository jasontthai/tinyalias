package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heroku/go-getting-started/models"
	"github.com/heroku/go-getting-started/pg"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	base = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
)

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://localhost:12345/services?sslmode=disable"
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
	fmt.Println(db)

	router.GET("", func(c *gin.Context) {
		c.HTML(http.StatusOK, "main.tmpl.html", gin.H{})
	})

	router.POST("", func(c *gin.Context) {

		url := c.PostForm("URL")
		var shortened string
		var error = "Oops. Something went wrong. Please try again."
		if url != "" {
			log.Printf("Got url: %v", url)
			url = strings.TrimSpace(url)

			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "https://" + url
			}

			urlObj, err := pg.GetURL(db, url, "")
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Error getting url: %q", err)
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
					log.Printf("Error creating shortened url: %q", err)
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
		log.Printf("Got slug: %v", slug)

		urlObj, err := pg.GetURL(db, "", slug)
		if err != nil && err != sql.ErrNoRows {
			log.Print("Error getting urls: ", err)
		}

		if urlObj != nil {
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
