package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func Database(databaseURL string) gin.HandlerFunc {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		c.Set("DB", db)
		c.Next()
	}
}

func GetDB(c *gin.Context) *sqlx.DB {
	return c.Value("DB").(*sqlx.DB)
}
