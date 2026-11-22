package ui

import (
	"fmt"
	"strings"
	"unicode"

	"apple-panic/internal/audio"
	"apple-panic/internal/levels"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const cheatCodeSequence = "iddqd"

// cheatCodeDetector matches a case-insensitive sequence of typed characters
// against a target cheat code string.
type cheatCodeDetector struct {
	target []rune
	buf    []rune
}

func newCheatCodeDetector(code string) cheatCodeDetector {
	lower := []rune(strings.ToLower(code))
	return cheatCodeDetector{
		target: lower,
		buf:    make([]rune, 0, len(lower)),
	}
}

func (d *cheatCodeDetector) Reset() {
	d.buf = d.buf[:0]
}

// Feed appends typed runes and returns true as soon as the target sequence is matched.
func (d *cheatCodeDetector) Feed(chars []rune) bool {
	if len(d.target) == 0 {
		return false
	}
	for _, r := range chars {
		lr := unicode.ToLower(r)
		d.buf = append(d.buf, lr)
		if len(d.buf) > len(d.target) {
			copy(d.buf, d.buf[len(d.buf)-len(d.target):])
			d.buf = d.buf[:len(d.target)]
		}
		if len(d.buf) == len(d.target) {
			matched := true
			for i := range d.target {
				if d.buf[i] != d.target[i] {
					matched = false
					break
				}
			}
			if matched {
				d.Reset()
				return true
			}
		}
	}
	return false
}

// applyLevelDigit updates the selected level number when the player types a digit 0..9.
func applyLevelDigit(current int, typing bool, digit int) (int, bool) {
	if digit < 0 || digit > 9 {
		return current, typing
	}
	if !typing {
		if digit >= 1 {
			return digit, true
		}
		return current, false
	}
	candidate := current*10 + digit
	if candidate >= 1 && candidate <= levels.MaxLevel {
		return candidate, false
	}
	if digit >= 1 {
		return digit, true
	}
	return current, false
}

// stepCheatLevel adjusts level by delta, wrapping within 1..levels.MaxLevel.
func stepCheatLevel(current, delta int) int {
	n := levels.MaxLevel
	offset := ((current - 1 + delta) % n + n) % n
	return offset + 1
}

// formatLevelSummary returns a short human-readable monster breakdown for a level.
func formatLevelSummary(level int) string {
	roster := levels.MonstersForLevel(level)
	var masks, butterflies, apples int
	for _, m := range roster {
		switch m {
		case levels.MonsterMask:
			masks++
		case levels.MonsterButterfly:
			butterflies++
		case levels.MonsterApple:
			apples++
		}
	}
	var parts []string
	if masks == 1 {
		parts = append(parts, "1 Mask")
	} else if masks > 1 {
		parts = append(parts, fmt.Sprintf("%d Masks", masks))
	}
	if butterflies == 1 {
		parts = append(parts, "1 Butterfly")
	} else if butterflies > 1 {
		parts = append(parts, fmt.Sprintf("%d Butterflies", butterflies))
	}
	if apples == 1 {
		parts = append(parts, "1 Apple")
	} else if apples > 1 {
		parts = append(parts, fmt.Sprintf("%d Apples", apples))
	}
	return strings.Join(parts, ", ")
}

func (a *App) openCheatMenu() {
	a.cheatReturnScreen = a.screen
	a.cheatReturnMenuIdx = a.menuIndex
	a.menuIndex = 0
	lvl := a.world.Level
	if lvl < 1 {
		lvl = 1
	}
	if lvl > levels.MaxLevel {
		lvl = levels.MaxLevel
	}
	a.cheatLevel = lvl
	a.cheatLevelTyping = false
	a.screen = ScreenCheat
	a.sfx.Play(audio.SfxExtraLife)
}

func (a *App) closeCheatMenu() {
	a.menuIndex = a.cheatReturnMenuIdx
	a.screen = a.cheatReturnScreen
	a.cheatLevelTyping = false
	a.cheatDetector.Reset()
}

func (a *App) jumpToCheatLevel() {
	a.world.Accessibility = a.cfg.Accessibility
	a.world.HighScore = a.cfg.HighScore
	a.world.JumpToLevel(a.cheatLevel)
	a.cheatLevelTyping = false
	a.cheatDetector.Reset()
	a.screen = ScreenPlaying
}

func (a *App) updateCheat() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.sfx.Play(audio.SfxMenuMove)
		a.closeCheatMenu()
		return nil
	}

	const totalItems = 3
	if a.menuUpPressed() {
		a.menuIndex = (a.menuIndex - 1 + totalItems) % totalItems
		a.cheatLevelTyping = false
		a.sfx.Play(audio.SfxMenuMove)
		return nil
	}
	if a.menuDownPressed() {
		a.menuIndex = (a.menuIndex + 1) % totalItems
		a.cheatLevelTyping = false
		a.sfx.Play(audio.SfxMenuMove)
		return nil
	}

	switch a.menuIndex {
	case 0: // INFINITE LIVES
		if a.menuLeftPressed() || a.menuRightPressed() || a.menuConfirmPressed() {
			a.world.InfiniteLives = !a.world.InfiniteLives
			a.sfx.Play(audio.SfxMenuSelect)
		}

	case 1: // LEVEL SELECT
		step := 1
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			step = 5
		}
		if a.menuLeftPressed() {
			a.cheatLevel = stepCheatLevel(a.cheatLevel, -step)
			a.cheatLevelTyping = false
			a.sfx.Play(audio.SfxMenuMove)
		} else if a.menuRightPressed() {
			a.cheatLevel = stepCheatLevel(a.cheatLevel, step)
			a.cheatLevelTyping = false
			a.sfx.Play(audio.SfxMenuMove)
		} else if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if a.cheatLevel >= 10 {
				a.cheatLevel /= 10
				a.cheatLevelTyping = true
			} else {
				a.cheatLevelTyping = false
			}
			a.sfx.Play(audio.SfxMenuMove)
		} else if a.menuConfirmPressed() {
			a.sfx.Play(audio.SfxMenuSelect)
			a.jumpToCheatLevel()
			return nil
		} else {
			for _, r := range ebiten.AppendInputChars(nil) {
				if r >= '0' && r <= '9' {
					next, typing := applyLevelDigit(a.cheatLevel, a.cheatLevelTyping, int(r-'0'))
					if next != a.cheatLevel || typing != a.cheatLevelTyping {
						a.cheatLevel = next
						a.cheatLevelTyping = typing
						a.sfx.Play(audio.SfxMenuMove)
					}
				}
			}
		}

	case 2: // BACK
		if a.menuConfirmPressed() {
			a.sfx.Play(audio.SfxMenuSelect)
			a.closeCheatMenu()
			return nil
		}
	}

	return nil
}

