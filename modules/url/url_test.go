package url

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zirius/tinyalias/modules/utils"
	"github.com/zirius/tinyalias/test"
)

func TestCreateURL(t *testing.T) {
	router := test.GetTestRouter()
	router.POST("/", CreateURL)
	router.GET("/:slug", Get)
	slug := utils.GenerateSlug(6)

	{
		w := httptest.NewRecorder()
		data := url.Values{}
		data.Add("URL", "example.com")
		data.Add("SLUG", slug)
		req, _ := http.NewRequest("POST", "/", strings.NewReader(data.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/%v", slug), nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 302, w.Code)
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/analytics?url=%v", slug), nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}
}

func TestGetHomePage(t *testing.T) {
	router := test.GetTestRouter()
	router.GET("/", GetHomePage)
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}
}

func TestGetAnalytics(t *testing.T) {
	router := test.GetTestRouter()
	router.GET("/analytics", GetAnalytics)
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/analytics", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}
}

func TestAPICreateURL(t *testing.T) {
	router := test.GetTestRouter()
	router.GET("/create", APICreateURL)
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/create?url=a.com", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		resp, err := ioutil.ReadAll(w.Body)
		assert.Nil(t, err, "Failed to read response body.")

		var rawRes map[string]interface{}
		err = json.Unmarshal(resp, &rawRes)
		assert.Nil(t, err, "Failed to parse response.")
		original := rawRes["original"].(string)
		assert.Equal(t, "a.com", original)
		assert.NotNil(t, rawRes["short"])
		assert.Nil(t, rawRes["password"])
		assert.Nil(t, rawRes["expiration"])
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/create?url=b.com&password=abc", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		resp, err := ioutil.ReadAll(w.Body)
		assert.Nil(t, err, "Failed to read response body.")

		var rawRes map[string]interface{}
		err = json.Unmarshal(resp, &rawRes)
		assert.Nil(t, err, "Failed to parse response.")
		original := rawRes["original"].(string)
		assert.NotNil(t, rawRes["short"])
		assert.Equal(t, "b.com", original)
		password := rawRes["password"].(string)
		assert.Equal(t, "abc", password)
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/create?url=c.com&expiration=1539729574", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		resp, err := ioutil.ReadAll(w.Body)
		assert.Nil(t, err, "Failed to read response body.")

		var rawRes map[string]interface{}
		err = json.Unmarshal(resp, &rawRes)
		assert.Nil(t, err, "Failed to parse response.")
		original := rawRes["original"].(string)
		assert.NotNil(t, rawRes["short"])
		assert.Equal(t, "c.com", original)
		assert.Nil(t, rawRes["password"])
		expiration := rawRes["expiration"].(float64)
		assert.Equal(t, float64(1539729574), expiration)
	}
	{
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/create?url=d.com&alias=somealias&password=abc&expiration=1539729574", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		resp, err := ioutil.ReadAll(w.Body)
		assert.Nil(t, err, "Failed to read response body.")

		var rawRes map[string]interface{}
		err = json.Unmarshal(resp, &rawRes)
		assert.Nil(t, err, "Failed to parse response.")
		assert.Equal(t, "somealias", rawRes["short"].(string))
		assert.Equal(t, "d.com", rawRes["original"].(string))
		assert.Equal(t, "abc", rawRes["password"].(string))
		assert.Equal(t, float64(1539729574), rawRes["expiration"].(float64))
	}
}
