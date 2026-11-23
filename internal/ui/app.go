package ui

import (
	"errors"
	"fmt"
	"image/color"
	"os"
	"slices"
	"time"

	"apple-panic/internal/assets"
	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/levels"
	"apple-panic/internal/sim"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ErrQuit signals a clean exit requested from the title menu.
var ErrQuit = errors.New("quit game")

type Screen int

const (
	ScreenTitle Screen = iota
	ScreenPlaying
	ScreenPaused
	ScreenControls
	ScreenAccessibility
	ScreenCheat
)

var (
	uiGreen  = color.NRGBA{R: 0x14, G: 0xF5, B: 0x3C, A: 0xFF}
	uiOrange = color.NRGBA{R: 0xFF, G: 0x6A, B: 0x3C, A: 0xFF}
	uiBlue   = color.NRGBA{R: 0x14, G: 0xCF, B: 0xF5, A: 0xFF}
	uiPurple = color.NRGBA{R: 0xFF, G: 0x44, B: 0xFD, A: 0xFF}
	uiWhite  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	uiGray   = color.NRGBA{R: 0xA8, G: 0xB2, B: 0xBE, A: 0xFF}
	uiPanel  = color.NRGBA{R: 0x08, G: 0x0C, B: 0x14, A: 0xEC}
	uiDim    = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xB8}
)

var speedChoices = []int{75, 100, 125, 150}

// App implements ebiten.Game and coordinates menus, settings, audio, assets, and simulation.
type App struct {
	pack       *assets.Pack
	sfx        *audio.Bank
	font       *FontRenderer
	cfg        config.Config
	cfgPath    string
	world       *sim.World
	playCanvas  *ebiten.Image
	frameBuffer *ebiten.Image
	monoShader  *ebiten.Shader

	screen       Screen
	returnScreen Screen
	menuIndex    int
	elapsedSec   float64

	// Title intro choreography state
	titleTimer        float64
	playedWhistle     bool
	playedBonk        bool
	titleAnimComplete bool

	// Control rebinding state
	waitingRebind bool
	rebindTarget  config.Action

	// Cheat menu state (triggered by typing "iddqd")
	cheatDetector      cheatCodeDetector
	cheatReturnScreen  Screen
	cheatReturnMenuIdx int
	cheatLevel         int
	cheatLevelTyping   bool
}

// NewApp constructs the Ebitengine game application.
func NewApp(pack *assets.Pack, sfx *audio.Bank, cfg config.Config, cfgPath string) *App {
	cw, ch := pack.CanvasSize()
	sfx.Muted = cfg.Muted
	monoShader, _ := newMonochromeShader()
	app := &App{
		pack:          pack,
		sfx:           sfx,
		font:          NewFontRenderer(),
		cfg:           cfg,
		cfgPath:       cfgPath,
		world:         sim.NewWorld(time.Now().UnixNano(), cfg.HighScore, cfg.Accessibility),
		playCanvas:    ebiten.NewImage(cw, ch),
		monoShader:    monoShader,
		screen:        ScreenTitle,
		cheatDetector: newCheatCodeDetector(cheatCodeSequence),
		cheatLevel:    1,
	}
	// Drain initial SfxLevelStart from pre-constructed world so title screen starts cleanly.
	app.world.DrainSfx()
	return app
}

func (a *App) resetTitleIntro() {
	a.titleTimer = 0
	a.playedWhistle = false
	a.playedBonk = false
	a.titleAnimComplete = false
}

func (a *App) skipTitleIntro() {
	a.titleTimer = 10.0
	a.playedWhistle = true
	a.playedBonk = true
	a.titleAnimComplete = true
}

func (a *App) saveConfig() {
	if a.world != nil && a.world.HighScore > a.cfg.HighScore {
		a.cfg.HighScore = a.world.HighScore
	}
	if a.cfgPath == "" {
		return
	}
	if err := a.cfg.Save(a.cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save config to %s: %v\n", a.cfgPath, err)
	}
}

// Layout returns a crisp 1120x768 (4x 280x192) virtual UI resolution so menus and
// classic/HD graphics both render razor-sharp.
func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	cw, ch := a.pack.CanvasSize()
	if cw < 1120 {
		return 1120, 768
	}
	return cw, ch
}

// Update advances menus or active gameplay at 60 TPS.
func (a *App) Update() error {
	const dt = 1.0 / 60.0
	a.elapsedSec += dt

	if a.screen != ScreenCheat && !(a.screen == ScreenControls && a.waitingRebind) {
		if chars := ebiten.AppendInputChars(nil); len(chars) > 0 {
			if a.cheatDetector.Feed(chars) {
				a.openCheatMenu()
				return nil
			}
		}
	}

	switch a.screen {
	case ScreenTitle:
		return a.updateTitle(dt)
	case ScreenPlaying:
		return a.updatePlaying(dt)
	case ScreenPaused:
		return a.updatePaused()
	case ScreenControls:
		return a.updateControls()
	case ScreenAccessibility:
		return a.updateAccessibility()
	case ScreenCheat:
		return a.updateCheat()
	}
	return nil
}

