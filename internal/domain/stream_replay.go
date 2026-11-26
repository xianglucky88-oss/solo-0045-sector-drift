package domain

import (
	"fmt"
	"math"
)

type streamReplayState struct {
	active       Set
	resident     int64
	staged       map[string]int64
	stagingBytes int64
}

func newStreamReplayState() *streamReplayState {
	return &streamReplayState{active: make(Set), staged: make(map[string]int64)}
}

func replayStreamPlan(plan Plan, assets map[string]Asset, consumers map[string][]string,
	budget int64, limits streamLimits) error {

	rs := newStreamReplayState()
	transferCap := limits.transfer
	if transferCap == 0 {
		transferCap = math.MaxInt64
	}
	stagingCap := limits.staging
	if stagingCap == 0 {
		stagingCap = math.MaxInt64
	}
	for fi, frame := range plan.Frames {
		var transferred int64
		for _, op := range frame.Operations {
			a, ok := assets[op.Asset]
			if !ok {
				return fmt.Errorf("frame %d: unknown asset %q", fi, op.Asset)
			}
			switch op.Action {
			case "transfer", "download":
				n := op.Bytes
				if n <= 0 {
					return fmt.Errorf("frame %d: transfer for %s has no byte count", fi, op.Asset)
				}
				if rs.active[a.ID] {
					return fmt.Errorf("frame %d: transfer of active asset %s", fi, a.ID)
				}
				if transferred+n > transferCap {
					return fmt.Errorf("frame %d: transfer quota exceeded by %s", fi, a.ID)
				}
				if rs.stagingBytes+n > stagingCap {
					return fmt.Errorf("frame %d: staging cap exceeded by %s", fi, a.ID)
				}
				if rs.staged[a.ID]+n > a.Bytes {
					return fmt.Errorf("frame %d: transferred more than asset size %s", fi, a.ID)
				}
				transferred += n
				rs.staged[a.ID] += n
				rs.stagingBytes += n
			case "load", "activate":
				if rs.active[a.ID] {
					return fmt.Errorf("frame %d: double activation %s", fi, a.ID)
				}
				if rs.staged[a.ID] != a.Bytes {
					return fmt.Errorf("frame %d: activation before complete transfer %s", fi, a.ID)
				}
				for _, d := range a.After {
					if !rs.active[d] {
						return fmt.Errorf("frame %d: activation %s before dependency %s", fi, a.ID, d)
					}
				}
				if rs.resident+a.Bytes > budget {
					return fmt.Errorf("frame %d: resident cap exceeded activating %s", fi, a.ID)
				}
				rs.active[a.ID] = true
				rs.resident += a.Bytes
				rs.stagingBytes -= a.Bytes
				delete(rs.staged, a.ID)
			case "evict":
				if !rs.active[a.ID] {
					return fmt.Errorf("frame %d: eviction of absent asset %s", fi, a.ID)
				}
				for _, child := range consumers[a.ID] {
					if rs.active[child] {
						return fmt.Errorf("frame %d: eviction orphans %s", fi, child)
					}
				}
				delete(rs.active, a.ID)
				rs.resident -= a.Bytes
			default:
				return fmt.Errorf("frame %d: unknown operation %q", fi, op.Action)
			}
		}
		if rs.resident > budget {
			return fmt.Errorf("frame %d: resident budget exceeded", fi)
		}
		if rs.stagingBytes > stagingCap {
			return fmt.Errorf("frame %d: staging budget exceeded", fi)
		}
		if transferred > transferCap {
			return fmt.Errorf("frame %d: transfer budget exceeded", fi)
		}
		if frame.TransferBytes != 0 && transferred != frame.TransferBytes {
			return fmt.Errorf("frame %d: transfer accounting mismatch", fi)
		}
		for _, id := range frame.Resident {
			if !rs.active[id] {
				return fmt.Errorf("frame %d: Resident does not match replay at %s", fi, id)
			}
		}
		if len(frame.Resident) != len(rs.active) {
			return fmt.Errorf("frame %d: Resident count does not match replay", fi)
		}
	}
	return nil
}
