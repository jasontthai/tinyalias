package main

import (
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jasontthai/tinyalias/middleware"
	"github.com/jasontthai/tinyalias/modules/auth"
	"github.com/jasontthai/tinyalias/modules/queue"
	"github.com/jasontthai/tinyalias/modules/url"
	_ "github.com/lib/pq"
	"github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/_integrations/nrgin/v1"
	log "github.com/sirupsen/logrus"
	"github.com/ulule/limiter"
	mgin "github.com/ulule/limiter/drivers/middleware/gin"
	"github.com/ulule/limiter/drivers/store/memory"
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

	sessionAuthKey := os.Getenv("SESSION_AUTHENTICATION_KEY")
	if sessionAuthKey == "" {
		log.Fatal("$SESSION_AUTHENTICATION_KEY must be set")
	}

	// 8, 16, or 32 byte string
	sessionEncryptKey := os.Getenv("SESSION_ENCRYPTION_KEY")
	if sessionEncryptKey == "" {
		log.Fatal("$SESSION_ENCRYPTION_KEY must be set")
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
	router.Use(middleware.Database(database))
	router.Use(middleware.Que(pgxpool, qc))
	router.Use(mgin.NewMiddleware(limiter.New(store, rate)))
	router.Use(middleware.SessionStore(sessionAuthKey, sessionEncryptKey))
	router.ForwardedByClientIP = true
	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Length", "Content-Type"},
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowCredentials: true,
		MaxAge:           10 * time.Minute,
	}))
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	if os.Getenv("NEW_RELIC_LICENSE_KEY") != "" {
		config := newrelic.NewConfig(os.Getenv("APP_NAME"), os.Getenv("NEW_RELIC_LICENSE_KEY"))
		app, err := newrelic.NewApplication(config)
		if err != nil {
			log.Fatal("error initializing new relic")
		}
		router.Use(nrgin.Middleware(app))
	}

	router.GET("", url.GetHomePage)
	router.GET("/:slug", url.Get)
	router.POST("/login", auth.Login)
	router.POST("/register", auth.Register)
	router.POST("/update-password", auth.UpdatePassword)
	router.POST("/del", url.HandleDeleteLinks)
	router.POST("/get", url.HandleGetLinks)
	router.POST("/signal", url.HandleCopySignal)

	router.Run(":" + port)
}
