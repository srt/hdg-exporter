package main

import (
	"testing"
	"time"
)

const messagesPage = `<div id="controls"><table id="valueTable">
	<tr>
		<td></td>
		<td style="color:#999; height:42px;">09.11.2026&nbsp;11:12</td>
		<td style="text-align:left;">ER037</td>
		<td style="text-align:left;"><span class="not_text_7">Störung Reinigung</span></td>
		<td style="text-align:right;">Kommt</td>
		<td></td>
	</tr>
	<tr><td></td><td>03.07.2026&nbsp;16:36</td><td>WA002</td><td>Spannungswiederkehr bei Stromausfall</td><td>Geht</td><td></td></tr>
	<tr><td></td><td>broken</td><td>ER001</td><td>x</td><td>Kommt</td><td></td></tr>
	<tr><td></td><td>01.01.2026 10:00</td><td>WT01</td><td>x</td><td>Wartung</td><td></td></tr>
</table></div>`

const maintenancePage = `<table>
	<tr><td></td><td>16.11.2026&nbsp;12:02</td><td>Aschenbehälter leeren (Störung)</td><td></td><td>Wartung</td><td></td></tr>
	<tr><td></td><td>10.11.2026&nbsp;14:55</td><td>Stokerschnecke schmieren</td><td></td><td>Wartung</td><td></td></tr>
	<tr><td></td><td>broken</td><td>x</td><td></td><td>Wartung</td><td></td></tr>
</table>`

func berlin(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParseMessages(t *testing.T) {
	got := parseMessages(messagesPage, berlin(t))
	want := []Message{
		{time.Date(2026, 11, 9, 10, 12, 0, 0, time.UTC), "ER037", "fault", "Störung Reinigung", "on"},
		{time.Date(2026, 7, 3, 14, 36, 0, 0, time.UTC), "WA002", "warning", "Spannungswiederkehr bei Stromausfall", "off"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Timestamp.Equal(want[i].Timestamp) || got[i].Code != want[i].Code || got[i].Kind != want[i].Kind ||
			got[i].Text != want[i].Text || got[i].State != want[i].State {
			t.Errorf("message %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseMaintenance(t *testing.T) {
	got := parseMaintenance(maintenancePage, berlin(t))
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}
	if got[0].Task != "Aschenbehälter leeren (Störung)" || !got[0].Timestamp.Equal(time.Date(2026, 11, 16, 11, 2, 0, 0, time.UTC)) {
		t.Errorf("entry 0: %+v", got[0])
	}
	if got[1].Task != "Stokerschnecke schmieren" {
		t.Errorf("entry 1: %+v", got[1])
	}
}

func TestParseMessagesEmptyPage(t *testing.T) {
	if got := parseMessages("<html></html>", berlin(t)); got == nil || len(got) != 0 {
		t.Errorf("want empty, non-nil slice, got %#v", got)
	}
}
