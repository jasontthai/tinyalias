package url

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guregu/null"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/tinyalias/middleware"
	"github.com/zirius/tinyalias/models"
	"github.com/zirius/tinyalias/modules/newsapi"
	"github.com/zirius/tinyalias/modules/queue"
	"github.com/zirius/tinyalias/modules/utils"
	"github.com/zirius/tinyalias/pg"
)

var baseUrl string
var secret string
var tinyUrlRegexp *regexp.Regexp

func init() {
	baseUrl = os.Getenv("BASE_URL")

	// secret in order to use API GET route
	secret = os.Getenv("SECRET")
	tinyUrlRegexp = regexp.MustCompile(os.Getenv("BASE_URL") + "(.+)")
}

const (
	NotFoundQuery = "not-found"
	ExpiredQuery  = "expired"
	ThreatQuery   = "threat"
	SlugQuery     = "slug"
	BaseURL       = "baseUrl"
)

type APIResponse struct {
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Short      string `json:"short"`
	Original   string `json:"original"`
	Password   string `json:"password,omitempty"`
	Expiration int64  `json:"expiration,omitempty"`
	Message    string `json:"message,omitempty"`
}

func GetHomePage(c *gin.Context) {
	if strings.Contains(c.Request.Host, "api") {
		APIGetURLs(c)
		return
	}

	notFoundQuery := c.Query(NotFoundQuery)
	var error string
	if notFoundQuery != "" {
		error = "The link you entered doesn't exist. Fancy creating one?"
	}

	threatQuery := c.Query(ThreatQuery)
	if threatQuery != "" {
		error = fmt.Sprintf("The link is detected as unsafe. Reason: %s", threatQuery)
	}

	expiredQuery := c.Query(ExpiredQuery)
	if expiredQuery != "" {
		error = "The link you entered has expired. Fancy creating one?"
	}

	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		BaseURL: baseUrl,
		"error": error,
	})
}

func CreateURL(c *gin.Context) {
	url := c.Query("url")
	slug := c.Query("alias")
	expiration := c.Query("expiration")
	password := c.PostForm("password")
	mindful := c.PostForm("mindful")

	var expirationTime time.Time
	var err error
	if expiration != "" {
		// 10/31/2018 1:57 PM
		expirationTime, err = time.Parse("01/02/2006 3:04 PM", expiration)
		if err != nil {
			c.Error(err)
			c.HTML(http.StatusInternalServerError, "main.tmpl.html", gin.H{
				"error": "Oops. Something went wrong. Please try again.",
				BaseURL: baseUrl,
			})
			return
		}
	}

	shortened, err := createURL(c, url, slug, password, expirationTime, mindful == "true")
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "main.tmpl.html", gin.H{
			"error":    "Oops. Something went wrong. Please try again.",
			"original": url,
			BaseURL:    baseUrl,
		})
		return
	}

	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"url":      shortened,
		"original": url,
		BaseURL:    baseUrl,
	})
}

