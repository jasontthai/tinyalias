package test

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/ulule/limiter"
	mgin "github.com/ulule/limiter/drivers/middleware/gin"
	"github.com/ulule/limiter/drivers/store/memory"
	"github.com/zirius/tinyalias/middleware"
	"github.com/zirius/tinyalias/modules/queue"
)

func GetTestPgURL() string {
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://localhost:12345/postgres?sslmode=disable"
	}
	return database
}

func GetTestSessionKeys() (authKey, encryptKey string) {
	authKey = os.Getenv("SESSION_AUTHENTICATION_KEY")
	if authKey == "" {
		authKey = "123"
	}
	encryptKey = os.Getenv("SESSION_ENCRYPTION_KEY")
	if encryptKey == "" {
		encryptKey = "WlFbVowe"
	}
}

func GetTestRouter() *gin.Engine {
	// Que-Go
	pgxpool, qc, err := queue.Setup(GetTestPgURL())
	if err != nil {
		log.Fatal("error initializing que-go")
	}

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
	router.LoadHTMLGlob("../../templates/*.tmpl.html")
	router.Use(middleware.Database(GetTestPgURL()))
	router.Use(middleware.Que(pgxpool, qc))
	router.Use(mgin.NewMiddleware(limiter.New(store, rate)))

	authKey, encryptKey := GetTestSessionKeys()
	router.Use(middleware.SessionStore(authKey, encryptKey))

	return router
}
