package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

type enumOption struct {
	Enum   string      `json:"enum"`
	Key    string      `json:"key"`
	Desc   string      `json:"desc"`
	RsiInt interface{} `json:"rsi_int"`
}

//go:embed enum_option.json
var enumOptionFile []byte

func loadEnumOptions(dict map[string]string) (map[string]map[string]enumOption, error) {
	enumOptionArray := make([]enumOption, 0)
	if err := json.Unmarshal(enumOptionFile, &enumOptionArray); err != nil {
		return nil, err
	}

	result := make(map[string]map[string]enumOption)
	for _, eo := range enumOptionArray {
		if eo.Key == "" {
			continue
		}

		eo.Desc = localizeDesc(eo.Desc, dict)
		if _, ok := result[eo.Enum]; !ok {
			result[eo.Enum] = make(map[string]enumOption)
		}

		desc := strings.ReplaceAll(eo.Desc, "-\r\n", "&shy;")
		result[eo.Enum][desc] = eo
	}

	return result, nil
}