func Get(c *gin.Context) {
	db := middleware.GetDB(c)
	_, qc := middleware.GetQue(c)

	if handled := handleSpecialRoutes(c); handled {
		return
	}

	slug := c.Param("slug")
	log.WithFields(log.Fields{
		"slug": slug,
	}).Debug("Got SLUG")

	urlObj, err := pg.GetURL(db, "", slug)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
	}

	if urlObj != nil {

		urlObj.Counter += 1
		err = pg.UpdateURL(db, urlObj)
		if err != nil {
			c.Error(err)
		}

		log.Debug("Dispatching job")
		// Dispatch ParseGeoRequestJob
		if err := queue.DispatchParseGeoRequestJob(qc, queue.ParseGeoRequest{
			Slug: slug,
			IP:   c.ClientIP(),
		}); err != nil {
			log.WithFields(log.Fields{
				"slug": slug,
				"ip":   c.ClientIP(),
			}).WithError(err).Error("error sending queue job")
		}

		if urlObj.Status == "expired" || (urlObj.Expired.Valid && urlObj.Expired.Time.Before(time.Now())) {
			c.Redirect(http.StatusFound, fmt.Sprintf("?%v=%v", ExpiredQuery, slug))
			return
		}

		// return spammed
		if urlObj.Status != "active" {
			c.Redirect(http.StatusFound, fmt.Sprintf("?%v=%v&%v=%v", ThreatQuery, urlObj.Status, SlugQuery, slug))
			return
		}

		if urlObj.Password != "" {
			if c.Query("password") != "" {
				err = models.VerifyPassword(urlObj.Password, c.Query("password"))
				if err != nil {
					c.HTML(http.StatusOK, "password.tmpl.html", gin.H{
						"baseUrl": baseUrl,
						"error":   "Wrong Password. Try Again.",
					})
					return
				}
			} else {
				c.HTML(http.StatusOK, "password.tmpl.html", gin.H{
					"baseUrl": baseUrl,
				})
				return
			}
		}

		if urlObj.Mindful {
			c.HTML(http.StatusOK, "mindful.tmpl.html", gin.H{
				"baseUrl": baseUrl,
				"url":     urlObj.Url,
			})
			return
		}

		c.Redirect(http.StatusFound, urlObj.Url)
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/?%v=%v", NotFoundQuery, slug))
	return
}

func APICreateURL(c *gin.Context) {
	url := c.Query("url")
	slug := c.Query("alias")
	password := c.Query("password")
	expired := c.Query("expiration")
	mindful := c.Query("mindful")

	var expiration time.Time
	if expired != "" {
		i, err := strconv.ParseInt(expired, 10, 64)
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Failed to parse expiration. Expiration must be unix timestamp",
			})
			return
		}
		expiration = time.Unix(i, 0)
	}

	shortened, err := createURL(c, url, slug, password, expiration, mindful == "true")
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	res := APIResponse{
		Success:  true,
		Password: password,
		Short:    shortened,
		Original: url,
	}
	if !expiration.Equal(time.Time{}) {
		res.Expiration = expiration.Unix()
	}

	c.JSON(http.StatusOK, res)
	return
}

func APIGetURL(c *gin.Context) {
	db := middleware.GetDB(c)
	fmt.Println(c.Query("url"))
	fmt.Println(tinyUrlRegexp.String())
	submatches := tinyUrlRegexp.FindStringSubmatch(c.Query("url"))
	if len(submatches) < 2 {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
		})
		return
	}
	slug := submatches[1]
	url, err := pg.GetURL(db, "", slug)
	if err != nil {
		if err != sql.ErrNoRows {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
		})
		return
	}
	if url.Status == "expired" || (url.Expired.Valid && url.Expired.Time.Before(time.Now())) {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Message: "link expired",
		})
		return
	}

	// return spammed
	if url.Status != "active" {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Message: url.Status,
		})
		return
	}

	if url.Password != "" {
		if c.Query("password") != "" {
			err = models.VerifyPassword(url.Password, c.Query("password"))
			if err != nil {
				c.JSON(http.StatusBadRequest, APIResponse{
					Success: false,
					Error:   "invalid password",
				})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "password is required",
			})
			return
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Short:    submatches[0],
		Original: url.Url,
	})
	return
}

func APIGetURLs(c *gin.Context) {
	secretQuery := c.Query("secret")
	if secret != secretQuery {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
		return
	}

	db := middleware.GetDB(c)

	clauses := make(map[string]interface{})
	urls, err := pg.GetURLs(db, clauses)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    urls,
	})
}

