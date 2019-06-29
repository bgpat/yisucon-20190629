package main

import (
	"encoding/json"
	"net/http"
)

func loadFriends(name string) ([]string, error) {
	resp, err := http.DefaultClient.Get(isutomoEndpoint + pathURIEscape("/"+name))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		Result []string `json:"friends"`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data.Result, err
}
