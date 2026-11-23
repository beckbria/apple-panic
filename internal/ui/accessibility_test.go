package ui

import (
	"path/filepath"
	"testing"

	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/sim"
)

func TestAccessibilitySettingsPersistAcrossRuns(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Load(cfgPath)

	app := &App{
		sfx:     audio.NewBank(false),
		cfg:     cfg,
		cfgPath: cfgPath,
		world:   sim.NewWorld(1, cfg.HighScore, cfg.Accessibility),
		screen:  ScreenAccessibility,
	}

	// Toggle every accessibility setting from its default value.
	for i := 0; i <= 6; i++ {
		app.applyAccessibilityChoice(i, 1, true)
	}

	want := config.Accessibility{
		ToggleControls:         false,
		KeepKilledMonstersDead: true,
		PauseWhileMonsterFalls: false,
		AutoAlignDigging:       false,
		ShowAlignmentGuides:    true,
		GameSpeedPercent:       125,
		VideoMode:              config.VideoModeMonochrome,
	}
	if app.cfg.Accessibility != want {
		t.Fatalf("in-memory cfg mismatch:\n got: %+v\nwant: %+v", app.cfg.Accessibility, want)
	}
	if app.world.Accessibility != want {
		t.Fatalf("in-memory world mismatch:\n got: %+v\nwant: %+v", app.world.Accessibility, want)
	}

	// Simulate restarting the application by reloading config from disk.
	reloaded := config.Load(cfgPath)
	if reloaded.Accessibility != want {
		t.Fatalf("persisted config mismatch:\n got: %+v\nwant: %+v", reloaded.Accessibility, want)
	}
}