func (a *App) menuUpPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionUp); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) menuDownPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionDown); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) menuLeftPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionLeft); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) menuRightPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionRight); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) menuConfirmPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionDig); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) updateTitle(dt float64) error {
	if !a.titleAnimComplete {
		a.titleTimer += dt
		if a.titleTimer >= 1.0 && !a.playedWhistle {
			a.playedWhistle = true
			a.sfx.Play(audio.SfxTitleWhistle)
		}
		if a.titleTimer >= 3.85 && !a.playedBonk {
			a.playedBonk = true
			a.sfx.Play(audio.SfxTitleBonk)
		}
		if a.titleTimer >= 4.25 {
			a.titleAnimComplete = true
		}
		// Any menu navigation or confirm key skips straight to the finished title logo
		// without starting the game automatically!
		if a.menuConfirmPressed() || a.menuUpPressed() || a.menuDownPressed() ||
			inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			a.skipTitleIntro()
			return nil
		}
		return nil
	}

	const itemCount = 6
	if a.menuUpPressed() {
		a.menuIndex = (a.menuIndex - 1 + itemCount) % itemCount
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuDownPressed() {
		a.menuIndex = (a.menuIndex + 1) % itemCount
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuConfirmPressed() {
		a.sfx.Play(audio.SfxMenuSelect)
		switch a.menuIndex {
		case 0: // START GAME
			a.world.Accessibility = a.cfg.Accessibility
			a.world.HighScore = a.cfg.HighScore
			a.world.StartNewGame()
			a.screen = ScreenPlaying
		case 1: // CONTROLS
			a.returnScreen = ScreenTitle
			a.menuIndex = 0
			a.waitingRebind = false
			a.screen = ScreenControls
		case 2: // ACCESSIBILITY SETTINGS
			a.returnScreen = ScreenTitle
			a.menuIndex = 0
			a.screen = ScreenAccessibility
		case 3: // SOUND ON/OFF
			a.cfg.Muted = !a.cfg.Muted
			a.sfx.Muted = a.cfg.Muted
			a.saveConfig()
		case 4: // REPLAY INTRO
			a.resetTitleIntro()
		case 5: // QUIT
			a.saveConfig()
			return ErrQuit
		}
	}
	return nil
}

func (a *App) isActionHeld(act config.Action) bool {
	if k, ok := a.cfg.KeyFor(act); ok && ebiten.IsKeyPressed(k) {
		return true
	}
	return false
}

func (a *App) isActionJustPressed(act config.Action) bool {
	if k, ok := a.cfg.KeyFor(act); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) isUnboundSpaceJustPressed() bool {
	if !inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return false
	}
	for _, act := range config.AllActions {
		if k, ok := a.cfg.KeyFor(act); ok && k == ebiten.KeySpace {
			return false
		}
	}
	return true
}

func (a *App) pauseTriggered() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return true
	}
	if k, ok := a.cfg.KeyFor(config.ActionPause); ok && inpututil.IsKeyJustPressed(k) {
		return true
	}
	return false
}

func (a *App) updatePlaying(dt float64) error {
	if a.world.Phase == sim.PhaseGameOver || a.world.Phase == sim.PhaseVictory {
		if a.world.HighScore > a.cfg.HighScore {
			a.cfg.HighScore = a.world.HighScore
			a.saveConfig()
		}
		if a.menuConfirmPressed() {
			a.sfx.Play(audio.SfxMenuSelect)
			a.world.Accessibility = a.cfg.Accessibility
			a.world.StartNewGame()
			return nil
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			a.sfx.Play(audio.SfxMenuSelect)
			a.skipTitleIntro()
			a.menuIndex = 0
			a.screen = ScreenTitle
			return nil
		}
		return nil
	}

	if a.pauseTriggered() {
		a.sfx.Play(audio.SfxMenuSelect)
		a.menuIndex = 0
		a.screen = ScreenPaused
		return nil
	}

	in := sim.InputSnapshot{
		Up:         a.isActionHeld(config.ActionUp),
		Down:       a.isActionHeld(config.ActionDown),
		Left:       a.isActionHeld(config.ActionLeft),
		Right:      a.isActionHeld(config.ActionRight),
		Dig:        a.isActionHeld(config.ActionDig),
		Fill:       a.isActionHeld(config.ActionFill),
		PressUp:    a.isActionJustPressed(config.ActionUp),
		PressDown:  a.isActionJustPressed(config.ActionDown),
		PressLeft:  a.isActionJustPressed(config.ActionLeft),
		PressRight: a.isActionJustPressed(config.ActionRight),
		PressDig:   a.isActionJustPressed(config.ActionDig),
		PressFill:  a.isActionJustPressed(config.ActionFill),
		PressStop:  a.isUnboundSpaceJustPressed(),
	}

	prevHigh := a.cfg.HighScore
	a.world.Update(dt, in)
	for _, sfx := range a.world.DrainSfx() {
		a.sfx.Play(sfx)
	}
	if a.world.HighScore > prevHigh {
		a.cfg.HighScore = a.world.HighScore
		a.saveConfig()
	}
	return nil
}

