package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"apple-panic/internal/assets"
	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/ui"

	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	// On Linux Wayland/XWayland systems without glxinfo installed, Ebitengine v2.10
	// defaults to OpenGL ES over EGL, which can segfault in eglInitialize (#3152).
	// Prefer desktop OpenGL over GLX when glxinfo is absent unless overridden.
	if runtime.GOOS == "linux" && os.Getenv("SteamEnv") == "" {
		if _, err := exec.LookPath("glxinfo"); err != nil {
			_ = os.Setenv("SteamEnv", "1")
		}
	}
}

func main() {
	assetsDir := flag.String("assets", "", "Optional directory containing manifest.json + HD sprite sheets and sfx/*.wav overrides")
	configPath := flag.String("config", config.DefaultConfigPath(), "Path to user settings JSON file")
	flag.Parse()

	pack, err := assets.LoadClassicPack()
	if err != nil {
		log.Fatalf("Failed to load classic Apple Panic assets: %v", err)
	}

	sfx := audio.NewBank(true)

	if *assetsDir != "" {
		if err := pack.ApplyExternalOverrides(*assetsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load asset overrides from %s: %v\n", *assetsDir, err)
		}
		sfx.LoadExternalOverrides(*assetsDir)
	}

	cfg := config.Load(*configPath)
	app := ui.NewApp(pack, sfx, cfg, *configPath)

	ebiten.SetWindowSize(1120, 768)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Apple Panic")

	if err := ebiten.RunGame(app); err != nil && !errors.Is(err, ui.ErrQuit) {
		log.Fatal(err)
	}
}
