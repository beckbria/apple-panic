package levels

import "testing"

func TestLevelProgressionAndBonuses(t *testing.T) {
	if got := MonstersForLevel(1); len(got) != 3 || got[0] != MonsterApple {
		t.Fatalf("Level 1 mismatch: %+v", got)
	}
	if got := MonstersForLevel(2); len(got) != 5 {
		t.Fatalf("Level 2 expected 5 monsters, got %d", len(got))
	}
	if got := MonstersForLevel(3); len(got) != 8 {
		t.Fatalf("Level 3 expected 8 monsters, got %d", len(got))
	}
	if got := MonstersForLevel(4); len(got) != 3 || got[0] != MonsterButterfly || got[1] != MonsterApple {
		t.Fatalf("Level 4 mismatch: %+v", got)
	}
	if got := MonstersForLevel(7); len(got) != 3 || got[0] != MonsterMask || got[1] != MonsterButterfly || got[2] != MonsterApple {
		t.Fatalf("Level 7 mismatch: %+v", got)
	}
	// Group 8 (Level 27): 1 Mask + 7 Butterflies (0 Apples)
	if got := MonstersForLevel(27); len(got) != 8 || got[0] != MonsterMask || got[1] != MonsterButterfly || got[7] != MonsterButterfly {
		t.Fatalf("Level 27 mismatch: %+v", got)
	}
	// Group 9 (Level 30): 2 Masks + 6 Butterflies
	if got := MonstersForLevel(30); len(got) != 8 || got[0] != MonsterMask || got[1] != MonsterMask || got[2] != MonsterButterfly || got[7] != MonsterButterfly {
		t.Fatalf("Level 30 mismatch: %+v", got)
	}
	// Group 14 (Level 45): 7 Masks + 1 Butterfly
	if got := MonstersForLevel(45); len(got) != 8 || got[6] != MonsterMask || got[7] != MonsterButterfly {
		t.Fatalf("Level 45 mismatch: %+v", got)
	}
	lvl48 := MonstersForLevel(48)
	if len(lvl48) != 8 {
		t.Fatalf("Level 48 expected 8 monsters, got %d", len(lvl48))
	}
	for i, m := range lvl48 {
		if m != MonsterMask {
			t.Fatalf("Level 48 slot %d expected Mask, got %v", i, m)
		}
	}

	bonusCases := map[int]int{
		1:  2000,
		3:  2000,
		4:  3000,
		7:  3000,
		8:  4000,
		27: 8000,
		28: 9000,
		48: 9000,
	}
	for lvl, want := range bonusCases {
		if got := StartingBonus(lvl); got != want {
			t.Errorf("StartingBonus(%d) = %d, want %d", lvl, got, want)
		}
	}
}
