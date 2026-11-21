package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // the image is FROM scratch, without zoneinfo
)

// The WebControl renders its logbooks server-side as an HTML table (templates/ntable.php);
// there is no JSON API for them. hdg_set=1 is the main menu, hdg_group=1 "Meldung/Wartung",
// tab 1 the maintenance logbook, tab 2 the message logbook (faults and warnings).
const (
	logbookMaintenanceTab = 1
	logbookMessagesTab    = 2
)

var (
	logbookRow  = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	logbookCell = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	htmlTag     = regexp.MustCompile(`<[^>]+>`)
	whitespace  = regexp.MustCompile(`\s+`)
)

// Message is one entry of the message logbook, e.g. ER037 "Störung Reinigung" coming.
type Message struct {
	Timestamp time.Time `json:"timestamp"`
	Code      string    `json:"code"`  // e.g. ER037 (fault), WA002 (warning)
	Kind      string    `json:"kind"`  // fault | warning | other
	Text      string    `json:"text"`  // localized text
	State     string    `json:"state"` // on (coming) | off (going)
}

// Maintenance is one acknowledged maintenance task, e.g. "Aschenbehälter leeren (Warnung)".
type Maintenance struct {
	Timestamp time.Time `json:"timestamp"`
	Task      string    `json:"task"`
}

type logbookHandler struct {
	endpoint string
	timeout  time.Duration
	location *time.Location
}

func (h logbookHandler) fetch(tab int) (string, error) {
	client := http.Client{Timeout: h.timeout}
	// The WebControl keeps the selected page in the PHP session; a request without a cookie
	// selects it from the query parameters.
	url := fmt.Sprintf("%s/?hdg_set=1&hdg_group=1&hdg_tab=%d", h.endpoint, tab)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "hdg-exporter")
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HDG returned HTTP %d for %s", res.StatusCode, url)
	}
	body, err := io.ReadAll(res.Body)
	return string(body), err
}

func (h logbookHandler) messages(w http.ResponseWriter, _ *http.Request) {
	page, err := h.fetch(logbookMessagesTab)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, parseMessages(page, h.location))
}

func (h logbookHandler) maintenance(w http.ResponseWriter, _ *http.Request) {
	page, err := h.fetch(logbookMaintenanceTab)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, parseMaintenance(page, h.location))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println(err)
	}
}

// logbookRows returns the text of each table row's cells.
func logbookRows(page string) [][]string {
	var rows [][]string
	for _, row := range logbookRow.FindAllStringSubmatch(page, -1) {
		var cells []string
		for _, cell := range logbookCell.FindAllStringSubmatch(row[1], -1) {
			cells = append(cells, cellText(cell[1]))
		}
		rows = append(rows, cells)
	}
	return rows
}

func cellText(s string) string {
	s = html.UnescapeString(htmlTag.ReplaceAllString(s, ""))
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(whitespace.ReplaceAllString(s, " "))
}

// parseTimestamp reads the WebControl's "02.01.2006 15:04" in the heating system's time zone.
func parseTimestamp(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation("02.01.2006 15:04", s, loc)
	return t, err == nil
}

// parseMessages reads the message logbook: [empty, time, code, text, coming/going, empty].
// The state column is localized; "Kommt"/"Geht" (deutsch) and "Coming"/"Going" are recognized.
func parseMessages(page string, loc *time.Location) []Message {
	messages := []Message{}
	for _, c := range logbookRows(page) {
		if len(c) < 5 || c[2] == "" {
			continue
		}
		ts, ok := parseTimestamp(c[1], loc)
		if !ok {
			continue
		}
		var state string
		switch strings.ToLower(c[4]) {
		case "kommt", "coming":
			state = "on"
		case "geht", "going":
			state = "off"
		default:
			continue
		}
		kind := "other"
		if strings.HasPrefix(c[2], "ER") {
			kind = "fault"
		} else if strings.HasPrefix(c[2], "WA") {
			kind = "warning"
		}
		messages = append(messages, Message{Timestamp: ts, Code: c[2], Kind: kind, Text: c[3], State: state})
	}
	return messages
}

// parseMaintenance reads the maintenance logbook: [empty, time, task, empty, "Wartung", empty].
func parseMaintenance(page string, loc *time.Location) []Maintenance {
	entries := []Maintenance{}
	for _, c := range logbookRows(page) {
		if len(c) < 3 || c[2] == "" {
			continue
		}
		ts, ok := parseTimestamp(c[1], loc)
		if !ok {
			continue
		}
		entries = append(entries, Maintenance{Timestamp: ts, Task: c[2]})
	}
	return entries
}
