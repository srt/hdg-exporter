package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	datatypeFloat  = 1
	datatypeString = 2
	datatypeEnum   = 10
)

type metricInfo struct {
	Desc        *prometheus.Desc
	Meta        meta
	EnumOptions map[string]enumOption
	Id          int
}

type Exporter struct {
	HdgEndpoint string
	Timeout     time.Duration
	UpMetric    *prometheus.Desc
	MetricInfos []metricInfo
	Ids         []int
}

func NewExporter(config Config) *Exporter {
	if (config.HdgEndpoint == "") {
		log.Println("Fatal: Environment variable HDG_ENDPOINT must be set to the URL of WebControl, e.g. http://192.168.1.20")
		os.Exit(1)
	}

	dict, err := loadDict(config.HdgEndpoint, config.Language)
	if err != nil {
		panic(err)
	}

	formats, err := loadFormats(config.HdgEndpoint, config.Language)
	if err != nil {
		panic(err)
	}

	enumOptions, err := loadEnumOptions(dict)
	if err != nil {
		panic(err)
	}

	meta, err := loadMeta(dict, formats)
	if err != nil {
		panic(err)
	}

	var metricInfos []metricInfo
	ids := unique(config.Ids)
	for _, id := range ids {
		if m, ok := meta[id]; ok {
			labels := map[string]string{
				"id":        strconv.Itoa(id),
				"desc1":     m.Desc1,
				"desc2":     m.Desc2,
				"enum":      m.Enum,
				"data_type": strconv.Itoa(m.DataType),
			}
			metricInfos = append(metricInfos, metricInfo{
				Desc: prometheus.NewDesc("hdg_value",
					"Measured value",
					nil, labels),
				Meta:        meta[id],
				EnumOptions: enumOptions[m.Enum],
				Id:          id,
			})
		}
	}

	return &Exporter{
		HdgEndpoint: config.HdgEndpoint,
		Timeout:     config.Timeout,
		UpMetric: prometheus.NewDesc("up",
			"Shows whether HDG is up and available",
			nil, nil,
		),
		MetricInfos: metricInfos,
		Ids:         ids,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.UpMetric
	for _, valueMetric := range e.MetricInfos {
		ch <- valueMetric.Desc
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	values, err := loadValues(e.HdgEndpoint, e.Timeout, e.Ids)
	if err != nil {
		log.Println(err)
		ch <- prometheus.MustNewConstMetric(
			e.UpMetric, prometheus.GaugeValue, 0,
		)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		e.UpMetric, prometheus.GaugeValue, 1,
	)

	for _, metricInfo := range e.MetricInfos {
		id := metricInfo.Id
		meta := metricInfo.Meta
		text := values[id].Text

		var value float64
		if text == "---" {
			value = 0.0
		} else if meta.DataType == datatypeFloat {
			if value, err = strconv.ParseFloat(text, 64); err != nil {
				log.Println(fmt.Sprintf("Unable to parse text '%s' for id %d as float", text, id), err)
				continue
			}
		} else if meta.Format != "" {
			if _, err := fmt.Sscanf(text, meta.Format, &value); err != nil {
				log.Println(fmt.Sprintf("Unable to parse text '%s' for id %d using format '%s'", text, id, meta.Format), err)
				continue
			}
		} else if meta.Enum != "" {
			if enumOption, ok := metricInfo.EnumOptions[text]; ok {
				if value, ok = enumOption.RsiInt.(float64); !ok {
					log.Println(fmt.Sprintf("Unable to parse enum text '%s' for id %d enum '%s' rsiInt '%s'", text, id, meta.Enum, enumOption.RsiInt))
					continue
				}
			} else {
				log.Println(fmt.Sprintf("Unable to parse enum text '%s' for id %d enum '%v' with options %v", text, id, meta.Enum, metricInfo.EnumOptions))
				continue
			}
		} else {
			if value, err = strconv.ParseFloat(text, 64); err != nil {
				log.Println(fmt.Sprintf("Unable to parse text '%s' for id %d data type %d format '%s' enum '%s'", text, id, meta.DataType, meta.Format, meta.Enum))
				continue
			}
		}

		ch <- prometheus.MustNewConstMetric(
			metricInfo.Desc, prometheus.GaugeValue, value,
		)
	}
}

func main() {
	http.Handle("/metrics", promhttp.Handler())

	config, err := loadConfig()
	if err != nil {
		panic(err)
	}

	exporter := NewExporter(config)
	prometheus.MustRegister(exporter)

	log.Printf("Starting server on port %v", config.Port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.Port), nil))
}
