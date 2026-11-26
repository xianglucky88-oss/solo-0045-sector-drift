package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.local/sector-drift/internal/domain"
)

func heapDemo() domain.Request {
	off := func(v int64) *int64 { return &v }
	return domain.Request{
		Sectors: []domain.Sector{
			{ID: "one", Assets: []string{"a"}},
			{ID: "two", Assets: []string{"b"}},
		},
		Assets: []domain.Asset{
			{ID: "z", Bytes: 8, FixedOffset: off(0)},
			{ID: "a", Bytes: 24, Align: 16, After: []string{"z"}, FirstOffset: off(16)},
			{ID: "b", Bytes: 24, Align: 16, After: []string{"z"}},
		},
		Gates: []domain.Gate{{ID: "g", From: "one", To: "two", At: 50, Reach: 100}},
		Route: []domain.Visit{
			{Sector: "one", Position: 1},
			{Sector: "two", Position: 1},
		},
		Policy: domain.Policy{
			Budget: 200, Lookahead: 0, Retain: 0,
			Pinned: []string{"z"},
			Heap:   &domain.HeapPolicy{Capacity: 64},
		},
	}
}

func post(t *testing.T, h http.Handler, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/build", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHeapBuildDownloadAndReplay(t *testing.T) {
	s := New(t.TempDir())
	h := s.Handler()
	status, receipt := post(t, h, heapDemo())
	if status != http.StatusCreated {
		t.Fatalf("build status = %d body = %v", status, receipt)
	}
	path, _ := receipt["artifact"].(string)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d", rec.Code)
	}
	var plan map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	heap, _ := plan["Heap"].(map[string]any)
	if heap == nil || heap["Capacity"].(float64) != 64 {
		t.Fatalf("missing heap block: %v", plan["Heap"])
	}
	frames := plan["Frames"].([]any)
	f0 := frames[0].(map[string]any)
	layout := f0["Layout"].([]any)
	positions := map[string]int64{}
	for _, item := range layout {
		pl := item.(map[string]any)
		positions[pl["Asset"].(string)] = int64(pl["Offset"].(float64))
	}
	if positions["z"] != 0 {
		t.Fatalf("fixed z must stay at 0, got %d", positions["z"])
	}
	if positions["a"] != 16 {
		t.Fatalf("a must honor first-frame offset 16, got %d", positions["a"])
	}
	// Frame 1 must contain an explicit evict/load or move sequence with
	// concrete addresses and keep the fixed pin at zero throughout.
	f1 := frames[1].(map[string]any)
	ops := f1["Operations"].([]any)
	addressed := 0
	for _, item := range ops {
		op := item.(map[string]any)
		if op["From"] != nil || op["To"] != nil {
			addressed++
		}
		if op["Asset"] == "z" && op["Action"] != "load" {
			t.Fatalf("fixed pin z must not be evicted or moved: %v", op)
		}
	}
	if addressed == 0 {
		t.Fatal("frame 1 operations carry no addresses")
	}
}

func TestHeapBuildIsDeterministicOverHTTP(t *testing.T) {
	s := New(t.TempDir())
	h := s.Handler()
	demo := heapDemo()
	status, first := post(t, h, demo)
	if status != http.StatusCreated {
		t.Fatalf("first build = %d", status)
	}
	// The identical document is content-addressed, so the second build returns
	// the existing digest with status 200.
	status, second := post(t, h, demo)
	if status != http.StatusOK {
		t.Fatalf("second build = %d", status)
	}
	if first["digest"] != second["digest"] {
		t.Fatal("identical heap inputs produced different digests")
	}
}

func TestHeapRejectsIllegalFirstOffset(t *testing.T) {
	s := New(t.TempDir())
	demo := heapDemo()
	// b is not mandatory on frame 0, so a first-frame offset is invalid there.
	off := int64(32)
	demo.Assets[2].FirstOffset = &off
	status, body := post(t, s.Handler(), demo)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "first-frame offset") {
		t.Fatalf("unexpected error: %v", body)
	}
}

func TestHeapRejectsOverlappingFixedOffsets(t *testing.T) {
	s := New(t.TempDir())
	demo := heapDemo()
	off := int64(4) // overlaps fixed z at [0,8)
	demo.Assets[1].FixedOffset = &off
	status, _ := post(t, s.Handler(), demo)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
}

func TestLegacyRequestHasNoHeapBlock(t *testing.T) {
	s := New(t.TempDir())
	demo := heapDemo()
	demo.Policy.Heap = nil
	status, receipt := post(t, s.Handler(), demo)
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, receipt["artifact"].(string), nil)
	s.Handler().ServeHTTP(rec, req)
	var plan map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &plan)
	if plan["Heap"] != nil {
		t.Fatal("legacy plan must not contain a Heap block")
	}
}
