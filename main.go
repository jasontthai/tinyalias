package main

import (
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	_ "github.com/lib/pq"
	"github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/_integrations/nrgin/v1"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/middleware"
	"github.com/zirius/url-shortener/modules"
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
		log.Fatal("$DATABASE_URL must be set")
	}

	config := newrelic.NewConfig("tinyalias", os.Getenv("NEW_RELIC_LICENSE_KEY"))
	app, err := newrelic.NewApplication(config)
	if err != nil {
		log.Fatal("error initializing new relic")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	//router.Static("/static", "static")
	router.Use(middleware.Database(database))
	router.Use(nrgin.Middleware(app))

	router.GET("", modules.GetHomePage)
	router.POST("", modules.CreateURL)
	router.GET("/:slug", modules.GetURL)

	router.Run(":" + port)
}