func (a *App) updatePaused() error {
	if a.pauseTriggered() {
		a.sfx.Play(audio.SfxMenuSelect)
		a.screen = ScreenPlaying
		return nil
	}
	const itemCount = 5
	if a.menuUpPressed() {
		a.menuIndex = (a.menuIndex - 1 + itemCount) % itemCount
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuDownPressed() {
		a.menuIndex = (a.menuIndex + 1) % itemCount
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuConfirmPressed() {
		a.sfx.Play(audio.SfxMenuSelect)
		switch a.menuIndex {
		case 0: // RESUME
			a.screen = ScreenPlaying
		case 1: // CONTROLS
			a.returnScreen = ScreenPaused
			a.menuIndex = 0
			a.waitingRebind = false
			a.screen = ScreenControls
		case 2: // ACCESSIBILITY
			a.returnScreen = ScreenPaused
			a.menuIndex = 0
			a.screen = ScreenAccessibility
		case 3: // RESTART LEVEL
			a.world.Accessibility = a.cfg.Accessibility
			a.world.RestartCurrentLevel()
			a.screen = ScreenPlaying
		case 4: // QUIT TO TITLE
			a.saveConfig()
			a.skipTitleIntro()
			a.menuIndex = 0
			a.screen = ScreenTitle
		}
	}
	return nil
}

func (a *App) updateControls() error {
	if a.waitingRebind {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			a.waitingRebind = false
			a.sfx.Play(audio.SfxMenuMove)
			return nil
		}
		for _, k := range config.AllBindableKeys() {
			if k == ebiten.KeyEscape {
				continue
			}
			if inpututil.IsKeyJustPressed(k) {
				a.cfg.SetBinding(a.rebindTarget, k)
				a.saveConfig()
				a.waitingRebind = false
				a.sfx.Play(audio.SfxMenuSelect)
				return nil
			}
		}
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.sfx.Play(audio.SfxMenuMove)
		a.menuIndex = 0
		a.screen = a.returnScreen
		return nil
	}

	totalItems := len(config.AllActions) + 4 // + Control Mode, Reset Modern, Classic Preset, Back
	if a.menuUpPressed() {
		a.menuIndex = (a.menuIndex - 1 + totalItems) % totalItems
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuDownPressed() {
		a.menuIndex = (a.menuIndex + 1) % totalItems
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuIndex == len(config.AllActions) && (a.menuLeftPressed() || a.menuRightPressed() || a.menuConfirmPressed()) {
		a.cfg.Accessibility.ToggleControls = !a.cfg.Accessibility.ToggleControls
		a.world.Accessibility = a.cfg.Accessibility
		a.saveConfig()
		a.sfx.Play(audio.SfxMenuSelect)
	} else if a.menuConfirmPressed() {
		a.sfx.Play(audio.SfxMenuSelect)
		if a.menuIndex < len(config.AllActions) {
			a.rebindTarget = config.AllActions[a.menuIndex]
			a.waitingRebind = true
		} else {
			switch a.menuIndex - len(config.AllActions) {
			case 1: // Reset to Modern Defaults (WASD + K/J)
				a.cfg.ApplyModernDefaults()
				a.saveConfig()
			case 2: // Classic Apple II Preset (IJKL + A/S)
				a.cfg.ApplyClassicApplePresets()
				a.saveConfig()
			case 3: // Back
				a.menuIndex = 0
				a.screen = a.returnScreen
			}
		}
	}
	return nil
}

func (a *App) updateAccessibility() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.sfx.Play(audio.SfxMenuMove)
		a.menuIndex = 0
		a.screen = a.returnScreen
		return nil
	}

	const totalItems = 8
	if a.menuUpPressed() {
		a.menuIndex = (a.menuIndex - 1 + totalItems) % totalItems
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuDownPressed() {
		a.menuIndex = (a.menuIndex + 1) % totalItems
		a.sfx.Play(audio.SfxMenuMove)
	} else if a.menuLeftPressed() || a.menuRightPressed() || a.menuConfirmPressed() {
		dir := 1
		if a.menuLeftPressed() {
			dir = -1
		}
		a.applyAccessibilityChoice(a.menuIndex, dir, a.menuConfirmPressed())
	}
	return nil
}

func (a *App) applyAccessibilityChoice(index, dir int, confirmed bool) {
	acc := &a.cfg.Accessibility
	switch index {
	case 0:
		acc.ToggleControls = !acc.ToggleControls
		a.sfx.Play(audio.SfxMenuSelect)
	case 1:
		acc.KeepKilledMonstersDead = !acc.KeepKilledMonstersDead
		a.sfx.Play(audio.SfxMenuSelect)
	case 2:
		acc.PauseWhileMonsterFalls = !acc.PauseWhileMonsterFalls
		a.sfx.Play(audio.SfxMenuSelect)
	case 3:
		acc.AutoAlignDigging = !acc.AutoAlignDigging
		a.sfx.Play(audio.SfxMenuSelect)
	case 4:
		acc.ShowAlignmentGuides = !acc.ShowAlignmentGuides
		a.sfx.Play(audio.SfxMenuSelect)
	case 5:
		idx := 1
		for i, sp := range speedChoices {
			if sp == acc.GameSpeedPercent {
				idx = i
				break
			}
		}
		idx = (idx + dir + len(speedChoices)) % len(speedChoices)
		acc.GameSpeedPercent = speedChoices[idx]
		a.sfx.Play(audio.SfxMenuSelect)
	case 6:
		if acc.VideoMode == config.VideoModeMonochrome {
			acc.VideoMode = config.VideoModeColor
		} else {
			acc.VideoMode = config.VideoModeMonochrome
		}
		a.sfx.Play(audio.SfxMenuSelect)
	case 7:
		if confirmed {
			a.sfx.Play(audio.SfxMenuSelect)
			a.menuIndex = 0
			a.screen = a.returnScreen
		}
		return
	default:
		return
	}
	a.world.Accessibility = *acc
	a.saveConfig()
}

// Draw renders the active screen, applying the monochrome green CRT filter when enabled.
func (a *App) Draw(screen *ebiten.Image) {
	if a.cfg.Accessibility.VideoMode == config.VideoModeMonochrome {
		sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
		if a.frameBuffer == nil || a.frameBuffer.Bounds().Dx() != sw || a.frameBuffer.Bounds().Dy() != sh {
			a.frameBuffer = ebiten.NewImage(sw, sh)
		}
		a.drawScene(a.frameBuffer)
		applyMonochromeFilter(screen, a.frameBuffer, a.monoShader)
		return
	}
	a.drawScene(screen)
}

func (a *App) drawScene(screen *ebiten.Image) {
	screen.Fill(color.Black)

	switch a.screen {
	case ScreenTitle:
		a.drawTitleScreen(screen)
	case ScreenPlaying:
		a.drawPlayingScreen(screen)
	case ScreenPaused:
		a.drawPlayingScreen(screen)
		a.drawPauseOverlay(screen)
	case ScreenControls:
		if a.returnScreen == ScreenPaused {
			a.drawPlayingScreen(screen)
		} else {
			a.drawTitleBackdropOnly(screen)
		}
		a.drawControlsOverlay(screen)
	case ScreenAccessibility:
		if a.returnScreen == ScreenPaused {
			a.drawPlayingScreen(screen)
		} else {
			a.drawTitleBackdropOnly(screen)
		}
		a.drawAccessibilityOverlay(screen)
	case ScreenCheat:
		switch a.cheatReturnScreen {
		case ScreenPlaying, ScreenPaused:
			a.drawPlayingScreen(screen)
		case ScreenControls, ScreenAccessibility:
			if a.returnScreen == ScreenPaused {
				a.drawPlayingScreen(screen)
			} else {
				a.drawTitleBackdropOnly(screen)
			}
		default:
			a.drawTitleBackdropOnly(screen)
		}
		a.drawCheatOverlay(screen)
	}
}

func (a *App) blitPlayCanvasToScreen(screen *ebiten.Image) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	cw, ch := a.playCanvas.Bounds().Dx(), a.playCanvas.Bounds().Dy()
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(float64(sw)/float64(cw), float64(sh)/float64(ch))
	screen.DrawImage(a.playCanvas, &op)
}

func (a *App) drawTitleBackdropOnly(screen *ebiten.Image) {
	a.playCanvas.Fill(color.Black)
	p := a.pack
	sf := p.ScaleFactor
	if sf <= 0 {
		sf = 2.0
	}
	p.DrawLogical(a.playCanvas, p.TitleBackdrop, 0, 0, assets.BaseWidth, assets.BaseHeight)
	// Cover the hardcoded Apple II bottom footer banner so we can show live custom controls!
	vector.DrawFilledRect(a.playCanvas, 0, float32(146*sf), float32(assets.BaseWidth*sf), float32(46*sf), color.Black, false)
	vector.DrawFilledRect(a.playCanvas, 0, float32(146*sf), float32(assets.BaseWidth*sf), float32(1.5*sf), uiWhite, false)

	drawAtByteAlign := func(img *ebiten.Image, x, y int) {
		if img == nil {
			return
		}
		// Snap X to 7-pixel Hi-Res byte column boundaries.
		xAligned := (x / 7) * 7
		lw := float64(img.Bounds().Dx()) / sf
		lh := float64(img.Bounds().Dy()) / sf
		p.DrawLogical(a.playCanvas, img, float64(xAligned), float64(y), lw, lh)
	}

	if p.TitleAppleLogo != nil {
		lw := float64(p.TitleAppleLogo.Bounds().Dx()) / sf
		lh := float64(p.TitleAppleLogo.Bounds().Dy()) / sf
		p.DrawLogical(a.playCanvas, p.TitleAppleLogo, 14, 0x41, lw, lh)
	}
	drawAtByteAlign(p.TitleMan, 0xE6, 0x56)
	if len(p.TitleTumblingApple) > 0 {
		drawAtByteAlign(p.TitleTumblingApple[0], 0xE6, 0x4C)
	}
	panicXs := [5]int{0x00, 0x1E, 0x3C, 0x5A, 0x6A}
	for i := 0; i < 5; i++ {
		drawAtByteAlign(p.TitlePanicLetters[i], panicXs[i], 0x64)
	}
	a.blitPlayCanvasToScreen(screen)
}

func (a *App) drawTitleScreen(screen *ebiten.Image) {
	a.playCanvas.Fill(color.Black)
	p := a.pack
	sf := p.ScaleFactor
	if sf <= 0 {
		sf = 2.0
	}

	p.DrawLogical(a.playCanvas, p.TitleBackdrop, 0, 0, assets.BaseWidth, assets.BaseHeight)
	// Replace static bottom banner (Y >= 147) with clean black area + crisp white divider.
	vector.DrawFilledRect(a.playCanvas, 0, float32(147*sf), float32(assets.BaseWidth*sf), float32(45*sf), color.Black, false)
	vector.DrawFilledRect(a.playCanvas, 0, float32(146*sf), float32(assets.BaseWidth*sf), float32(1.5*sf), uiWhite, false)

	drawTitleSpr := func(img *ebiten.Image, x, y float64) {
		if img == nil {
			return
		}
		xAligned := float64((int(x) / 7) * 7)
		lw := float64(img.Bounds().Dx()) / sf
		lh := float64(img.Bounds().Dy()) / sf
		p.DrawLogical(a.playCanvas, img, xAligned, y, lw, lh)
	}

	t := a.titleTimer
	if a.titleAnimComplete {
		t = 10.0
	}

	// 0.1s+: "apple" logo (blinking ascender/descenders from 0.2s..1.0s, then complete "apple")
	if t >= 0.10 {
		logoImg := p.TitleAppleLogo
		if t < 1.00 && (t < 0.20 || int(t*18)%2 == 1) && p.TitleSparkle != nil {
			logoImg = p.TitleSparkle
		}
		if logoImg != nil {
			p.DrawLogical(a.playCanvas, logoImg, 14, 0x41,
				float64(logoImg.Bounds().Dx())/sf,
				float64(logoImg.Bounds().Dy())/sf,
			)
		}
	}
	// 1.0s+: standing character on the right
	if t >= 1.00 {
		drawTitleSpr(p.TitleMan, 0xE6, 0x56)
	}
	// 1.35s..1.65s: apple spawns next to "apple"
	if t >= 1.35 && t < 1.65 && len(p.TitleTumblingApple) > 4 {
		drawTitleSpr(p.TitleTumblingApple[4], 0x69, 0x46)
	}
	// 1.65s..3.85s: parabolic bounce trajectory along TitleArc!
	if t >= 1.65 && t < 3.85 && len(p.TitleArc) > 0 {
		u := (t - 1.65) / (3.85 - 1.65)
		idx := int(u * float64(len(p.TitleArc)))
		if idx >= len(p.TitleArc) {
			idx = len(p.TitleArc) - 1
		}
		step := p.TitleArc[idx]
		if step.FrameIdx >= 0 && step.FrameIdx < len(p.TitleTumblingApple) {
			p.DrawLogical(a.playCanvas, p.TitleTumblingApple[step.FrameIdx],
				step.X, step.Y,
				float64(p.TitleTumblingApple[step.FrameIdx].Bounds().Dx())/sf,
				float64(p.TitleTumblingApple[step.FrameIdx].Bounds().Dy())/sf,
			)
		}
	}
	// 3.85s+: apple rests on the character's head!
	if t >= 3.85 && len(p.TitleTumblingApple) > 0 {
		drawTitleSpr(p.TitleTumblingApple[0], 0xE6, 0x4C)
	}
	// 3.95s+: stamp P-A-N-I-C letters sequentially!
	panicXs := [5]float64{0x00, 0x1E, 0x3C, 0x5A, 0x6A}
	for i := 0; i < 5; i++ {
		if t >= 3.92+float64(i)*0.06 {
			drawTitleSpr(p.TitlePanicLetters[i], panicXs[i], 0x64)
		}
	}

	a.blitPlayCanvasToScreen(screen)

	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0

	// Live control summary banner at the bottom
	modeTag := ""
	if a.cfg.Accessibility.ToggleControls {
		modeTag = " (TOGGLE)"
	}
	ctrlLine := fmt.Sprintf("MOVE: %s/%s/%s/%s%s   DIG: %s   FILL: %s   PAUSE: %s/ESC",
		a.cfg.Bindings[config.ActionUp],
		a.cfg.Bindings[config.ActionLeft],
		a.cfg.Bindings[config.ActionDown],
		a.cfg.Bindings[config.ActionRight],
		modeTag,
		a.cfg.Bindings[config.ActionDig],
		a.cfg.Bindings[config.ActionFill],
		a.cfg.Bindings[config.ActionPause],
	)
	a.font.DrawCentered(screen, ctrlLine, sw/2, sh-84*u, 2.0*u, uiGreen)

	if !a.titleAnimComplete {
		a.font.DrawCentered(screen, "PRESS ENTER OR SPACE TO SKIP INTRO", sw/2, sh-42*u, 2.0*u, uiGray)
		return
	}

	hiLine := fmt.Sprintf("HIGH SCORE: %06d", a.cfg.HighScore)
	a.font.DrawCentered(screen, hiLine, sw/2, sh-42*u, 2.0*u, uiPurple)

	// Interactive Title Menu box positioned cleanly between PANIC and the standing character
	boxW, boxH := 365*u, 270*u
	boxX, boxY := 530*u, 265*u
	drawModalBox(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), uiBlue)

	soundLabel := "SOUND: ON"
	if a.cfg.Muted {
		soundLabel = "SOUND: OFF"
	}
	items := []string{
		"START GAME",
		"CONTROLS",
		"ACCESSIBILITY",
		soundLabel,
		"REPLAY INTRO",
		"QUIT",
	}
	for i, label := range items {
		clr := uiWhite
		prefix := "  "
		if i == a.menuIndex {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+label, boxX+28*u, boxY+26*u+float64(i)*38*u, 2.3*u, clr)
	}
}

