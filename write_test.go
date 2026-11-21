package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeBoiler emulates the WebControl: set_value_changed stores a value, dataRefresh returns it.
// Enum values are stored and returned as the localized desc (as the real WebControl does).
type fakeBoiler struct {
	values     map[int]string // id -> current text (localized)
	keyToDesc  map[string]string
	reflectSet bool // if false, set_value_changed is accepted but not reflected (→ pending)
}

func (b *fakeBoiler) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("action") {
		case "set_value_changed":
			r.ParseForm()
			id := r.Form.Get("i")
			v := r.Form.Get("v")
			if b.reflectSet {
				text := v
				if d, ok := b.keyToDesc[v]; ok {
					text = d
				}
				b.values[atoi(id)] = text
			}
			w.WriteHeader(http.StatusOK)
		case "dataRefresh":
			r.ParseForm()
			var out []value
			for _, idstr := range strings.Split(r.Form.Get("nodes"), "-") {
				id := atoi(idstr)
				out = append(out, value{ID: id, Text: b.values[id]})
			}
			json.NewEncoder(w).Encode(out)
		}
	})
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			continue
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func f(v float64) *float64 { return &v }

// testWriter builds a writer for id 6022 (number, 15..24, step 0.5, °C) and
// 20032 (enum BETRIEBSART_CURRENT, keys NORMAL/URLAUB) pointed at the fake boiler.
func testWriter(endpoint string) *writer {
	config := Config{HdgEndpoint: endpoint, Timeout: 2 * time.Second, WriteToken: "s3cret",
		WriteAllow: `[{"id":6022,"type":"number","min":15,"max":24,"step":0.5,"unit":"°C"},
		              {"id":20032,"type":"enum","keys":["NORMAL","URLAUB"]}]`}
	meta := map[int]meta{
		6022:  {ID: 6022, DataType: 2},
		20032: {ID: 20032, DataType: 10, Enum: "BETRIEBSART_CURRENT"},
	}
	enumOptions := map[string]map[string]enumOption{
		"BETRIEBSART_CURRENT": {
			"Normal": {Enum: "BETRIEBSART_CURRENT", Key: "NORMAL", RsiInt: float64(0)},
			"Urlaub": {Enum: "BETRIEBSART_CURRENT", Key: "URLAUB", RsiInt: float64(2)},
		},
	}
	w, err := newWriter(config, meta, enumOptions)
	if err != nil {
		panic(err)
	}
	w.readbackTimeout = 200 * time.Millisecond
	w.readbackInterval = 10 * time.Millisecond
	return w
}

func newFakeBoiler() *fakeBoiler {
	return &fakeBoiler{
		values:     map[int]string{6022: "21 °C", 20032: "Normal"},
		keyToDesc:  map[string]string{"NORMAL": "Normal", "URLAUB": "Urlaub"},
		reflectSet: true,
	}
}

func TestApplyNumberAppliedAndReadBack(t *testing.T) {
	boiler := newFakeBoiler()
	srv := httptest.NewServer(boiler.handler())
	defer srv.Close()

	r := testWriter(srv.URL).apply(setRequest{Id: 6022, Value: f(22.5)})

	if r.Status != "applied" || r.Confirmed == nil || *r.Confirmed != 22.5 {
		t.Fatalf("expected applied/22.5, got %+v", r)
	}
}

func TestApplyEnumSendsKey(t *testing.T) {
	boiler := newFakeBoiler()
	srv := httptest.NewServer(boiler.handler())
	defer srv.Close()

	r := testWriter(srv.URL).apply(setRequest{Id: 20032, Key: "URLAUB"})

	if r.Status != "applied" || r.Confirmed == nil || *r.Confirmed != 2 {
		t.Fatalf("expected applied/2, got %+v", r)
	}
	if boiler.values[20032] != "Urlaub" {
		t.Errorf("boiler should have stored the enum, got %q", boiler.values[20032])
	}
}

func TestApplyRejections(t *testing.T) {
	srv := httptest.NewServer(newFakeBoiler().handler())
	defer srv.Close()
	w := testWriter(srv.URL)

	cases := []struct {
		name string
		req  setRequest
	}{
		{"id not writable", setRequest{Id: 22003, Value: f(50)}},
		{"above max", setRequest{Id: 6022, Value: f(30)}},
		{"below min", setRequest{Id: 6022, Value: f(10)}},
		{"not on step", setRequest{Id: 6022, Value: f(21.3)}},
		{"enum key not allowed", setRequest{Id: 20032, Key: "FROSTSCHUTZ"}},
		{"number without value", setRequest{Id: 6022}},
	}
	for _, c := range cases {
		if r := w.apply(c.req); r.Status != "rejected" {
			t.Errorf("%s: expected rejected, got %+v", c.name, r)
		}
	}
}

func TestApplyPendingWhenNotReflected(t *testing.T) {
	boiler := newFakeBoiler()
	boiler.reflectSet = false
	srv := httptest.NewServer(boiler.handler())
	defer srv.Close()

	r := testWriter(srv.URL).apply(setRequest{Id: 6022, Value: f(22.5)})

	if r.Status != "pending" {
		t.Fatalf("expected pending, got %+v", r)
	}
}

func TestSetRequiresToken(t *testing.T) {
	srv := httptest.NewServer(newFakeBoiler().handler())
	defer srv.Close()
	w := testWriter(srv.URL)

	body, _ := json.Marshal(setRequest{Id: 6022, Value: f(22)})
	for _, tc := range []struct {
		auth string
		code int
	}{
		{"", http.StatusUnauthorized},
		{"Bearer wrong", http.StatusUnauthorized},
		{"Bearer s3cret", http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/set", bytes.NewReader(body))
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		rec := httptest.NewRecorder()
		w.set(rec, req)
		if rec.Code != tc.code {
			t.Errorf("auth %q: expected %d, got %d", tc.auth, tc.code, rec.Code)
		}
	}
}

func TestWritableListsAllowlist(t *testing.T) {
	srv := httptest.NewServer(newFakeBoiler().handler())
	defer srv.Close()
	rec := httptest.NewRecorder()
	testWriter(srv.URL).writable(rec, httptest.NewRequest(http.MethodGet, "/api/writable", nil))

	var got []allowEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
}
