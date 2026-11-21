package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Write support is optional and OFF unless HDG_WRITE_ENABLED=true, a token is set and the
// allowlist is non-empty. The allowlist is the only policy: an id that is not listed can never
// be written, and numbers/enum keys outside the configured bounds are rejected before any request
// reaches the heating system. Keep the list minimal; the heating system itself does not check it.

const (
	writeReadbackTimeout  = 8 * time.Second
	writeReadbackInterval = 1 * time.Second
	writeEpsilon          = 0.001
)

// allowEntry is one writable parameter. Type is "number" or "enum".
type allowEntry struct {
	Id   int      `json:"id"`
	Type string   `json:"type"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Step *float64 `json:"step,omitempty"`
	Unit string   `json:"unit,omitempty"`
	Keys []string `json:"keys,omitempty"` // allowed enum keys, in display order
}

// setRequest is the body of POST /api/set: a number in Value, or an enum Key.
type setRequest struct {
	Id    int      `json:"id"`
	Value *float64 `json:"value,omitempty"`
	Key   string   `json:"key,omitempty"`
}

// setResult is returned to the caller and logged by the hub.
type setResult struct {
	Id        int      `json:"id"`
	Status    string   `json:"status"` // applied | pending | rejected
	Requested string   `json:"requested"`
	Confirmed *float64 `json:"confirmed,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type writer struct {
	endpoint string
	timeout  time.Duration
	token    string
	allow    map[int]allowEntry
	meta     map[int]meta
	// enumKeys[enum][key] = rsi int value (for validation and readback comparison)
	enumKeys map[string]map[string]float64
	// enumByDesc[enum][localized desc] = rsi int value (dataRefresh returns the localized text)
	enumByDesc map[string]map[string]float64

	readbackTimeout  time.Duration // how long to wait for the change to be reflected
	readbackInterval time.Duration // poll interval for the read-back
}

func newWriter(config Config, meta map[int]meta, enumOptions map[string]map[string]enumOption) (*writer, error) {
	allow := map[int]allowEntry{}
	if strings.TrimSpace(config.WriteAllow) != "" {
		var entries []allowEntry
		if err := json.Unmarshal([]byte(config.WriteAllow), &entries); err != nil {
			return nil, fmt.Errorf("HDG_WRITE_ALLOW is not valid JSON: %w", err)
		}
		for _, e := range entries {
			if e.Type != "number" && e.Type != "enum" {
				return nil, fmt.Errorf("allowlist id %d: type must be number or enum", e.Id)
			}
			allow[e.Id] = e
		}
	}

	enumKeys := map[string]map[string]float64{}
	enumByDesc := map[string]map[string]float64{}
	for enum, byDesc := range enumOptions {
		enumByDesc[enum] = map[string]float64{}
		enumKeys[enum] = map[string]float64{}
		for desc, eo := range byDesc {
			if f, ok := eo.RsiInt.(float64); ok {
				enumByDesc[enum][desc] = f
				enumKeys[enum][eo.Key] = f
			}
		}
	}

	return &writer{
		endpoint:         config.HdgEndpoint,
		timeout:          config.Timeout,
		token:            config.WriteToken,
		allow:            allow,
		meta:             meta,
		enumKeys:         enumKeys,
		enumByDesc:       enumByDesc,
		readbackTimeout:  writeReadbackTimeout,
		readbackInterval: writeReadbackInterval,
	}, nil
}

func (w *writer) enabled() bool {
	return w.token != "" && len(w.allow) > 0
}

// writable serves the allowlist (GET /api/writable) so a client can render controls.
func (w *writer) writable(rw http.ResponseWriter, _ *http.Request) {
	entries := []allowEntry{}
	for _, e := range w.allow {
		entries = append(entries, e)
	}
	writeJSON(rw, entries)
}