func (a *App) drawPlayingScreen(screen *ebiten.Image) {
	a.renderWorldToCanvas()
	a.blitPlayCanvasToScreen(screen)

	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0

	switch a.world.Phase {
	case sim.PhaseReady:
		drawModalBox(screen, float32(sw/2-190*u), float32(sh/2-52*u), float32(380*u), float32(104*u), uiGreen)
		a.font.DrawCentered(screen, fmt.Sprintf("LEVEL %d", a.world.Level), sw/2, sh/2-30*u, 3.0*u, uiWhite)
		a.font.DrawCentered(screen, "GET READY!", sw/2, sh/2+12*u, 2.2*u, uiGreen)

	case sim.PhaseLevelClear:
		drawModalBox(screen, float32(sw/2-210*u), float32(sh/2-52*u), float32(420*u), float32(104*u), uiBlue)
		a.font.DrawCentered(screen, fmt.Sprintf("LEVEL %d CLEAR!", a.world.Level), sw/2, sh/2-26*u, 2.8*u, uiBlue)
		a.font.DrawCentered(screen, "BONUS AWARDED", sw/2, sh/2+14*u, 2.0*u, uiWhite)

	case sim.PhaseSirenFreeze:
		if int(a.elapsedSec*14)%2 == 0 {
			a.font.DrawCentered(screen, "NEEDS MORE HOLES!", sw/2, 24*u, 2.4*u, uiOrange)
		}

	case sim.PhaseGameOver:
		vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)
		drawModalBox(screen, float32(sw/2-260*u), float32(sh/2-115*u), float32(520*u), float32(230*u), uiOrange)
		a.font.DrawCentered(screen, "GAME OVER", sw/2, sh/2-82*u, 3.6*u, uiOrange)
		a.font.DrawCentered(screen, fmt.Sprintf("FINAL SCORE: %06d", a.world.Score), sw/2, sh/2-22*u, 2.2*u, uiWhite)
		a.font.DrawCentered(screen, fmt.Sprintf("REACHED LEVEL %d OF %d", a.world.Level, levels.MaxLevel), sw/2, sh/2+12*u, 2.0*u, uiBlue)
		a.font.DrawCentered(screen, "ENTER/SPACE: PLAY AGAIN    ESC: TITLE", sw/2, sh/2+66*u, 1.8*u, uiGreen)

	case sim.PhaseVictory:
		vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)
		drawModalBox(screen, float32(sw/2-330*u), float32(sh/2-160*u), float32(660*u), float32(320*u), uiGreen)
		a.font.DrawCentered(screen, "CONGRATULATIONS!", sw/2, sh/2-130*u, 3.2*u, uiGreen)
		a.font.DrawCentered(screen, "ALL 48 LEVELS COMPLETED!", sw/2, sh/2-84*u, 2.2*u, uiWhite)
		a.font.DrawCentered(screen, fmt.Sprintf("FINAL SCORE: %06d", a.world.Score), sw/2, sh/2-44*u, 2.4*u, uiOrange)
		a.font.DrawCentered(screen, "ORIGINAL 6502 ENGINE BY T.NAGAWA & S.BEPPU", sw/2, sh/2+12*u, 1.8*u, uiBlue)
		a.font.DrawCentered(screen, "(OKAYAMA UNIVERSITY OF SCIENCE)", sw/2, sh/2+40*u, 1.8*u, uiPurple)
		a.font.DrawCentered(screen, "ENTER/SPACE: PLAY AGAIN    ESC: TITLE", sw/2, sh/2+104*u, 1.8*u, uiGreen)
	}
}

