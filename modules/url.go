package modules

import (
	"database/sql"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/middleware"
	"github.com/zirius/url-shortener/models"
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
	base = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456789"
)

func GetHomePage(c *gin.Context) {
	if strings.Contains(c.Request.Host, "api") {
		APIGetURL(c)
		return
	}

	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"baseUrl": baseUrl,
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
		c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
			"error":   "Oops. Something went wrong. Please try again.",
			"baseUrl": baseUrl,
		})
		return
	}
	c.HTML(http.StatusOK, "main.tmpl.html", gin.H{
		"url":     shortened,
		"baseUrl": baseUrl,
	})
}

func Get(c *gin.Context) {
	db := middleware.GetDB(c)

	if handled := handleSpecialRoutes(c); handled {
		return
	}

	slug := c.Param("slug")
	log.WithFields(log.Fields{
		"slug": slug,
	}).Info("Got SLUG")

	urlObj, err := pg.GetURL(db, "", slug)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
	}

	if urlObj != nil {

		urlObj.Counter += 1
		urlObj.AccessIPs = append(urlObj.AccessIPs, c.ClientIP())
		err = pg.UpdateURL(db, urlObj)
		if err != nil {
			c.Error(err)
		}

		c.Redirect(http.StatusFound, urlObj.Url)
		return
	}
	c.Redirect(http.StatusFound, baseUrl)
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
	urls, err := pg.GetURLs(db)
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

		urlObj, err := pg.GetURL(db, url, "")
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}

		if slug != "" {
			urlObjBySlug, err := pg.GetURL(db, "", slug)
			if err != nil && err != sql.ErrNoRows {
				return "", err
			}
			if urlObjBySlug != nil {
				// slug already exists so we generate new slug
				slug = slug + "-" + generateSlug(2)
			}
		} else {
			slug = generateSlug(6)
		}

		if urlObj == nil {
			// New URL
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
		}
		shortened = baseUrl + urlObj.Slug
	}
	log.Info("Shortened URL generated: ", shortened)
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

	if slug == "wakemydyno.txt" {
		c.String(http.StatusOK, "wakemydyno")
		handled = true
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
	return handled
}

func GetAnalytics(c *gin.Context) {
	db := middleware.GetDB(c)
	geoIP := middleware.GetGeoIP(c)

	tinyURLStr := strings.TrimSpace(c.Query("url"))
	if tinyURLStr == "" {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{})
		return
	}

	strSlice := strings.Split(tinyURLStr, "/")
	if len(strSlice) == 0 {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
		})
		return
	}
	slug := strSlice[len(strSlice)-1]
	if slug == "" {
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
		})
		return
	}

	url, err := pg.GetURL(db, "", slug)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
			"error": "Invalid URL. Try again.",
		})
		return
	}

	var countryToCityToCityCount = make(map[string]map[string]int)
	analytics := make([]models.Analytics, 0)

	for _, accessIP := range url.AccessIPs {
		ip := net.ParseIP(accessIP)
		record, err := geoIP.City(ip)
		if err != nil {
			log.WithFields(log.Fields{
				"ip":   ip,
				"slug": url.Slug,
			}).WithError(err).Error("Error getting Geo Info")
			continue
		}
		country := record.Country.Names["en"]
		if _, ok := countryToCityToCityCount[country]; !ok {
			countryToCityToCityCount[country] = make(map[string]int)
		}
		countryToCityToCityCount[country][record.City.Names["en"]] += 1
	}
	for country, cityMap := range countryToCityToCityCount {
		for city, count := range cityMap {
			analytics = append(analytics, models.Analytics{
				Country: country,
				City:    city,
				Count:   count,
			})
		}
	}

	// sort in descending order of count
	sort.Slice(analytics, func(i, j int) bool { return analytics[i].Count > analytics[j].Count })

	log.WithFields(log.Fields{
		"url":       tinyURLStr,
		"count":     url.Counter,
		"analytics": analytics,
	}).Info("Returned values")

	c.HTML(http.StatusOK, "analytics.tmpl.html", gin.H{
		"url":       tinyURLStr,
		"count":     url.Counter,
		"analytics": analytics,
	})
	return
}