func createURL(c *gin.Context, url, slug, password string, expiration time.Time, mindful bool) (string, error) {
	db := middleware.GetDB(c)
	_, qc := middleware.GetQue(c)

	var shortened string
	if url == "" {
		return shortened, nil
	}

	// URL sanitization
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	urlObj, err := pg.GetURL(db, url, slug)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if urlObj != nil {
		return baseUrl + urlObj.Slug, nil
	}

	if slug == "" {
		slug = utils.GenerateSlug(6)
	}

	urlObj = &models.URL{
		Url:     url,
		Slug:    slug,
		Created: time.Now(),
		IP:      c.ClientIP(),
		Mindful: mindful,
	}

	if password != "" {
		urlObj.Password, err = models.TransformPassword(password)
		if err != nil {
			return "", err
		}
	}
	if !expiration.Equal(time.Time{}) {
		urlObj.Expired = null.TimeFrom(expiration)
	}

	// Run spam job on new link
	err = pg.CreateURL(db, urlObj)
	if err != nil {
		return "", err
	}
	shortened = baseUrl + urlObj.Slug

	// Dispatch ParseGeoRequestJob
	if err := queue.DispatchDetectSpamJob(qc, url); err != nil {
		log.WithFields(log.Fields{
			"url": url,
		}).WithError(err).Error("error sending spam detect job")
	}

	log.WithFields(log.Fields{
		"short":    shortened,
		"original": url,
	}).Info("Shortened URL generated")
	return shortened, nil
}

func handleSpecialRoutes(c *gin.Context) bool {
	slug := c.Param("slug")
	var handled bool

	if strings.Contains(c.Request.Host, "api") {
		if slug == "create" {
			APICreateURL(c)
		} else if slug == "get" {
			APIGetURL(c)
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
			})
		}
		return true
	}

	if slug == "create" {
		APICreateURL(c)
		handled = true
	}
	if slug == "shorten" {
		CreateURL(c)
		handled = true
	}
	if slug == "favicon.ico" {
		c.File("./static/favicon.ico")
		handled = true
	}
	if slug == "robots.txt" {
		c.File("./static/robots.txt")
		handled = true
	}
	if slug == "analytics" {
		GetAnalytics(c)
		handled = true
	}
	if slug == "privacy-policy" {
		c.HTML(http.StatusOK, "privacypolicy.tmpl.html", gin.H{})
		handled = true
	}

	if slug == "api" {
		c.HTML(http.StatusOK, "api.tmpl.html", gin.H{
			BaseURL: baseUrl,
		})
		handled = true
	}

	if slug == "news" {
		GetNews(c)
		handled = true
	}
	return handled
}

func GetNews(c *gin.Context) {
	client := newsapi.NewClient(os.Getenv("NEWS_API_KEY"))
	articles, err := client.GetTopHeadlines()
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusOK, "news.tmpl.html", gin.H{
			"error": err.Error(),
			BaseURL: baseUrl,
		})
	} else {
		c.HTML(http.StatusOK, "news.tmpl.html", gin.H{
			"articles": articles,
			BaseURL:    baseUrl,
		})
	}
}

func GetAnalytics(c *gin.Context) {
	db := middleware.GetDB(c)

	submatches := tinyUrlRegexp.FindStringSubmatch(c.Query("url"))
	if len(submatches) < 2 {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			BaseURL: baseUrl,
		})
		return
	}
	slug := submatches[1]

	stats, err := pg.GetURLStats(db, map[string]interface{}{
		"slug": slug,
	})
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
			BaseURL: baseUrl,
		})
		return
	}

	var counter int
	analytics := make([]models.Analytics, 0)

	for _, stat := range stats {
		counter += stat.Counter
		analytics = append(analytics, models.Analytics{
			Country: stat.Country,
			State:   stat.State,
			Count:   stat.Counter,
		})
	}

	// sort in descending order of count
	sort.Slice(analytics, func(i, j int) bool { return analytics[i].Count > analytics[j].Count })

	log.WithFields(log.Fields{
		"url":       c.Query("url"),
		"count":     counter,
		"analytics": analytics,
	}).Info("Returned values")

	c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
		"url":       c.Query("url"),
		"count":     counter,
		"analytics": analytics,
		BaseURL:     baseUrl,
	})
	return
}