type guideSegment struct {
	X int
	Y int
	H int
}

const (
	brickGirderHeight = 6
	guideDashHeight   = 3
	guideDashPeriod   = 6
	guideSpanTopPad   = 2
)

// alignmentGuideSegments computes non-overlapping vertical dashed guide segments
// beneath open/partially-dug holes. Multiple aligned holes at the same X share a
// single guide starting from the topmost hole, and each inter-floor span uses
// identical dash positions so segments never overlap or clip into girders.
func alignmentGuideSegments(holes []sim.Hole) []guideSegment {
	if len(holes) == 0 {
		return nil
	}
	topFloorByX := make(map[int]int, len(holes))
	xs := make([]int, 0, len(holes))
	for _, h := range holes {
		if h.Stage <= 0 || h.FloorIdx < 0 || h.FloorIdx >= len(sim.BrickFloorYs) {
			continue
		}
		if prev, ok := topFloorByX[h.X]; !ok {
			topFloorByX[h.X] = h.FloorIdx
			xs = append(xs, h.X)
		} else if h.FloorIdx < prev {
			topFloorByX[h.X] = h.FloorIdx
		}
	}
	slices.Sort(xs)

	var segs []guideSegment
	for _, x := range xs {
		for f := topFloorByX[x]; f < len(sim.BrickFloorYs); f++ {
			spanTop := sim.BrickFloorYs[f] + brickGirderHeight
			spanBot := sim.GroundFloorY
			if f+1 < len(sim.BrickFloorYs) {
				spanBot = sim.BrickFloorYs[f+1]
			}
			for y := spanTop + guideSpanTopPad; y+guideDashHeight <= spanBot; y += guideDashPeriod {
				segs = append(segs, guideSegment{X: x, Y: y, H: guideDashHeight})
			}
		}
	}
	return segs
}

