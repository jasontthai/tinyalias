package url

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/middleware"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/modules/queue"
	"github.com/zirius/url-shortener/pg"
)

var baseUrl string
var secret string

func init() {
	baseUrl = os.Getenv("BASE_URL")

	// secret in order to use API GET route
	secret = os.Getenv("SECRET")
}

const (
	base          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
	NotFoundQuery = "not-found"
	ThreatQuery   = "threat"
	SlugQuery     = "slug"
	BaseURL       = "baseUrl"
)

func GetHomePage(c *gin.Context) {
	if strings.Contains(c.Request.Host, "api") {
		APIGetURL(c)
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

	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		BaseURL: baseUrl,
		"error": error,
	})
}

func CreateURL(c *gin.Context) {
	url := c.PostForm("URL")
	slug := c.PostForm("SLUG")
	log.WithFields(log.Fields{
		"url":  url,
		"slug": slug,
	}).Info("Got Post Form")

	shortened, err := createURL(c, url, slug)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "main.tmpl.html", gin.H{
			"error": "Oops. Something went wrong. Please try again.",
			BaseURL: baseUrl,
		})
		return
	}

	// Run spam job on new link
	_, qc := middleware.GetQue(c)
	// Dispatch ParseGeoRequestJob
	if err := queue.DispatchDetectSpamJob(qc, url); err != nil {
		log.WithFields(log.Fields{
			"url": url,
		}).WithError(err).Error("error sending spam detect job")
	}

	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"url":   shortened,
		BaseURL: baseUrl,
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

		// return spammed
		if urlObj.Status != "active" {
			c.Redirect(http.StatusFound, fmt.Sprintf("?%v=%v&%v=%v", ThreatQuery, urlObj.Status, SlugQuery, slug))
			return
		}

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

		c.Redirect(http.StatusFound, urlObj.Url)
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/?%v=%v", NotFoundQuery, slug))
	return
}

func APICreateURL(c *gin.Context) {
	url := c.Query("url")
	slug := c.Query("alias")
	log.WithFields(log.Fields{
		"url":  url,
		"slug": slug,
	}).Info("Got Queries")

	shortened, err := createURL(c, url, slug)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Run spam job on new link
	_, qc := middleware.GetQue(c)
	// Dispatch ParseGeoRequestJob
	if err := queue.DispatchDetectSpamJob(qc, url); err != nil {
		log.WithFields(log.Fields{
			"url": url,
		}).WithError(err).Error("error sending spam detect job")
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"short":    shortened,
		"original": url,
	})
}

func APIGetURL(c *gin.Context) {
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

func createURL(c *gin.Context, url, slug string) (string, error) {
	db := middleware.GetDB(c)

	var shortened string
	if url != "" {
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
			slug = generateSlug(6)
		}

		urlObj = &models.URL{
			Url:     url,
			Slug:    slug,
			Created: time.Now(),
			IP:      c.ClientIP(),
		}
		err = pg.CreateURL(db, urlObj)
		if err != nil {
			return "", err
		}
		shortened = baseUrl + urlObj.Slug
	}
	log.WithFields(log.Fields{
		"short":    shortened,
		"original": url,
	}).Info("Shortened URL generated")
	return shortened, nil
}

func generateSlug(size int) string {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	var slug string
	for i := 0; i < size; i++ {
		idx := r.Intn(len(base))
		slug = slug + string(base[idx])
	}
	return slug
}

func handleSpecialRoutes(c *gin.Context) bool {
	slug := c.Param("slug")
	var handled bool

	if strings.Contains(c.Request.Host, "api") {
		if slug == "create" {
			APICreateURL(c)
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
			})
		}
		return true
	}
	if slug == "favicon.ico" {
		c.File("./static/favicon.ico")
		handled = true
	}
	if slug == "robots.txt" {
		c.String(http.StatusOK, "User-agent: *")
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
	return handled
}

func GetAnalytics(c *gin.Context) {
	db := middleware.GetDB(c)

	tinyURLStr := strings.TrimSpace(c.Query("url"))
	if tinyURLStr == "" {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			BaseURL: baseUrl,
		})
		return
	}

	strSlice := strings.Split(tinyURLStr, "/")
	if len(strSlice) == 0 {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
			BaseURL: baseUrl,
		})
		return
	}
	slug := strSlice[len(strSlice)-1]
	if slug == "" {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
			BaseURL: baseUrl,
		})
		return
	}

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
		"url":       tinyURLStr,
		"count":     counter,
		"analytics": analytics,
	}).Info("Returned values")

	c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
		"url":       tinyURLStr,
		"count":     counter,
		"analytics": analytics,
		BaseURL:     baseUrl,
	})
	return
}
