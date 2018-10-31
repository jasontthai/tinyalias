package utils

import (
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zirius/tinyalias/middleware"
	"github.com/zirius/tinyalias/modules/auth"
)

const (
	base = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
)

func GenerateSlug(size int) string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	var slug string
	for i := 0; i < size; i++ {
		idx := r.Intn(len(base))
		slug = slug + string(base[idx])
	}
	return slug
}

func HandleHtmlResponse(c *gin.Context, statusCode int, template string, h gin.H) {
	sessionStore := middleware.GetSessionStore(c)

	session, err := sessionStore.Get(c.Request, auth.SessionName)
	if err != nil {
		c.Error(err)
	}

	username, found := session.Values["username"]
	if found {
		h["user"] = username
	}
	c.HTML(statusCode, template, h)
}