func (a *App) renderWorldToCanvas() {
	dst := a.playCanvas
	dst.Fill(color.Black)
	p := a.pack
	w := a.world
	sf := p.ScaleFactor
	if sf <= 0 {
		sf = 2.0
	}

	// 1. Optional vertical alignment plumb guides beneath holes
	if w.Accessibility.ShowAlignmentGuides {
		guideClr := color.RGBA{R: 0x0E, G: 0x9E, B: 0xC0, A: 0xFF}
		for _, seg := range alignmentGuideSegments(w.Holes) {
			cx := float32(float64(2*seg.X+7) * sf)
			vector.FillRect(dst, cx-float32(0.75*sf), float32(float64(seg.Y)*sf), float32(1.5*sf), float32(float64(seg.H)*sf), guideClr, false)
		}
	}

	// 2. Draw 5 brick girders + bottom ground floor
	for _, fy := range sim.BrickFloorYs {
		p.DrawLogical(dst, p.BrickGirder, 0, float64(fy), assets.BaseWidth, 6)
	}
	p.DrawLogical(dst, p.GroundFloor, 0, float64(sim.GroundFloorY), assets.BaseWidth, 2)

	// 3. Draw hole cutouts on the 5 brick girders
	if nStages := len(p.HoleStages); nStages > 0 {
		for _, h := range w.Holes {
			if h.Stage <= 0 {
				continue
			}
			idx := ((h.Stage - 1) * nStages) / 5
			if h.Stage >= 5 {
				idx = nStages - 1
			}
			if idx < 0 {
				idx = 0
			}
			fy := sim.BrickFloorYs[h.FloorIdx]
			p.DrawLogical(dst, p.HoleStages[idx], float64(2*h.X), float64(fy), 14, 6)
		}
	}

	// 4. Draw ladders (starts at y = YTop - 9, 2px rail, then repeating 2px rung + 2px rail)
	for _, lad := range w.Ladders {
		lx := float64(2 * lad.X)
		y := lad.YTop - 9
		p.DrawLogical(dst, p.LadderRail2, lx, float64(y), 14, 2)
		y += 2
		for y < lad.YBot {
			p.DrawLogical(dst, p.LadderRung2, lx, float64(y), 14, 2)
			y += 2
			p.DrawLogical(dst, p.LadderRail2, lx, float64(y), 14, 2)
			y += 2
		}
	}

	// 5. Draw Oxygen / Time Bar (Y=180..182, depletes left-to-right across 130 2px units)
	depleted := sim.MaxOxygen - w.Oxygen
	if depleted < 0 {
		depleted = 0
	}
	for unit := depleted; unit < sim.MaxOxygen; unit++ {
		clr := uiBlue
		if unit >= 112 { // Rightmost 18 units = Orange/Red danger zone!
			clr = uiOrange
		}
		vector.DrawFilledRect(
			dst,
			float32(float64(unit*2)*sf),
			float32(180.0*sf),
			float32(2.0*sf),
			float32(3.0*sf),
			clr,
			false,
		)
	}

	// 6. Draw HUD Banner (Y=184), Reserve Lives, Score, Bonus, and High Score
	p.DrawLogical(dst, p.HUDBanner, 0, 184, assets.BaseWidth, 8)

	reserveLives := w.Lives - 1
	if w.InfiniteLives {
		reserveLives = 4
	}
	if w.Phase == sim.PhaseGameOver {
		reserveLives = 0
	}
	for i := 0; i < 4; i++ {
		var icon *ebiten.Image
		if i < reserveLives {
			icon = p.LifeIcon
		} else {
			icon = p.Digits[10]
		}
		p.DrawLogical(dst, icon, float64(i*7), 184, 7, 8)
	}
	drawHUDNumber(p, dst, w.Score, 6, 63, 184)
	drawHUDNumber(p, dst, w.Bonus, 4, 161, 184)
	drawHUDNumber(p, dst, w.HighScore, 6, 238, 184)

	monsterAnimKey := func(t levels.MonsterType) string {
		switch t {
		case levels.MonsterApple:
			return "apple_walk"
		case levels.MonsterButterfly:
			return "butterfly_walk"
		default:
			return "mask_walk"
		}
	}

	cx, cy, _, hasCatchingMonster := w.CatchingMonsterPos()

	// 7. Draw Active Monsters (deferring the catching monster until after the player)
	for i := range w.Monsters {
		m := &w.Monsters[i]
		if !m.Active {
			continue
		}
		if hasCatchingMonster && i == w.CatchingMonsterSlot {
			continue
		}
		p.DrawAnimation(dst, monsterAnimKey(m.Type), m.AnimFrame, a.elapsedSec, float64(2*m.X), float64(m.Y))
	}

	// 8. Draw Player (solid underneath catching monster; blink rapidly on oxygen death)
	if w.Phase != sim.PhaseDying || hasCatchingMonster || int(a.elapsedSec*16)%2 == 0 {
		pl := &w.Player
		var animKey string
		step := pl.AnimStep
		switch pl.State {
		case sim.PlayerClimbing:
			animKey = "player_climb"
			step = (pl.Y >> 2) & 1
		case sim.PlayerFalling:
			animKey = "player_fall"
		case sim.PlayerDigging, sim.PlayerFilling:
			if pl.FacingRight {
				animKey = "player_dig_right"
			} else {
				animKey = "player_dig_left"
			}
			if pl.ShovelDown {
				step = 1
			} else {
				step = 0
			}
		case sim.PlayerRunning:
			if pl.FacingRight {
				animKey = "player_run_right"
			} else {
				animKey = "player_run_left"
			}
			step = pl.X & 1
		default:
			if pl.FacingRight {
				animKey = "player_idle_right"
			} else {
				animKey = "player_idle_left"
			}
			step = pl.X & 1
		}
		p.DrawAnimation(dst, animKey, step, a.elapsedSec, float64(2*pl.X), float64(pl.Y))
	}

	// 8b. Draw Catching Monster jumping onto / perched atop the player's head
	if hasCatchingMonster {
		m := &w.Monsters[w.CatchingMonsterSlot]
		// Opaque 14x10 mask behind the monster prevents the player's head and torso from
		// bleeding through the monster's interior while keeping their legs visible below.
		vector.DrawFilledRect(dst, float32((2*cx)*sf), float32(cy*sf), float32(14.0*sf), float32(10.0*sf), color.Black, false)
		p.DrawAnimation(dst, monsterAnimKey(m.Type), m.AnimFrame, a.elapsedSec, 2*cx, cy)
	}

	// 9. Draw Floating Score Popups (e.g. " 100", " 400", " 900", "1800")
	for _, pop := range w.Popups {
		px := float64(2 * pop.X)
		if px > 250 {
			px = 250
		}
		drawHUDNumber(p, dst, pop.Points, 4, px, float64(pop.Y))
	}
}

