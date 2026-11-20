package ui

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"apple-panic/internal/assets"
	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/sim"

	"github.com/hajimehoshi/ebiten/v2"
)

type captureGame struct {
	app    *App
	outDir string
	done   bool
}

func (c *captureGame) Layout(ow, oh int) (int, int) { return 1120, 768 }

func (c *captureGame) Update() error {
	if c.done {
		return ErrQuit
	}
	return nil
}

func (c *captureGame) Draw(screen *ebiten.Image) {
	if c.done {
		return
	}
	c.done = true
	save := func(name string) {
		buf := ebiten.NewImage(1120, 768)
		c.app.Draw(buf)
		f, err := os.Create(filepath.Join(c.outDir, name))
		if err == nil {
			_ = png.Encode(f, buf)
			f.Close()
		}
	}

	// 1. Completed Title Screen with Start Menu
	c.app.skipTitleIntro()
	c.app.screen = ScreenTitle
	save("shot_title.png")

	// 2. Active Gameplay on Level 7 (Mask + Butterfly + Apple + dug hole + alignment guide)
	c.app.world.Level = 7
	c.app.world.Accessibility.ShowAlignmentGuides = true
	c.app.world.RestartCurrentLevel()
	c.app.world.Phase = sim.PhasePlaying
	c.app.world.Score = 2400
	c.app.world.Holes = append(c.app.world.Holes,
		sim.Hole{FloorIdx: 2, X: 52, Stage: 5},
		sim.Hole{FloorIdx: 3, X: 52, Stage: 3},
	)
	c.app.screen = ScreenPlaying
	save("shot_gameplay.png")

	// 3. Pause Menu
	c.app.screen = ScreenPaused
	c.app.menuIndex = 0
	save("shot_pause.png")

	// 4. Controls Customization Menu (showing Control Mode = TOGGLE)
	c.app.cfg.Accessibility.ToggleControls = true
	c.app.returnScreen = ScreenTitle
	c.app.screen = ScreenControls
	c.app.menuIndex = 7
	save("shot_controls.png")

	// 5. Accessibility Settings Menu (Color)
	c.app.cfg.Accessibility.KeepKilledMonstersDead = true
	c.app.cfg.Accessibility.VideoMode = config.VideoModeColor
	c.app.screen = ScreenAccessibility
	c.app.menuIndex = 6
	save("shot_accessibility.png")

	// 6. Monochrome Mode: Accessibility Settings Menu
	c.app.cfg.Accessibility.VideoMode = config.VideoModeMonochrome
	c.app.screen = ScreenAccessibility
	c.app.menuIndex = 6
	save("shot_accessibility_mono.png")

	// 7. Monochrome Mode: Title Screen
	c.app.screen = ScreenTitle
	c.app.menuIndex = 0
	save("shot_title_mono.png")

	// 8. Monochrome Mode: Gameplay
	c.app.screen = ScreenPlaying
	save("shot_gameplay_mono.png")
}

func TestVisualScreens(t *testing.T) {
	outDir := os.Getenv("APPLE_PANIC_SHOT_DIR")
	if outDir == "" {
		t.Skip("APPLE_PANIC_SHOT_DIR not set")
	}
	pack, err := assets.LoadClassicPack()
	if err != nil {
		t.Fatalf("LoadClassicPack: %v", err)
	}
	cfg := config.DefaultConfig()
	app := NewApp(pack, audio.NewBank(false), cfg, filepath.Join(t.TempDir(), "cfg.json"))

	ebiten.SetWindowSize(320, 240)
	err = ebiten.RunGame(&captureGame{app: app, outDir: outDir})
	if err != nil && !errors.Is(err, ErrQuit) {
		t.Fatalf("RunGame: %v", err)
	}
}
