package ui

import (
	"slices"
	"testing"

	"apple-panic/internal/sim"
)

func TestAlignmentGuideSegmentsNoOverlapForAlignedHoles(t *testing.T) {
	single := alignmentGuideSegments([]sim.Hole{
		{FloorIdx: 2, X: 52, Stage: 5},
	})
	stacked := alignmentGuideSegments([]sim.Hole{
		{FloorIdx: 4, X: 52, Stage: 5},
		{FloorIdx: 2, X: 52, Stage: 5},
		{FloorIdx: 3, X: 52, Stage: 3},
	})

	if !slices.Equal(single, stacked) {
		t.Fatalf("aligned holes should produce identical segments to topmost hole:\nsingle:  %v\nstacked: %v", single, stacked)
	}
	if len(stacked) == 0 {
		t.Fatalf("expected non-empty segments")
	}

	// Verify no two segments overlap and every segment has consistent height and gap within a span.
	for i, seg := range stacked {
		if seg.H != guideDashHeight {
			t.Fatalf("segment %d has height %d, want %d", i, seg.H, guideDashHeight)
		}
		if seg.Y+seg.H > sim.GroundFloorY {
			t.Fatalf("segment %d [%d,%d) extends past GroundFloorY (%d)", i, seg.Y, seg.Y+seg.H, sim.GroundFloorY)
		}
		for f, fy := range sim.BrickFloorYs {
			if seg.Y < fy+brickGirderHeight && seg.Y+seg.H > fy {
				t.Fatalf("segment %d [%d,%d) overlaps brick floor %d [%d,%d)", i, seg.Y, seg.Y+seg.H, f, fy, fy+brickGirderHeight)
			}
		}
		if i > 0 {
			prev := stacked[i-1]
			if seg.Y < prev.Y+prev.H {
				t.Fatalf("segment %d (%v) overlaps previous segment (%v)", i, seg, prev)
			}
			// Within the same inter-floor span, consecutive dashes must have an exact gap of guideDashPeriod - guideDashHeight.
			gap := seg.Y - (prev.Y + prev.H)
			if gap != guideDashPeriod-guideDashHeight {
				// Only allowed when crossing a brick girder between spans.
				crossedGirder := false
				for _, fy := range sim.BrickFloorYs {
					if prev.Y+prev.H <= fy && seg.Y >= fy+brickGirderHeight {
						crossedGirder = true
						break
					}
				}
				if !crossedGirder {
					t.Fatalf("inconsistent intra-span gap %d between %v and %v", gap, prev, seg)
				}
			}
		}
	}
}

func TestAlignmentGuideSegmentsConsistentPhaseAcrossColumns(t *testing.T) {
	segs := alignmentGuideSegments([]sim.Hole{
		{FloorIdx: 1, X: 20, Stage: 5},
		{FloorIdx: 3, X: 60, Stage: 5},
	})

	var col20Bottom, col60Bottom []int
	bottomSpanTop := sim.BrickFloorYs[4] + brickGirderHeight
	for _, s := range segs {
		if s.Y < bottomSpanTop {
			continue
		}
		switch s.X {
		case 20:
			col20Bottom = append(col20Bottom, s.Y)
		case 60:
			col60Bottom = append(col60Bottom, s.Y)
		}
	}
	if len(col20Bottom) == 0 || !slices.Equal(col20Bottom, col60Bottom) {
		t.Fatalf("expected matching dash Y positions in shared bottom span, got col20=%v col60=%v", col20Bottom, col60Bottom)
	}
}

func TestAlignmentGuideSegmentsIgnoresInvalidHoles(t *testing.T) {
	segs := alignmentGuideSegments([]sim.Hole{
		{FloorIdx: 2, X: 40, Stage: 0},
		{FloorIdx: -1, X: 42, Stage: 5},
		{FloorIdx: len(sim.BrickFloorYs), X: 44, Stage: 5},
	})
	if len(segs) != 0 {
		t.Fatalf("expected no segments for invalid/closed holes, got %v", segs)
	}
}
