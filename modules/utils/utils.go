package utils

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/zirius/tinyalias/modules/auth"
)

const (
	BaseURL    = "baseUrl"
	ApiBaseURL = "APIUrl"
)

var BaseUrl string
var ApiBaseUrl string

func init() {
	BaseUrl = os.Getenv("BASE_URL")
	ApiBaseUrl = os.Getenv("API_BASE_URL")
}

func HandleHtmlResponse(c *gin.Context, statusCode int, template string, h gin.H) {
	user := auth.GetAuthenticatedUser(c)
	if user != nil {
		h["user"] = user.Username
	}
	h[BaseURL] = BaseUrl
	h[ApiBaseURL] = ApiBaseUrl
	c.HTML(statusCode, template, h)
}