// drawHUDNumber renders an nDigits right-aligned integer with leading zeros suppressed.
func drawHUDNumber(p *assets.Pack, dst *ebiten.Image, val, nDigits int, x0, y0 float64) {
	if val < 0 {
		val = 0
	}
	digits := make([]int, nDigits)
	v := val
	for i := nDigits - 1; i >= 0; i-- {
		digits[i] = v % 10
		v /= 10
	}
	seenNonZero := false
	for i := 0; i < nDigits; i++ {
		g := digits[i]
		if g == 0 && !seenNonZero && i < nDigits-1 {
			g = 10 // Blank leading zero
		} else {
			seenNonZero = true
		}
		p.DrawLogical(dst, p.Digits[g], x0+float64(i*7), y0, 7, 8)
	}
}

func (a *App) drawPauseOverlay(screen *ebiten.Image) {
	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)

	boxW, boxH := 560*u, 330*u
	boxX, boxY := (sw-boxW)/2, (sh-boxH)/2
	drawModalBox(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), uiBlue)

	a.font.DrawCentered(screen, fmt.Sprintf("PAUSED  -  LEVEL %d", a.world.Level), sw/2, boxY+28*u, 2.6*u, uiBlue)

	items := []string{
		"RESUME GAME",
		"CONTROLS",
		"ACCESSIBILITY SETTINGS",
		"RESTART LEVEL",
		"QUIT TO TITLE",
	}
	for i, label := range items {
		clr := uiWhite
		prefix := "  "
		if i == a.menuIndex {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+label, boxX+44*u, boxY+92*u+float64(i)*42*u, 2.2*u, clr)
	}
}

