package ui

import (
	"testing"

	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/levels"
	"apple-panic/internal/sim"
)

func TestCheatCodeDetector(t *testing.T) {
	d := newCheatCodeDetector(cheatCodeSequence)

	// Noise followed by lowercase "iddqd" fed one rune at a time.
	for _, r := range "wasdiddq" {
		if d.Feed([]rune{r}) {
			t.Fatalf("unexpected match before completing iddqd at %q", r)
		}
	}
	if !d.Feed([]rune{'d'}) {
		t.Fatalf("expected match on final 'd' of iddqd")
	}

	// Buffer should reset after match; extra 'd' must not trigger again.
	if d.Feed([]rune{'d'}) {
		t.Fatalf("unexpected repeat match after reset")
	}

	// Case-insensitive match in a single batch.
	if !d.Feed([]rune("xxIDDQD")) {
		t.Fatalf("expected case-insensitive match for IDDQD")
	}

	// Broken sequence followed by valid sequence.
	if d.Feed([]rune("iddqx")) {
		t.Fatalf("unexpected match for broken sequence iddqx")
	}
	if !d.Feed([]rune("IdDqD")) {
		t.Fatalf("expected match for mixed-case IdDqD")
	}
}

func TestApplyLevelDigit(t *testing.T) {
	lvl, typing := 1, false

	// Typing '2' then '4' => 24
	lvl, typing = applyLevelDigit(lvl, typing, 2)
	if lvl != 2 || !typing {
		t.Fatalf("after '2': got (%d, %v), want (2, true)", lvl, typing)
	}
	lvl, typing = applyLevelDigit(lvl, typing, 4)
	if lvl != 24 || typing {
		t.Fatalf("after '4': got (%d, %v), want (24, false)", lvl, typing)
	}

	// Typing '4' then '8' => 48 (MaxLevel)
	lvl, typing = applyLevelDigit(lvl, typing, 4)
	lvl, typing = applyLevelDigit(lvl, typing, 8)
	if lvl != levels.MaxLevel || typing {
		t.Fatalf("after '48': got (%d, %v), want (%d, false)", lvl, typing, levels.MaxLevel)
	}

	// Typing '5' then '9' => 59 > MaxLevel, falls back to 9 (still typing)
	lvl, typing = applyLevelDigit(lvl, typing, 5)
	if lvl != 5 || !typing {
		t.Fatalf("after '5': got (%d, %v), want (5, true)", lvl, typing)
	}
	lvl, typing = applyLevelDigit(lvl, typing, 9)
	if lvl != 9 || !typing {
		t.Fatalf("after '9': got (%d, %v), want (9, true)", lvl, typing)
	}

	// Leading '0' when not typing is ignored
	lvl, typing = applyLevelDigit(12, false, 0)
	if lvl != 12 || typing {
		t.Fatalf("after leading '0': got (%d, %v), want (12, false)", lvl, typing)
	}
}

func TestStepCheatLevel(t *testing.T) {
	if got := stepCheatLevel(1, -1); got != levels.MaxLevel {
		t.Fatalf("stepCheatLevel(1, -1) = %d, want %d", got, levels.MaxLevel)
	}
	if got := stepCheatLevel(levels.MaxLevel, 1); got != 1 {
		t.Fatalf("stepCheatLevel(MaxLevel, 1) = %d, want 1", got)
	}
	if got := stepCheatLevel(3, 5); got != 8 {
		t.Fatalf("stepCheatLevel(3, 5) = %d, want 8", got)
	}
	if got := stepCheatLevel(2, -5); got != levels.MaxLevel-3 {
		t.Fatalf("stepCheatLevel(2, -5) = %d, want %d", got, levels.MaxLevel-3)
	}
}

func TestFormatLevelSummary(t *testing.T) {
	if got := formatLevelSummary(1); got != "3 Apples" {
		t.Fatalf("level 1 summary = %q, want %q", got, "3 Apples")
	}
	if got := formatLevelSummary(4); got != "1 Butterfly, 2 Apples" {
		t.Fatalf("level 4 summary = %q, want %q", got, "1 Butterfly, 2 Apples")
	}
	if got := formatLevelSummary(12); got != "1 Mask, 2 Butterflies, 5 Apples" {
		t.Fatalf("level 12 summary = %q, want %q", got, "1 Mask, 2 Butterflies, 5 Apples")
	}
	if got := formatLevelSummary(48); got != "8 Masks" {
		t.Fatalf("level 48 summary = %q, want %q", got, "8 Masks")
	}
}

func TestCheatMenuLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	app := &App{
		sfx:           audio.NewBank(false),
		cfg:           cfg,
		world:         sim.NewWorld(1, 0, cfg.Accessibility),
		screen:        ScreenPaused,
		menuIndex:     2,
		cheatDetector: newCheatCodeDetector(cheatCodeSequence),
		cheatLevel:    1,
	}
	app.world.Level = 7

	app.openCheatMenu()
	if app.screen != ScreenCheat {
		t.Fatalf("expected ScreenCheat, got %v", app.screen)
	}
	if app.cheatLevel != 7 {
		t.Fatalf("expected cheatLevel initialized to current level 7, got %d", app.cheatLevel)
	}
	if app.menuIndex != 0 {
		t.Fatalf("expected menuIndex reset to 0, got %d", app.menuIndex)
	}

	// Closing returns cleanly to previous screen and menuIndex.
	app.closeCheatMenu()
	if app.screen != ScreenPaused || app.menuIndex != 2 {
		t.Fatalf("expected return to ScreenPaused index 2, got (%v, %d)", app.screen, app.menuIndex)
	}

	// Re-open, select Level 25, toggle infinite lives, and jump.
	app.openCheatMenu()
	app.world.InfiniteLives = true
	app.cheatLevel = 25
	app.jumpToCheatLevel()

	if app.screen != ScreenPlaying {
		t.Fatalf("expected ScreenPlaying after jump, got %v", app.screen)
	}
	if app.world.Level != 25 {
		t.Fatalf("expected world.Level == 25, got %d", app.world.Level)
	}
	if !app.world.InfiniteLives {
		t.Fatalf("expected InfiniteLives to stay true after jump")
	}
}
