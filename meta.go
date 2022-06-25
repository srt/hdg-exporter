package main

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

var floatFormatRegexp = regexp.MustCompile(`%\d+\.\d+f`)

type meta struct {
	ID        int    `json:"id"`
	Enum      string `json:"enum"`
	DataType  int    `json:"data_type"`
	Desc1     string `json:"desc1"`
	Desc2     string `json:"desc2"`
	Desc      string
	Formatter string `json:"formatter"`
	Format    string
}

//go:embed data.json
var dataFile []byte

func loadMeta(dict map[string]string, formats map[string]string) (map[int]meta, error) {
	metaArray := make([]meta, 0)
	if err := json.Unmarshal(dataFile, &metaArray); err != nil {
		return nil, err
	}

	result := make(map[int]meta)
	for _, d := range metaArray {
		d.Desc1 = desc(d.Desc1, dict)
		d.Desc2 = desc(d.Desc2, dict)
		if d.Desc2 == "" {
			d.Desc = d.Desc1
		} else {
			d.Desc = d.Desc1 + " " + d.Desc2
		}

		if format, ok := formats[d.Formatter]; ok {
			d.Format = sanitizeFormat(format)
		}

		result[d.ID] = d
	}
	return result, nil
}

func desc(s string, dict map[string]string) string {
	return sanitizeDesc(localizeDesc(s, dict))
}

func localizeDesc(s string, dict map[string]string) string {
	if strings.HasPrefix(s, "#") {
		if localized, ok := dict[strings.TrimPrefix(s, "#")]; ok {
			return localized
		} else {
			return s
		}
	} else {
		return s
	}
}

func sanitizeDesc(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func sanitizeFormat(s string) string {
	s = floatFormatRegexp.ReplaceAllString(s, "%f")
	return s
}