// set validates the request against the allowlist, writes it to the heating system and reads the
// value back (POST /api/set, requires the bearer token). It never writes an id outside the list.
func (w *writer) set(rw http.ResponseWriter, r *http.Request) {
	if !w.authorized(r) {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req setRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "invalid JSON body", http.StatusBadRequest)
		return
	}
	result := w.apply(req)
	if result.Status == "rejected" {
		rw.WriteHeader(http.StatusBadRequest)
	}
	writeJSON(rw, result)
}

func (w *writer) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if w.token == "" || !strings.HasPrefix(h, prefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(h[len(prefix):]), []byte(w.token)) == 1
}

// apply does the validation, the write and the read-back. Separated from set() for testing.
func (w *writer) apply(req setRequest) setResult {
	entry, ok := w.allow[req.Id]
	if !ok {
		return setResult{Id: req.Id, Status: "rejected", Error: "id is not writable"}
	}

	var sendValue string // what we POST as v
	var target float64   // expected numeric value for the read-back
	var requested string // human-readable, for the result/audit

	switch entry.Type {
	case "enum":
		rsi, ok := w.enumKeys[w.meta[req.Id].Enum][req.Key]
		if !ok || !containsString(entry.Keys, req.Key) {
			return setResult{Id: req.Id, Status: "rejected", Error: "enum key not allowed"}
		}
		sendValue, target, requested = req.Key, rsi, req.Key
	case "number":
		if req.Value == nil {
			return setResult{Id: req.Id, Status: "rejected", Error: "value required"}
		}
		v := *req.Value
		if entry.Min != nil && v < *entry.Min || entry.Max != nil && v > *entry.Max {
			return setResult{Id: req.Id, Status: "rejected", Error: "value out of range"}
		}
		if entry.Step != nil && *entry.Step > 0 && math.Abs(math.Remainder(v, *entry.Step)) > writeEpsilon {
			return setResult{Id: req.Id, Status: "rejected", Error: "value not a multiple of step"}
		}
		sendValue = strconv.FormatFloat(v, 'f', -1, 64)
		target, requested = v, sendValue
	default:
		return setResult{Id: req.Id, Status: "rejected", Error: "unknown type"}
	}

	if err := w.sendChange(req.Id, sendValue); err != nil {
		return setResult{Id: req.Id, Status: "rejected", Requested: requested, Error: err.Error()}
	}

	// The heating system applies the change asynchronously; poll the read-back until it matches.
	deadline := time.Now().Add(w.readbackTimeout)
	for {
		if got, ok := w.readback(req.Id); ok {
			if math.Abs(got-target) <= writeEpsilon {
				g := got
				return setResult{Id: req.Id, Status: "applied", Requested: requested, Confirmed: &g}
			}
		}
		if time.Now().After(deadline) {
			return setResult{Id: req.Id, Status: "pending", Requested: requested}
		}
		time.Sleep(w.readbackInterval)
	}
}

func (w *writer) sendChange(id int, value string) error {
	body := url.Values{"i": {strconv.Itoa(id)}, "v": {value}}.Encode()
	req, err := http.NewRequest(http.MethodPost,
		w.endpoint+"/ActionManager.php?action=set_value_changed", strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "hdg-exporter")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	res, err := (&http.Client{Timeout: w.timeout}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HDG returned HTTP %d", res.StatusCode)
	}
	return nil
}

// readback reads the current numeric value of one id (enum via its rsi mapping).
func (w *writer) readback(id int) (float64, bool) {
	values, err := loadValues(w.endpoint, w.timeout, []int{id})
	if err != nil {
		return 0, false
	}
	text := strings.TrimSpace(values[id].Text)
	if text == "" || text == "---" {
		return 0, false
	}
	if enum := w.meta[id].Enum; enum != "" {
		if f, ok := w.enumByDesc[enum][text]; ok {
			return f, true
		}
		return 0, false
	}
	if f, err := strconv.ParseFloat(strings.Fields(text)[0], 64); err == nil {
		return f, true
	}
	return 0, false
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
