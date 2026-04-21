package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"golang.org/x/text/encoding/charmap"
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

//go:embed all:formatters
var formatterFiles embed.FS

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

func loadFormats(language string) (map[string]string, error) {
	val, err := formatterFiles.ReadFile(path.Join("formatters", language+"_formatters.xml"))
	if err != nil {
		return nil, fmt.Errorf("embedded formatter file for language %q not found: %w", language, err)
	}

	var formatters formatters
	decoder := xml.NewDecoder(bytes.NewReader(trimBom(val)))
	decoder.CharsetReader = charsetReader
	if err := decoder.Decode(&formatters); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, object := range formatters.Objects {
		result[object.Name] = object.Format
	}

	return result, nil
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(charset) {
	case "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1.NewDecoder().Reader(input), nil
	default:
		return nil, fmt.Errorf("unsupported charset: %s", charset)
	}
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
