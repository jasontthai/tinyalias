package utils

import (
	"errors"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jasontthai/tinyalias/modules/auth"
)

const (
	BaseURL    = "baseUrl"
	ApiBaseURL = "APIUrl"

	DefaultLimit  = 20
	DefaultOffset = 0
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

func GetLimitAndOffsetQueries(c *gin.Context) (limit, offset uint64, err error) {
	offsetStr := c.Query("offset")
	if offsetStr != "" {
		offset, err = strconv.ParseUint(offsetStr, 10, 32)
		if err != nil {
			return 0, 0, err
		}
	} else {
		offset = DefaultOffset
	}

	limitStr := c.Query("limit")
	if limitStr != "" {
		limit, err = strconv.ParseUint(limitStr, 10, 32)
		if err != nil {
			return 0, 0, err
		}
	} else {
		limit = DefaultLimit
	}
	if limit > 100 {
		return 0, 0, errors.New("limit must be <= 100")
	}
	return limit, offset, err
}

func DataTableGetStartAndLengthQueries(c *gin.Context) (limit, offset uint64, err error) {
	startStr := c.PostForm("start")
	if startStr != "" {
		offset, err = strconv.ParseUint(startStr, 10, 32)
		if err != nil {
			return 0, 0, err
		}
	} else {
		offset = DefaultOffset
	}
	lengthStr := c.PostForm("length")
	if lengthStr != "" {
		limit, err = strconv.ParseUint(lengthStr, 10, 32)
		if err != nil {
			return 0, 0, err
		}
	} else {
		limit = DefaultLimit
	}
	if limit > 100 {
		return 0, 0, errors.New("limit must be <= 100")
	}
	return
}
