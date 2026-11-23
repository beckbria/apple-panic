package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestDefaultsRebindingAndPersistence(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Accessibility.ToggleControls {
		t.Fatalf("expected ToggleControls to default to true")
	}
	if !cfg.Accessibility.PauseWhileMonsterFalls {
		t.Fatalf("expected PauseWhileMonsterFalls to default to true")
	}
	if cfg.Accessibility.VideoMode != VideoModeColor {
		t.Fatalf("expected VideoMode to default to %q, got %q", VideoModeColor, cfg.Accessibility.VideoMode)
	}
	if k, _ := cfg.KeyFor(ActionUp); k != ebiten.KeyW {
		t.Fatalf("expected W for Up, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionLeft); k != ebiten.KeyA {
		t.Fatalf("expected A for Left, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionDown); k != ebiten.KeyS {
		t.Fatalf("expected S for Down, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionRight); k != ebiten.KeyD {
		t.Fatalf("expected D for Right, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionDig); k != ebiten.KeyK {
		t.Fatalf("expected K for Dig, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionFill); k != ebiten.KeyJ {
		t.Fatalf("expected J for Fill, got %v", k)
	}

	// Rebind Dig to W -> should swap Up to K so no duplicate keys exist!
	cfg.SetBinding(ActionDig, ebiten.KeyW)
	if k, _ := cfg.KeyFor(ActionDig); k != ebiten.KeyW {
		t.Fatalf("expected Dig=W, got %v", k)
	}
	if k, _ := cfg.KeyFor(ActionUp); k != ebiten.KeyK {
		t.Fatalf("expected conflict swap Up=K, got %v", k)
	}

	cfg.Accessibility.ToggleControls = false
	cfg.Accessibility.KeepKilledMonstersDead = true
	cfg.Accessibility.PauseWhileMonsterFalls = false
	cfg.Accessibility.AutoAlignDigging = false
	cfg.Accessibility.ShowAlignmentGuides = true
	cfg.Accessibility.GameSpeedPercent = 125
	cfg.Accessibility.VideoMode = VideoModeMonochrome
	cfg.Muted = true
	cfg.HighScore = 12400

	tmp := filepath.Join(t.TempDir(), "nested", "cfg.json")
	if err := cfg.Save(tmp); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded := Load(tmp)
	if loaded.Accessibility != cfg.Accessibility {
		t.Fatalf("loaded accessibility mismatch:\n got: %+v\nwant: %+v", loaded.Accessibility, cfg.Accessibility)
	}
	if !loaded.Muted || loaded.HighScore != 12400 {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}
	if k, _ := loaded.KeyFor(ActionDig); k != ebiten.KeyW {
		t.Fatalf("loaded binding mismatch: %v", k)
	}

	// Verify subsequent saves overwrite cleanly without leaving temp files behind.
	loaded.Accessibility.ShowAlignmentGuides = false
	loaded.Accessibility.VideoMode = VideoModeColor
	loaded.Accessibility.GameSpeedPercent = 75
	if err := loaded.Save(tmp); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}
	reloaded := Load(tmp)
	if reloaded.Accessibility != loaded.Accessibility {
		t.Fatalf("reloaded accessibility mismatch:\n got: %+v\nwant: %+v", reloaded.Accessibility, loaded.Accessibility)
	}
	entries, err := os.ReadDir(filepath.Dir(tmp))
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "cfg.json" {
		t.Fatalf("expected only cfg.json in config dir, got %d entries", len(entries))
	}
}

func TestVideoModeFallback(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "invalid_video_mode.json")
	if err := os.WriteFile(tmp, []byte(`{"accessibility":{"video_mode":"amber"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	loaded := Load(tmp)
	if loaded.Accessibility.VideoMode != VideoModeColor {
		t.Fatalf("expected invalid VideoMode to fall back to %q, got %q", VideoModeColor, loaded.Accessibility.VideoMode)
	}
	if !loaded.Accessibility.PauseWhileMonsterFalls {
		t.Fatalf("expected omitted PauseWhileMonsterFalls to default to true")
	}
}
