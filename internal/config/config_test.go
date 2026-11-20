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

	cfg.Accessibility.KeepKilledMonstersDead = true
	cfg.Accessibility.PauseWhileMonsterFalls = false
	cfg.Accessibility.VideoMode = VideoModeMonochrome
	cfg.HighScore = 12400

	tmp := filepath.Join(t.TempDir(), "cfg.json")
	if err := cfg.Save(tmp); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded := Load(tmp)
	if !loaded.Accessibility.KeepKilledMonstersDead || loaded.HighScore != 12400 {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}
	if loaded.Accessibility.PauseWhileMonsterFalls {
		t.Fatalf("expected loaded PauseWhileMonsterFalls=false, got true")
	}
	if loaded.Accessibility.VideoMode != VideoModeMonochrome {
		t.Fatalf("expected loaded VideoMode=%q, got %q", VideoModeMonochrome, loaded.Accessibility.VideoMode)
	}
	if k, _ := loaded.KeyFor(ActionDig); k != ebiten.KeyW {
		t.Fatalf("loaded binding mismatch: %v", k)
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
