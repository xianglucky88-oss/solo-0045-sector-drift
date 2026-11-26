package domain_test

import (
	"encoding/json"
	"testing"

	"example.local/sector-drift/internal/domain"
)

func leaseRTRequest(budget int64) domain.Request {
	return domain.Request{
		Sectors: []domain.Sector{{ID: "main", Assets: []string{"a"}}},
		Assets: []domain.Asset{
			{ID: "d", Bytes: 10},
			{ID: "e", Bytes: 8},
			{ID: "a", Bytes: 20, After: []string{"d"}},
			{ID: "b", Bytes: 20, After: []string{"d", "e"}},
			{ID: "pin", Bytes: 4},
		},
		Route:  []domain.Visit{{Sector: "main", Position: 0}, {Sector: "main", Position: 1}},
		Policy: domain.Policy{Budget: budget, Pinned: []string{"pin"}},
		Leases: []domain.Lease{{Credential: "c1", Frame: 0, Roots: []string{"b"}, Until: 1}},
	}
}

func leaseRTPlan(t *testing.T, r domain.Request) domain.Plan {
	t.Helper()
	blob, err := domain.Build(r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var p domain.Plan
	if err := json.Unmarshal(blob, &p); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	return p
}

func TestLeaseRTAcceptAndExpire(t *testing.T) {
	r := leaseRTRequest(80)
	p := leaseRTPlan(t, r)
	rep, err := domain.RunReplay(&p, nil, domain.ReplayRequest{Events: []domain.LeaseEvent{
		{Frame: 0, HasFrame: true, Credential: "c1", Kind: domain.EventAcquire},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Results[0].Accepted {
		t.Fatalf("expected acquire accepted: %s", rep.Results[0].Status)
	}
	if !leaseRTContains(rep.Frames[0].Resident, "b") {
		t.Fatalf("b should be resident on acquire frame: %v", rep.Frames[0].Resident)
	}
	// until=1 inclusive: still held frame 1, auto-expired before frame 2 (run through).
	rep2, err := domain.RunReplay(&p, nil, domain.ReplayRequest{
		Through: 2, HasThrough: true,
		Events: []domain.LeaseEvent{
			{Frame: 0, HasFrame: true, Credential: "c1", Kind: domain.EventAcquire},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !leaseRTContains(rep2.Frames[1].Resident, "b") {
		t.Fatalf("b must remain through until frame 1")
	}
	if leaseRTContains(rep2.Frames[2].Resident, "b") {
		t.Fatalf("b must be released after until")
	}
}

func TestLeaseRTConflictEarliestFrame(t *testing.T) {
	// Mandatory 34; b closure union adds b20+e8 => 62. Budget 50 rejects at 0.
	r := leaseRTRequest(50)
	p := leaseRTPlan(t, r)
	rep, err := domain.RunReplay(&p, nil, domain.ReplayRequest{Events: []domain.LeaseEvent{
		{Frame: 0, HasFrame: true, Credential: "c1", Kind: domain.EventAcquire},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res := rep.Results[0]
	if res.Accepted || res.ConflictFrame == nil || *res.ConflictFrame != 0 {
		t.Fatalf("expected conflict at frame 0, got accepted=%v cf=%v", res.Accepted, res.ConflictFrame)
	}
	if leaseRTContains(rep.Frames[0].Resident, "b") {
		t.Fatal("rejected lease closure must not be resident")
	}
}

func TestLeaseRTRefcountHandoff(t *testing.T) {
	r := leaseRTRequest(80)
	r.Leases = []domain.Lease{
		{Credential: "old", Roots: []string{"b"}, Until: 3},
		{Credential: "new", Roots: []string{"b"}, Until: 4},
	}
	p := leaseRTPlan(t, r)
	rep, err := domain.RunReplay(&p, nil, domain.ReplayRequest{Events: []domain.LeaseEvent{
		{Frame: 0, HasFrame: true, Credential: "old", Kind: domain.EventAcquire},
		{Frame: 1, HasFrame: true, Credential: "new", Kind: domain.EventAcquire},
		{Frame: 1, HasFrame: true, Credential: "old", Kind: domain.EventEnd},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f1 := rep.Frames[1]
	for _, op := range f1.Operations {
		if op.Asset == "b" || op.Asset == "d" || op.Asset == "e" {
			t.Fatalf("handoff must not reload/evict shared closure, op=%+v all=%+v", op, f1.Operations)
		}
	}
	if !leaseRTContains(f1.Resident, "b") {
		t.Fatal("b must remain held by new credential")
	}
	// duplicate acquire is not accepted twice
	dup, err := domain.RunReplay(&p, nil, domain.ReplayRequest{Events: []domain.LeaseEvent{
		{Frame: 0, HasFrame: true, Credential: "old", Kind: domain.EventAcquire},
		{Frame: 0, HasFrame: true, Credential: "old", Kind: domain.EventAcquire},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if dup.Results[1].Accepted || !dup.Results[1].Duplicate {
		t.Fatalf("second acquire must be a non-accepted duplicate: %+v", dup.Results[1])
	}
}

func leaseRTContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
