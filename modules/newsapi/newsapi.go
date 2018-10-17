package newsapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/zirius/tinyalias/models"
)

type NewsAPIResponse struct {
	Status      string           `json:"status"`
	Code        string           `json:"code"`
	Message     string           `json:"message"`
	TotalResult int              `json:"totalResults"`
	Articles    []models.Article `json:"articles"`
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) GetTopHeadlines() ([]models.Article, error) {
	url := "https://newsapi.org/v2/top-headlines?country=us"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res NewsAPIResponse
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(res.Message)
	}

	return filterArticle(res.Articles), nil
}

func filterArticle(articles []models.Article) []models.Article {
	var res []models.Article
	for _, a := range articles {
		if a.Content != "" && a.Description != "" && a.Title != "" {
			res = append(res, a)
		}
	}
	return res
}
