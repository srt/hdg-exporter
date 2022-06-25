package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type value struct {
	ID         int    `json:"id"`
	Hidden     bool   `json:"hidden"`
	Text       string `json:"text"`
	Background string `json:"background"`
}

func loadValues(hdgEndpoint string, timeout time.Duration, ids []int) (map[int]value, error) {
	url := hdgEndpoint + "/ApiManager.php?action=dataRefresh"

	hdgClient := http.Client{
		Timeout: timeout,
	}

	reqBody := "nodes=" + strings.Trim(strings.Replace(fmt.Sprint(ids), " ", "-", -1), "[]")
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "hdg-exporter")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	res, reqErr := hdgClient.Do(req)
	if reqErr != nil {
		return nil, reqErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	resBody, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	values := make([]value, 0)
	jsonErr := json.Unmarshal(resBody, &values)
	if jsonErr != nil {
		return nil, jsonErr
	}

	result := make(map[int]value)
	for _, v := range values {
		result[v.ID] = v
	}
	return result, nil
}
