package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/oschwald/geoip2-golang"
)

func GeoIP() gin.HandlerFunc {
	db, err := geoip2.Open("static/GeoLite2-City.mmdb")
	if err != nil {
		panic(err)
	}

	return func(c *gin.Context) {
		c.Set("GeoIPDB", db)
		c.Next()
	}
}

func GetGeoIP(c *gin.Context) *geoip2.Reader {
	return c.Value("GeoIPDB").(*geoip2.Reader)
}
