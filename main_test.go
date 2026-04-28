package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseRow unit tests
// ---------------------------------------------------------------------------

func TestParseRow_Valid(t *testing.T) {
	fields := make([]string, 9)
	fields[0] = "Lightning Bolt"
	fields[6] = "4"
	fields[8] = "abc-123"

	name, id, qty, err := parseRow(fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Lightning Bolt" {
		t.Errorf("want name %q, got %q", "Lightning Bolt", name)
	}
	if id != "abc-123" {
		t.Errorf("want id %q, got %q", "abc-123", id)
	}
	if qty != 4 {
		t.Errorf("want qty 4, got %d", qty)
	}
}

func TestParseRow_TooFewColumns(t *testing.T) {
	_, _, _, err := parseRow([]string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for too few columns")
	}
}

func TestParseRow_BadQuantity(t *testing.T) {
	fields := make([]string, 9)
	fields[6] = "notanumber"
	_, _, _, err := parseRow(fields)
	if err == nil {
		t.Fatal("expected error for non-numeric quantity")
	}
}

func TestParseRow_WhitespaceQuantity(t *testing.T) {
	fields := make([]string, 9)
	fields[0] = "Island"
	fields[6] = " 20 "
	fields[8] = "xyz"
	_, _, qty, err := parseRow(fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qty != 20 {
		t.Errorf("want 20, got %d", qty)
	}
}

// ---------------------------------------------------------------------------
// callAPI httptest-based tests
// ---------------------------------------------------------------------------

func TestCallAPI_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("want Accept %q, got %q", "application/json", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("expected User-Agent header to be set")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"mana_cost":"{R}","cmc":1}`)) //nolint:errcheck
	}))
	defer srv.Close()

	body, err := callAPI(srv.Client(), srv.URL, "some-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp ScryfallCardResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unexpected json error: %v", err)
	}
	if resp.ManaCost != "{R}" {
		t.Errorf("want ManaCost %q, got %q", "{R}", resp.ManaCost)
	}
}

func TestCallAPI_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := callAPI(srv.Client(), srv.URL, "some-id")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error message, got: %v", err)
	}
}

func TestCallAPI_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{}`)) //nolint:errcheck
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 10 * time.Millisecond}
	_, err := callAPI(client, srv.URL, "some-id")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCallAPI_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not valid json`)) //nolint:errcheck
	}))
	defer srv.Close()

	// callAPI itself succeeds (HTTP 200 with any body); JSON parsing is the caller's job.
	body, err := callAPI(srv.Client(), srv.URL, "some-id")
	if err != nil {
		t.Fatalf("callAPI should succeed on HTTP 200: %v", err)
	}
	var resp ScryfallCardResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		t.Fatal("expected JSON unmarshal error for invalid payload")
	}
}

// ---------------------------------------------------------------------------
// Integration test: CSV input-to-output flow
// ---------------------------------------------------------------------------

// makeTestCSV writes a minimal CSV where index 0=name, 6=quantity, 8=scryfallID.
func makeTestCSV(t *testing.T, rows [][]string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "input*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// header + data rows, 9 columns each
	header := "name,c1,c2,c3,c4,c5,quantity,c7,scryfallID\n"
	if _, err := f.WriteString(header); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if _, err := f.WriteString(strings.Join(row, ",") + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	return f.Name()
}

func TestIntegration_CSVFlow(t *testing.T) {
	apiCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		json.NewEncoder(w).Encode(ScryfallCardResponse{ //nolint:errcheck
			ManaCost:      "{1}{U}",
			Cmc:           2,
			TypeLine:      "Instant",
			OracleText:    "Draw a card.",
			Colors:        []string{"U"},
			ColorIdentity: []string{"U"},
		})
	}))
	defer srv.Close()

	// Two rows sharing the same ScryfallID to verify the cache (only 1 API call expected).
	inputFile := makeTestCSV(t, [][]string{
		{"Ponder", "a", "b", "c", "d", "e", "1", "g", "scry-001"},
		{"Brainstorm", "a", "b", "c", "d", "e", "2", "g", "scry-001"},
	})
	outputFile := inputFile + "_out.csv"
	t.Cleanup(func() { os.Remove(outputFile) })

	cards, err := readCardsCSV(srv.Client(), srv.URL, inputFile, 2)
	if err != nil {
		t.Fatalf("readCardsCSV error: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("want 2 cards, got %d", len(cards))
	}
	if apiCallCount != 1 {
		t.Errorf("want 1 API call (cache hit for duplicate ID), got %d", apiCallCount)
	}

	if err := writeCardsCSV(cards, outputFile); err != nil {
		t.Fatalf("writeCardsCSV error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("could not read output file: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "Ponder") {
		t.Errorf("output missing Ponder: %s", out)
	}
	if !strings.Contains(out, "Brainstorm") {
		t.Errorf("output missing Brainstorm: %s", out)
	}
	if !strings.Contains(out, "{1}{U}") {
		t.Errorf("output missing mana cost: %s", out)
	}
}