func (a *App) drawCheatOverlay(screen *ebiten.Image) {
	sw := float64(screen.Bounds().Dx())
	sh := float64(screen.Bounds().Dy())
	u := sw / 1120.0
	vector.DrawFilledRect(screen, 0, 0, float32(sw), float32(sh), uiDim, false)

	boxW, boxH := 820*u, 390*u
	boxX, boxY := (sw-boxW)/2, (sh-boxH)/2
	drawModalBox(screen, float32(boxX), float32(boxY), float32(boxW), float32(boxH), uiOrange)

	a.font.DrawCentered(screen, "CHEAT MENU", sw/2, boxY+24*u, 2.8*u, uiOrange)
	a.font.DrawCentered(screen, "IDDQD ACTIVATED   -   ESC: BACK", sw/2, boxY+62*u, 1.7*u, uiGray)

	y := boxY + 112*u

	// Row 0: INFINITE LIVES
	{
		clr := uiWhite
		prefix := "  "
		if a.menuIndex == 0 {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+"INFINITE LIVES", boxX+44*u, y, 2.2*u, clr)

		val := "[ OFF ]"
		valClr := uiGray
		if a.world.InfiniteLives {
			val = "[ ON ]"
			valClr = uiGreen
		}
		valW := float64(len(val)) * 8.0 * 2.2 * u
		a.font.DrawShadowed(screen, val, boxX+boxW-44*u-valW, y, 2.2*u, valClr)
		a.font.DrawShadowed(screen, "Never lose a life when caught or out of oxygen.", boxX+76*u, y+28*u, 1.4*u, uiBlue)
		y += 88 * u
	}

	// Row 1: LEVEL SELECT
	{
		clr := uiWhite
		prefix := "  "
		if a.menuIndex == 1 {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+"LEVEL SELECT", boxX+44*u, y, 2.2*u, clr)

		val := fmt.Sprintf("< %02d / %d >", a.cheatLevel, levels.MaxLevel)
		valClr := uiGreen
		if a.menuIndex == 1 {
			valClr = uiOrange
		}
		valW := float64(len(val)) * 8.0 * 2.2 * u
		a.font.DrawShadowed(screen, val, boxX+boxW-44*u-valW, y, 2.2*u, valClr)

		summary := fmt.Sprintf("ENTER: Jump to Level %d (%s)", a.cheatLevel, formatLevelSummary(a.cheatLevel))
		a.font.DrawShadowed(screen, summary, boxX+76*u, y+28*u, 1.4*u, uiBlue)
		a.font.DrawShadowed(screen, "LEFT/RIGHT (SHIFT: +/-5) or type digits 1-48.", boxX+76*u, y+48*u, 1.3*u, uiGray)
		y += 96 * u
	}

	// Row 2: BACK
	{
		clr := uiWhite
		prefix := "  "
		if a.menuIndex == 2 {
			clr = uiOrange
			prefix = "> "
		}
		a.font.DrawShadowed(screen, prefix+"BACK", boxX+44*u, y+12*u, 2.2*u, clr)
	}
}
