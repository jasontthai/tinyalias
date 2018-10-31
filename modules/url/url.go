package url

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	url2 "net/url"
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
	"github.com/zirius/tinyalias/modules/auth"
	"github.com/zirius/tinyalias/modules/newsapi"
	"github.com/zirius/tinyalias/modules/queue"
	"github.com/zirius/tinyalias/modules/utils"
	"github.com/zirius/tinyalias/pg"
)

var secret string
var tinyUrlRegexp *regexp.Regexp

func init() {
	// secret in order to use API GET route
	secret = os.Getenv("SECRET")
	tinyUrlRegexp = regexp.MustCompile(os.Getenv("BASE_URL") + "(.+)")
}

const (
	NotFoundQuery    = "not-found"
	ExpiredQuery     = "expired"
	ThreatQuery      = "threat"
	SlugQuery        = "slug"
	XForwardedHeader = "X-Forwarded-For"
	DefaultLimit     = 20
	DefaultOffset    = 0
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

	utils.HandleHtmlResponse(c, http.StatusOK, "main.tmpl.html", gin.H{
		"error": error,
	})
}

func CreateURL(c *gin.Context) {
	url := c.Query("url")
	slug := c.Query("alias")
	expiration := c.Query("expiration")
	password := c.Query("password")
	mindful := c.Query("mindful")

	var expirationTime time.Time
	var err error
	if expiration != "" {
		// 10/31/2018 1:57 PM
		expirationTime, err = time.Parse("01/02/2006 3:04 PM", expiration)
		if err != nil {
			c.Error(err)
			utils.HandleHtmlResponse(c, http.StatusInternalServerError, "main.tmpl.html", gin.H{
				"error": "Oops. Something went wrong. Please try again.",
			})
			return
		}
	}

	shortened, err := createURL(c, url, slug, password, expirationTime, mindful == "true")
	if err != nil {
		c.Error(err)
		utils.HandleHtmlResponse(c, http.StatusInternalServerError, "main.tmpl.html", gin.H{
			"error":    fmt.Errorf("Something went wrong: %v", err.Error()),
			"original": url,
		})
		return
	}

	utils.HandleHtmlResponse(c, http.StatusOK, "main.tmpl.html", gin.H{
		"url":      shortened,
		"original": url,
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
		// Update from pending to active if link is clicked
		if urlObj.Status == models.Pending {
			urlObj.Status = models.Active
		}
		err = pg.UpdateURL(db, urlObj)
		if err != nil {
			c.Error(err)
		}

		ip := c.ClientIP()
		if c.GetHeader(XForwardedHeader) != "" {
			ip = c.GetHeader(XForwardedHeader)
		}
		// Dispatch ParseGeoRequestJob
		if err := queue.DispatchParseGeoRequestJob(qc, queue.ParseGeoRequest{
			Slug: slug,
			IP:   ip,
		}); err != nil {
			log.WithFields(log.Fields{
				"slug": slug,
				"ip":   ip,
			}).WithError(err).Error("error sending queue job")
		}

		if urlObj.Status == models.Expired || (urlObj.Expired.Valid && urlObj.Expired.Time.Before(time.Now())) {
			c.Redirect(http.StatusFound, fmt.Sprintf("?%v=%v", ExpiredQuery, slug))
			return
		}

		// return spammed
		if urlObj.Status != models.Active {
			c.Redirect(http.StatusFound, fmt.Sprintf("?%v=%v&%v=%v", ThreatQuery, urlObj.Status, SlugQuery, slug))
			return
		}

		if urlObj.Password != "" {
			if c.Query("password") != "" {
				err = models.VerifyPassword(urlObj.Password, c.Query("password"))
				if err != nil {
					utils.HandleHtmlResponse(c, http.StatusOK, "password.tmpl.html", gin.H{
						"error": "Wrong Password. Try Again.",
					})
					return
				}
			} else {
				utils.HandleHtmlResponse(c, http.StatusOK, "password.tmpl.html", gin.H{})
				return
			}
		}

		if urlObj.Mindful {
			utils.HandleHtmlResponse(c, http.StatusOK, "mindful.tmpl.html", gin.H{
				"url": urlObj.Url,
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
	if url.Status != models.Active && url.Status != models.Pending {
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
	db := middleware.GetDB(c)

	if secret != c.Query("secret") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
		return
	}

	limit, offset, err := GetLimitAndOffsetQueries(c)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	clauses := make(map[string]interface{})
	clauses["_limit"] = limit
	clauses["_offset"] = offset
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

	parseURL, err := url2.Parse(url)
	if err != nil {
		c.Error(err)
	}
	if parseURL != nil {
		domain, err := pg.GetDomain(db, strings.ToLower(parseURL.Hostname()))
		if err != nil && err != sql.ErrNoRows {
			c.Error(err)
		}
		if domain != nil && domain.Blacklist {
			return "", fmt.Errorf("You are blacklisted.")
		}
	}

	urlObj, err := pg.GetURL(db, url, slug)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if urlObj != nil {
		return utils.BaseUrl + urlObj.Slug, nil
	}

	if slug == "" {
		slug = utils.GenerateSlug(6)
	}

	ip := c.ClientIP()
	if c.GetHeader(XForwardedHeader) != "" {
		ip = c.GetHeader(XForwardedHeader)
	}

	urlObj = &models.URL{
		Url:     url,
		Slug:    slug,
		Created: time.Now(),
		IP:      ip,
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

	sessionStore := middleware.GetSessionStore(c)

	session, err := sessionStore.Get(c.Request, auth.SessionName)
	if err != nil {
		c.Error(err)
	}

	username, found := session.Values["username"].(string)
	if found && username != "" {
		urlObj.Username = username
	}

	// Run spam job on new link
	err = pg.CreateURL(db, urlObj)
	if err != nil {
		return "", err
	}
	shortened = utils.BaseUrl + urlObj.Slug

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
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
			})
		}
		return true
	}
	if slug == "signal" {
		HandleCopySignal(c)
		handled = true
	}
	if slug == "create" {
		APICreateURL(c)
		handled = true
	}
	if slug == "get" {
		HandleGetLinks(c)
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
		utils.HandleHtmlResponse(c, http.StatusOK, "api.tmpl.html", gin.H{})
		handled = true
	}
	if slug == "news" {
		GetNews(c)
		handled = true
	}
	if slug == "links" {
		GetLinks(c)
		handled = true
	}
	if slug == "auth" {
		utils.HandleHtmlResponse(c, http.StatusOK, "auth.tmpl.html", gin.H{})
		handled = true
	}
	if slug == "logout" {
		auth.Logout(c)
		handled = true
	}
	return handled
}

func GetLinks(c *gin.Context) {
	db := middleware.GetDB(c)
	sessionStore := middleware.GetSessionStore(c)

	session, err := sessionStore.Get(c.Request, auth.SessionName)
	if err != nil {
		c.Error(err)
	}

	username, found := session.Values["username"].(string)
	if !found || username == "" {
		utils.HandleHtmlResponse(c, http.StatusForbidden, "links.tmpl.html", gin.H{
			"count": 0,
		})
		return
	}

	clauses := make(map[string]interface{})
	clauses["username"] = username

	count, err := pg.GetURLCount(db, clauses)
	if err != nil {
		c.Error(err)
	}
	utils.HandleHtmlResponse(c, http.StatusOK, "links.tmpl.html", gin.H{
		"count": count,
	})
}

func HandleGetLinks(c *gin.Context) {
	db := middleware.GetDB(c)

	sessionStore := middleware.GetSessionStore(c)

	session, err := sessionStore.Get(c.Request, auth.SessionName)
	if err != nil {
		c.Error(err)
	}

	username, found := session.Values["username"].(string)
	if !found || username == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
	}

	limit, offset, err := GetLimitAndOffsetQueries(c)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	clauses := make(map[string]interface{})
	clauses["_limit"] = limit
	clauses["_offset"] = offset
	clauses["username"] = username
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
	return
}

func HandleCopySignal(c *gin.Context) {
	url := c.Query("copied")
	log.WithField("url", url).Info("Copied")

	submatches := tinyUrlRegexp.FindStringSubmatch(url)
	if len(submatches) < 2 {
		log.WithField("url", url).Error("Unable to parse slug")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
		})
		return
	}
	slug := submatches[1]

	db := middleware.GetDB(c)
	urlObj, err := pg.GetURL(db, "", slug)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Set status to active if copied
	urlObj.Status = models.Active

	err = pg.UpdateURL(db, urlObj)
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
	})
}

func GetNews(c *gin.Context) {
	client := newsapi.NewClient(os.Getenv("NEWS_API_KEY"))
	articles, err := client.GetTopHeadlines()
	if err != nil {
		c.Error(err)
		utils.HandleHtmlResponse(c, http.StatusOK, "news.tmpl.html", gin.H{
			"error": err.Error(),
		})
	} else {
		utils.HandleHtmlResponse(c, http.StatusOK, "news.tmpl.html", gin.H{
			"articles": articles,
		})
	}
}

func GetAnalytics(c *gin.Context) {
	db := middleware.GetDB(c)

	submatches := tinyUrlRegexp.FindStringSubmatch(c.Query("url"))
	if len(submatches) < 2 {
		utils.HandleHtmlResponse(c, http.StatusOK, "analytics.tmpl.html", gin.H{})
		return
	}
	slug := submatches[1]

	stats, err := pg.GetURLStats(db, map[string]interface{}{
		"slug": slug,
	})
	if err != nil {
		c.Error(err)
		utils.HandleHtmlResponse(c, http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
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

	utils.HandleHtmlResponse(c, http.StatusOK, "analytics.tmpl.html", gin.H{
		"url":       c.Query("url"),
		"count":     counter,
		"analytics": analytics,
	})
	return
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
