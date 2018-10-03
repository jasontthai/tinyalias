package main

import (
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	_ "github.com/lib/pq"
	"github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/_integrations/nrgin/v1"
	log "github.com/sirupsen/logrus"
	"github.com/ulule/limiter"
	mgin "github.com/ulule/limiter/drivers/middleware/gin"
	"github.com/ulule/limiter/drivers/store/memory"
	"github.com/zirius/url-shortener/middleware"
	"github.com/zirius/url-shortener/modules/queue"
	"github.com/zirius/url-shortener/modules/url"
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

	config := newrelic.NewConfig(os.Getenv("APP_NAME"), os.Getenv("NEW_RELIC_LICENSE_KEY"))
	app, err := newrelic.NewApplication(config)
	if err != nil {
		log.Fatal("error initializing new relic")
	}

	// Que-Go
	pgxpool, qc, err := queue.Setup(database)
	if err != nil {
		log.Fatal("error initializing que-go")
	}
	defer pgxpool.Close()

	// Rate Limiter
	rate := limiter.Rate{
		Period: time.Second,
		Limit: func() int64 {
			rate, err := strconv.Atoi(os.Getenv("RATE_LIMIT"))
			if err != nil {
				return 100
			}
			return int64(rate)
		}(),
	}
	store := memory.NewStore()

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	//router.Static("/static", "static")
	router.Use(middleware.Database(database))
	router.Use(middleware.GeoIP())
	router.Use(middleware.Que(pgxpool, qc))
	router.Use(nrgin.Middleware(app))
	router.Use(mgin.NewMiddleware(limiter.New(store, rate)))
	router.ForwardedByClientIP = true

	router.GET("", url.GetHomePage)
	router.POST("", url.CreateURL)
	router.GET("/:slug", url.Get)

	router.Run(":" + port)
}
