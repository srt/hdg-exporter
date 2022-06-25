package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
)

type formatters struct {
	XMLName xml.Name `xml:"formatters"`
	Objects []object `xml:"object"`
}

type object struct {
	XMLName xml.Name `xml:"object"`
	Name    string   `xml:"name,attr"`
	Format  string   `xml:"format,attr"`
}

func loadDict(hdgEndpoint string, language string) (map[string]string, error) {
	val, err := get(hdgEndpoint + "/data/dictionaries/" + language + ".json")
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	if err := json.Unmarshal(trimBom(val), &result); err != nil {
		return nil, err
	}

	return result, nil
}

func loadFormats(hdgEndpoint string, language string) (map[string]string, error) {
	val, err := get(hdgEndpoint + "/data/dictionaries/" + language + "_formatters.xml")
	if err != nil {
		return nil, err
	}

	var formatters formatters
	if err := xml.Unmarshal(trimBom(val), &formatters); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, object := range formatters.Objects {
		result[object.Name] = object.Format
	}

	return result, nil
}

func trimBom(fileBytes []byte) []byte {
	return bytes.Trim(fileBytes, "\xef\xbb\xbf")
}

func get(url string) ([]byte, error) {
	res, reqErr := http.Get(url)
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

	return resBody, nil
}