func (a *App) drawControlsOverlay(screen *ebiten.Image) {
	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)

	boxW, boxH := 780*u, 610*u
	boxX, boxY := (sw-boxW)/2, (sh-boxH)/2
	drawModalBox(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), uiGreen)

	a.font.DrawCentered(screen, "CUSTOMIZE CONTROLS", sw/2, boxY+24*u, 2.8*u, uiGreen)
	if a.waitingRebind {
		prompt := fmt.Sprintf("PRESS NEW KEY FOR %s  (ESC TO CANCEL)", config.ActionLabel(a.rebindTarget))
		a.font.DrawCentered(screen, prompt, sw/2, boxY+64*u, 1.9*u, uiOrange)
	} else {
		a.font.DrawCentered(screen, "SELECT AN ACTION AND PRESS ENTER TO REBIND", sw/2, boxY+64*u, 1.8*u, uiGray)
	}

	rowY := boxY + 106*u
	for i, act := range config.AllActions {
		selected := (i == a.menuIndex)
		clr := uiWhite
		prefix := "  "
		if selected {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+config.ActionLabel(act), boxX+56*u, rowY, 2.1*u, clr)

		badge := fmt.Sprintf("[ %s ]", a.cfg.Bindings[act])
		badgeClr := uiBlue
		if selected && a.waitingRebind {
			badge = "[ ... ]"
			badgeClr = uiOrange
		}
		a.font.DrawShadowed(screen, badge, boxX+480*u, rowY, 2.1*u, badgeClr)
		rowY += 38 * u
	}

	// Control Mode (Held vs Toggle) row
	{
		modeIdx := len(config.AllActions)
		clr := uiWhite
		prefix := "  "
		if a.menuIndex == modeIdx {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+"Control Mode", boxX+56*u, rowY, 2.1*u, clr)
		modeBadge := "[ HELD ]"
		modeClr := uiGray
		if a.cfg.Accessibility.ToggleControls {
			modeBadge = "[ TOGGLE ]"
			modeClr = uiGreen
		}
		a.font.DrawShadowed(screen, modeBadge, boxX+480*u, rowY, 2.1*u, modeClr)
		rowY += 48 * u
	}

	extra := []string{
		"RESET TO DEFAULTS (WASD + K/J)",
		"CLASSIC APPLE II PRESET (IJKL + A/S)",
		"BACK",
	}
	for idx, label := range extra {
		itemIdx := len(config.AllActions) + 1 + idx
		clr := uiWhite
		prefix := "  "
		if itemIdx == a.menuIndex {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+label, boxX+56*u, rowY+float64(idx)*38*u, 2.0*u, clr)
	}
}

func (a *App) drawAccessibilityOverlay(screen *ebiten.Image) {
	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)

	boxW, boxH := 960*u, 660*u
	boxX, boxY := (sw-boxW)/2, (sh-boxH)/2
	drawModalBox(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), uiPurple)

	a.font.DrawCentered(screen, "ACCESSIBILITY & GAMEPLAY SETTINGS", sw/2, boxY+22*u, 2.6*u, uiPurple)
	a.font.DrawCentered(screen, "USE ENTER OR LEFT/RIGHT TO TOGGLE   ESC: BACK", sw/2, boxY+54*u, 1.7*u, uiGray)

	acc := a.cfg.Accessibility
	type rowSpec struct {
		title string
		val   string
		on    bool
		desc  string
	}
	onOff := func(b bool) string {
		if b {
			return "[ ON ]"
		}
		return "[ OFF ]"
	}
	videoModeVal := "< COLOR >"
	if acc.VideoMode == config.VideoModeMonochrome {
		videoModeVal = "< MONOCHROME >"
	}
	rows := []rowSpec{
		{
			title: "TOGGLE-BASED CONTROLS",
			val:   onOff(acc.ToggleControls),
			on:    acc.ToggleControls,
			desc:  "Tap once to move until an obstacle; queues 1 turn for the next junction.",
		},
		{
			title: "KEEP KILLED MONSTERS DEAD",
			val:   onOff(acc.KeepKilledMonstersDead),
			on:    acc.KeepKilledMonstersDead,
			desc:  "Monsters defeated in the current level stay dead if you lose a life.",
		},
		{
			title: "PAUSE WHILE MONSTERS FALL",
			val:   onOff(acc.PauseWhileMonsterFalls),
			on:    acc.PauseWhileMonsterFalls,
			desc:  "Pauses player and monsters while a monster falls (Apple II original).",
		},
		{
			title: "AUTO-ALIGN DIGGING & FILLING",
			val:   onOff(acc.AutoAlignDigging),
			on:    acc.AutoAlignDigging,
			desc:  "Snaps 1px onto valid hole alignment when pressing Dig or Fill.",
		},
		{
			title: "HOLE ALIGNMENT GUIDES",
			val:   onOff(acc.ShowAlignmentGuides),
			on:    acc.ShowAlignmentGuides,
			desc:  "Shows vertical plumb markers beneath dug holes for 2- and 3-hole traps.",
		},
		{
			title: "GAME SPEED",
			val:   fmt.Sprintf("< %d%% >", acc.GameSpeedPercent),
			on:    acc.GameSpeedPercent != 100,
			desc:  "Scales simulation and monster speed (100% = authentic Apple II speed).",
		},
		{
			title: "VIDEO MODE",
			val:   videoModeVal,
			on:    acc.VideoMode == config.VideoModeMonochrome,
			desc:  "Switches between Apple II color and 1980s green monochrome monitor.",
		},
	}

	y := boxY + 90*u
	for i, r := range rows {
		clr := uiWhite
		prefix := "  "
		if i == a.menuIndex {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+r.title, boxX+44*u, y, 2.1*u, clr)
		valClr := uiGray
		if r.on {
			valClr = uiGreen
		}
		valW := float64(len(r.val)) * 8.0 * 2.1 * u
		a.font.DrawShadowed(screen, r.val, boxX+boxW-44*u-valW, y, 2.1*u, valClr)
		a.font.DrawShadowed(screen, r.desc, boxX+76*u, y+24*u, 1.35*u, uiBlue)
		y += 72 * u
	}

	backClr := uiWhite
	backPrefix := "  "
	if a.menuIndex == len(rows) {
		backClr = uiOrange
		backPrefix = "> "
	}
	a.font.DrawShadowed(screen, backPrefix+"BACK", boxX+44*u, y+10*u, 2.2*u, backClr)
}

func drawModalBox(dst *ebiten.Image, x, y, w, h float32, border color.Color) {
	vector.DrawFilledRect(dst, x, y, w, h, uiPanel, false)
	vector.StrokeRect(dst, x, y, w, h, 3, border, false)
	vector.StrokeRect(dst, x+6, y+6, w-12, h-12, 1, uiWhite, false)
}
