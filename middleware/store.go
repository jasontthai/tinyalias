package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
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

func SessionStore(authKey, encryptKey string) gin.HandlerFunc {
	sessionStore := sessions.NewCookieStore(
		[]byte(authKey), []byte(encryptKey),
	)
	return func(c *gin.Context) {
		c.Set("SessionStore", sessionStore)
		c.Next()
	}
}

func GetSessionStore(c *gin.Context) *sessions.CookieStore {
	return c.Value("SessionStore").(*sessions.CookieStore)
}
