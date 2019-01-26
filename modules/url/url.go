package url

import (
	"database/sql"
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

	shortened, status, err := createURL(c, url, slug, password, expirationTime, mindful == "true")
	if err != nil {
		if status == http.StatusInternalServerError {
			c.Error(err)
		}
		utils.HandleHtmlResponse(c, status, "main.tmpl.html", gin.H{
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

	urlObj, err := pg.GetURL(db, slug)
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

	shortened, status, err := createURL(c, url, slug, password, expiration, mindful == "true")
	if err != nil {
		if status == http.StatusInternalServerError {
			c.Error(err)
		}
		c.AbortWithStatusJSON(status, gin.H{
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

func APIGetURLs(c *gin.Context) {
	db := middleware.GetDB(c)

	if secret != c.Query("secret") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
		return
	}

	limit, offset, err := utils.GetLimitAndOffsetQueries(c)
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

func createURL(c *gin.Context, url, slug, password string, expiration time.Time, mindful bool) (string, int, error) {
	db := middleware.GetDB(c)
	_, qc := middleware.GetQue(c)

	var shortened string
	if url == "" {
		return shortened, http.StatusOK, nil
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
			return "", http.StatusBadRequest, fmt.Errorf("You are blacklisted.")
		}
	}

	urlObj, err := pg.GetURL(db, slug)
	if err != nil && err != sql.ErrNoRows {
		return "", http.StatusInternalServerError, err
	}
	if urlObj != nil {
		if urlObj.Url == url {
			return utils.BaseUrl + urlObj.Slug, http.StatusOK, nil
		}
		// url already exists with this slug, generate a new slug
		slug = ""
	}

	if slug == "" {
		slug = models.GenerateSlug(6)
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
			return "", http.StatusInternalServerError, err
		}
	}
	if !expiration.Equal(time.Time{}) {
		urlObj.Expired = null.TimeFrom(expiration)
	}

	user := auth.GetAuthenticatedUser(c)
	if user != nil {
		urlObj.Username = user.Username
	}

	// Run spam job on new link
	err = pg.CreateURL(db, urlObj)
	if err != nil {
		return "", http.StatusInternalServerError, err
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
	return shortened, http.StatusOK, nil
}

func handleSpecialRoutes(c *gin.Context) bool {
	slug := c.Param("slug")
	var handled bool = true

	hostname := strings.Split(c.Request.Host, ".")

	if hostname[0] == "api" {
		if slug == "create" {
			APICreateURL(c)
		} else if slug == "status" {
			db := middleware.GetDB(c)
			err := db.Ping()
			if err != nil {
				c.Error(err)
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"status": "NOT OK",
				})
				return true
			}
			c.JSON(http.StatusOK, gin.H{
				"status": "OK",
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
			})
		}
		return true
	}
	switch slug {
	case "shorten":
		CreateURL(c)
	case "favicon.ico":
		c.File("./static/favicon.ico")
	case "robots.txt":
		c.File("./static/robots.txt")
	case "analytics":
		GetAnalytics(c)
	case "privacy-policy":
		c.HTML(http.StatusOK, "privacypolicy.tmpl.html", gin.H{})
	case "api":
		utils.HandleHtmlResponse(c, http.StatusOK, "api.tmpl.html", gin.H{})
	case "news":
		GetNews(c)
	case "auth":
		utils.HandleHtmlResponse(c, http.StatusOK, "auth.tmpl.html", gin.H{})
	case "logout":
		auth.Logout(c)
	default:
		handled = false
	}
	return handled
}

func HandleDeleteLinks(c *gin.Context) {
	db := middleware.GetDB(c)
	slug := c.PostForm("slug")
	urlStr := c.PostForm("url")

	user := auth.GetAuthenticatedUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
		return
	}

	url, err := pg.GetURL(db, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"success": true,
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err,
		})
		return
	}

	if user.Role != models.RoleAdmin && url.Username != user.Username {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
		return
	}

	err = pg.DeleteURL(db, urlStr, slug)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err,
		})
		return
	}

	log.WithField("slug", slug).
		WithField("url", urlStr).
		Info("Deleted URL")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func HandleGetLinks(c *gin.Context) {
	db := middleware.GetDB(c)

	user := auth.GetAuthenticatedUser(c)
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
		})
	}

	limit, offset, err := utils.DataTableGetStartAndLengthQueries(c)
	logEntry := log.WithField("limit", limit).WithField("offset", offset)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	var orderByStr string
	if c.PostForm("order[0][column]") != "" && c.PostForm("order[0][dir]") != "" {
		sortColumnStr := c.PostForm("order[0][column]")
		columnName := c.PostForm(fmt.Sprintf("columns[%v][data]", sortColumnStr))
		orderStr := c.PostForm("order[0][dir]")
		orderByStr = fmt.Sprintf("%v %v", columnName, orderStr)

		logEntry = logEntry.WithField("order_by", orderByStr)
	}
	logEntry.Info("table get")

	drawStr := c.PostForm("draw")
	draw, err := strconv.ParseInt(drawStr, 10, 32)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	searchStr := c.PostForm("search[value]")
	if searchStr != "" {
		// TODO only search for searchable column
		searchStr = fmt.Sprintf(`%%%v%%`, searchStr)
	}

	clauses := make(map[string]interface{})
	countClauses := make(map[string]interface{})

	clauses["_limit"] = limit
	clauses["_offset"] = offset
	clauses["_order_by"] = orderByStr
	clauses["_like"] = searchStr

	// allow admin to query for all urls
	if user.Role != models.RoleAdmin {
		clauses["username"] = user.Username
		countClauses["username"] = user.Username
	}
	urls, err := pg.GetURLs(db, clauses)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	totalCount, err := pg.GetURLCount(db, countClauses)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	//filtered count
	countClauses["_like"] = searchStr
	filteredCount, err := pg.GetURLCount(db, countClauses)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"data":            urls,
		"draw":            draw,
		"recordsTotal":    totalCount,
		"recordsFiltered": filteredCount,
	})
	return
}

func HandleCopySignal(c *gin.Context) {
	url := c.PostForm("copy")

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
	urlObj, err := pg.GetURL(db, slug)
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

	user := auth.GetAuthenticatedUser(c)
	var count int
	var err error
	if user != nil {
		clauses := make(map[string]interface{})

		// Allow admins to query for all urls
		if user.Role != models.RoleAdmin {
			clauses["username"] = user.Username
		}

		count, err = pg.GetURLCount(db, clauses)
		if err != nil {
			c.Error(err)
		}
	}

	submatches := tinyUrlRegexp.FindStringSubmatch(c.Query("url"))
	if len(submatches) < 2 {
		utils.HandleHtmlResponse(c, http.StatusOK, "analytics.tmpl.html", gin.H{
			"count": count,
		})
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
			"count": count,
		})
		return
	}

	var clicks int
	analytics := make([]models.Analytics, 0)

	for _, stat := range stats {
		clicks += stat.Counter
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
		"clicks":    clicks,
		"analytics": analytics,
	}).Info("Returned values")

	utils.HandleHtmlResponse(c, http.StatusOK, "analytics.tmpl.html", gin.H{
		"url":       c.Query("url"),
		"clicks":    clicks,
		"analytics": analytics,
		"count":     count,
	})
	return
}
