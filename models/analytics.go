package models

type Analytics struct {
	State   string `json:"state"`
	Country string `json:"country"`
	Count   int    `json:"count"`
}
