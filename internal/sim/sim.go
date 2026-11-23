package sim

import (
	"math/rand"

	"apple-panic/internal/audio"
	"apple-panic/internal/config"
	"apple-panic/internal/levels"
)

const (
	// BaseOuterLoopsPerSec matches our cycle-accurate 6502 measurement (~7.5 Hz).
	// Each outer loop performs 2 player updates (15 Hz) and 1 monster pass.
	BaseOuterLoopsPerSec = 7.5

	PlayerStartX = 0x44 // 68 (center of 0..132 playfield)
	PlayerStartY = 0xA0 // 160 (standing on ground floor at Y=176)
	PlayerMinX   = 0x01
	PlayerMaxX   = 0x82 // 130 (rightmost player logical X)

	MonsterMinX = 0x00
	MonsterMaxX = 0x84 // 132

	MaxOxygen          = 130 // Initial oxygen bar units
	OxygenLoopsPerTick = 32  // Outer loops between oxygen bar decrements
	BonusLoopsPerTick  = 33  // Outer loops between bonus countdown decrements
	BonusStep          = 20

	TrappedWiggleTicks = 128 // Monster passes before a trapped monster climbs out
	ExtraLifeScore     = 4000
	InitialLives       = 3

	// Death / monster-catch animation timings
	DeathDuration      = 1.15 // Total PhaseDying duration in seconds
	CatchJumpDuration  = 0.28 // Duration of the monster's jump arc onto the player's head
	CatchJumpArcHeight = 8.0  // Peak vertical arc height (in scanlines) during the jump
)

// BrickFloorYs lists the 5 diggable brick girders from top to bottom.
var BrickFloorYs = [5]int{0x10, 0x30, 0x50, 0x70, 0x90}

// GroundFloorY is the solid indestructible floor at the bottom of the playfield.
const GroundFloorY = 0xB0

// Dir represents movement direction for monsters and player.
type Dir int

const (
	DirNone  Dir = 0
	DirRight Dir = 1
	DirLeft  Dir = 2
	DirUp    Dir = 3
	DirDown  Dir = 4
)

// PlayerState describes what the player character is currently doing.
type PlayerState int

const (
	PlayerIdle PlayerState = iota
	PlayerRunning
	PlayerClimbing
	PlayerDigging
	PlayerFilling
	PlayerFalling
)

// LatchedAction represents an active toggle-latched dig or fill action.
type LatchedAction int

const (
	LatchNone LatchedAction = iota
	LatchDig
	LatchFill
)

// Player holds the player's state in logical coordinates.
type Player struct {
	X           int
	Y           int
	FacingRight bool
	State       PlayerState
	AnimStep    int
	DigCounter  int  // 0..10 sub-ticks per dig/fill scoop
	ShovelDown  bool // Toggles between shovel-up and shovel-down animation frames

	// Toggle-based control state
	LatchedDir        Dir
	QueuedDir         Dir
	MovedInLatchedDir bool
	LatchedAct        LatchedAction
}

// Hole represents a partially or fully dug pit in one of the 5 brick girders.
type Hole struct {
	FloorIdx int // 0..4 corresponding to BrickFloorYs
	X        int // Even logical X in [2..130]
	Stage    int // 1..5 (5 = fully open hole)
	Occupied bool
}

// YHole returns the scanline immediately above the hole's brick girder (floorY - 1: 15, 47, 79, 111, or 143).
func (h Hole) YHole() int {
	return BrickFloorYs[h.FloorIdx] - 1
}

// MonsterState tracks a monster's behavior phase.
type MonsterState int

const (
	MonsterPatrol MonsterState = iota
	MonsterTrapped
	MonsterEscaping
	MonsterFalling
)

// FallPassenger tracks an extra trapped monster knocked out during a chain fall.
type FallPassenger struct {
	BasePoints  int
	HolesFallen int
}

// Monster holds one enemy slot's state.
type Monster struct {
	Slot              int
	Active            bool
	KilledInThisLevel bool
	Type              levels.MonsterType
	X                 int
	Y                 int
	Dir               Dir
	State             MonsterState
	AnimFrame         int
	WiggleTicks       int
	TrappedFloor      int
	TrappedHoleX      int

	// Falling tracking
	HolesFallen int
	ChainKill   bool
	Passengers  []FallPassenger
}

// ScorePopup displays points earned on screen at the kill site.
type ScorePopup struct {
	X       int
	Y       int
	Points  int
	Timer   float64
}

// InputSnapshot captures player inputs for a frame or simulation tick.
type InputSnapshot struct {
	// Held states (used when ToggleControls is false)
	Up    bool
	Down  bool
	Left  bool
	Right bool
	Dig   bool
	Fill  bool

	// Edge-triggered key taps (used when ToggleControls is true)
	PressUp    bool
	PressDown  bool
	PressLeft  bool
	PressRight bool
	PressDig   bool
	PressFill  bool
	PressStop  bool
}

// Phase describes high-level stage flow within a playthrough.
type Phase int

const (
	PhaseReady       Phase = iota // Brief level-start fanfare
	PhasePlaying                  // Active gameplay
	PhaseSirenFreeze              // Brief pause when Butterfly/Mask survives a fall
	PhaseDying                    // Death animation/pause before retry or game over
	PhaseLevelClear               // Brief pause adding remaining bonus before next level
	PhaseGameOver                 // Out of lives
	PhaseVictory                  // Cleared all 48 levels!
)

// World encapsulates the complete deterministic game state.
type World struct {
	Level            int
	Lives            int
	InfiniteLives    bool
	Score            int
	HighScore        int
	ExtraLifeAwarded bool

	Oxygen          int
	OxygenLoopCount int
	Bonus           int
	BonusLoopCount  int

	LayoutCounter int
	Ladders       []levels.Ladder
	Holes         []Hole
	Player        Player
	Monsters      [8]Monster
	Popups        []ScorePopup

	Phase      Phase
	PhaseTimer float64

	// Monster-catch death animation state (active during PhaseDying)
	CatchingMonsterSlot int     // -1 for non-monster death (e.g. oxygen), else 0..7
	CatchAnimElapsed    float64 // Seconds elapsed since PhaseDying started
	CatchStartX         float64 // Catching monster's logical X at capture
	CatchStartY         float64 // Catching monster's logical Y at capture

	Accessibility config.Accessibility
	SfxQueue      []audio.Effect

	rng               *rand.Rand
	accumPlayerSteps  float64
	accumMonsterPass  float64
	playerHalfStep    int
	footstepToggle    bool
}

// NewWorld creates a fresh World ready to start a game.
func NewWorld(seed int64, highScore int, acc config.Accessibility) *World {
	w := &World{
		HighScore:     highScore,
		Accessibility: acc,
		rng:           rand.New(rand.NewSource(seed)),
	}
	w.StartNewGame()
	return w
}

func (w *World) emit(sfx audio.Effect) {
	w.SfxQueue = append(w.SfxQueue, sfx)
}

// DrainSfx returns and clears queued sound effects since the last frame.
func (w *World) DrainSfx() []audio.Effect {
	if len(w.SfxQueue) == 0 {
		return nil
	}
	out := make([]audio.Effect, len(w.SfxQueue))
	copy(out, w.SfxQueue)
	w.SfxQueue = w.SfxQueue[:0]
	return out
}

// StartNewGame resets score, lives, and begins Level 1.
func (w *World) StartNewGame() {
	w.Score = 0
	w.Lives = InitialLives
	w.Level = 1
	w.ExtraLifeAwarded = false
	w.clearLevelKillMemory()
	w.setupLevel(false)
}

func (w *World) clearLevelKillMemory() {
	for i := range w.Monsters {
		w.Monsters[i].KilledInThisLevel = false
	}
}

// RestartCurrentLevel resets the current level completely (including kill memory).
func (w *World) RestartCurrentLevel() {
	w.clearLevelKillMemory()
	w.setupLevel(false)
}

// JumpToLevel switches immediately to the specified level (clamped to 1..MaxLevel)
// and initializes it cleanly from the start.
func (w *World) JumpToLevel(level int) {
	if level < 1 {
		level = 1
	}
	if level > levels.MaxLevel {
		level = levels.MaxLevel
	}
	if w.Lives <= 0 {
		w.Lives = InitialLives
	}
	w.Level = level
	w.clearLevelKillMemory()
	w.setupLevel(false)
}

// setupLevel initializes ladders, holes, timers, player, and monsters for the current level.
func (w *World) setupLevel(isDeathRetry bool) {
	w.Player = Player{
		X:           PlayerStartX,
		Y:           PlayerStartY,
		FacingRight: true,
		State:       PlayerIdle,
	}
	w.Bonus = levels.StartingBonus(w.Level)
	w.BonusLoopCount = 0
	w.Oxygen = MaxOxygen
	w.OxygenLoopCount = 0
	w.Holes = w.Holes[:0]
	w.Popups = w.Popups[:0]
	w.accumPlayerSteps = 0
	w.accumMonsterPass = 0
	w.playerHalfStep = 0

	if !isDeathRetry {
		// Rotate through the 4 ladder layouts based on lives and layout counter.
		w.LayoutCounter++
		ladderIdx := (w.Lives + w.LayoutCounter) & 3
		srcLadders := levels.LadderLayouts[ladderIdx]
		w.Ladders = make([]levels.Ladder, len(srcLadders))
		copy(w.Ladders, srcLadders)
	}

	// Rotate through the 4 monster spawn sets based on level and layout counter.
	w.LayoutCounter++
	spawnIdx := (w.Level + w.LayoutCounter) & 3
	spawns := levels.SpawnLayouts[spawnIdx]

	roster := levels.MonstersForLevel(w.Level)
	for i := 0; i < 8; i++ {
		keepDead := isDeathRetry && w.Accessibility.KeepKilledMonstersDead && w.Monsters[i].KilledInThisLevel
		if i < len(roster) && !keepDead {
			w.Monsters[i] = Monster{
				Slot:              i,
				Active:            true,
				KilledInThisLevel: false,
				Type:              roster[i],
				X:                 spawns[i].X,
				Y:                 spawns[i].Y,
				Dir:               DirNone,
				State:             MonsterPatrol,
				AnimFrame:         i % 2,
			}
		} else {
			killed := false
			if i < len(roster) && keepDead {
				killed = true
			}
			w.Monsters[i] = Monster{
				Slot:              i,
				Active:            false,
				KilledInThisLevel: killed,
			}
		}
	}

	w.Phase = PhaseReady
	w.PhaseTimer = 0.65
	w.CatchingMonsterSlot = -1
	w.CatchAnimElapsed = 0
	w.emit(audio.SfxLevelStart)
}

// Update advances the world by dt seconds (typically 1/60s from Ebitengine).
func (w *World) Update(dt float64, in InputSnapshot) {
	if dt <= 0 {
		return
	}
	if dt > 0.1 {
		dt = 0.1
	}

	// Update score popups
	for i := len(w.Popups) - 1; i >= 0; i-- {
		w.Popups[i].Timer -= dt
		if w.Popups[i].Timer <= 0 {
			w.Popups = append(w.Popups[:i], w.Popups[i+1:]...)
		}
	}

	switch w.Phase {
	case PhaseReady:
		w.PhaseTimer -= dt
		if w.PhaseTimer <= 0 {
			w.Phase = PhasePlaying
			// Edge case: if all monsters were already dead, clear immediately.
			w.checkLevelClear()
		}
		return

	case PhaseSirenFreeze:
		w.PhaseTimer -= dt
		if w.PhaseTimer <= 0 {
			w.Phase = PhasePlaying
		}
		return

	case PhaseDying:
		w.PhaseTimer -= dt
		w.CatchAnimElapsed += dt
		if w.CatchingMonsterSlot >= 0 && w.CatchingMonsterSlot < len(w.Monsters) {
			w.Monsters[w.CatchingMonsterSlot].AnimFrame = int(w.CatchAnimElapsed*10.0) & 1
		}
		if w.PhaseTimer <= 0 {
			if w.InfiniteLives {
				if w.Lives < 1 {
					w.Lives = 1
				}
				w.setupLevel(true)
			} else {
				w.Lives--
				if w.Lives <= 0 {
					w.Lives = 0
					w.Phase = PhaseGameOver
				} else {
					w.setupLevel(true)
				}
			}
		}
		return

	case PhaseLevelClear:
		w.PhaseTimer -= dt
		if w.PhaseTimer <= 0 {
			w.Level++
			if w.Level > levels.MaxLevel {
				w.Level = levels.MaxLevel
				w.Phase = PhaseVictory
			} else {
				w.clearLevelKillMemory()
				w.setupLevel(false)
			}
		}
		return

	case PhaseGameOver, PhaseVictory:
		return
	}

	speedMul := float64(w.Accessibility.GameSpeedPercent) / 100.0
	if speedMul <= 0 {
		speedMul = 1.0
	}
	outerRate := BaseOuterLoopsPerSec * speedMul
	playerStepRate := outerRate * 2.0

	// Progressive monster speed-up as oxygen enters the low-oxygen danger zone:
	// Oxygen >= 30: 1.0x; 20..29: 9/8x; 10..19: 10/8x; 1..9: 11/8x.
	monsterSpeedRatio := 1.0
	if w.Oxygen < 10 {
		monsterSpeedRatio = 11.0 / 8.0
	} else if w.Oxygen < 20 {
		monsterSpeedRatio = 10.0 / 8.0
	} else if w.Oxygen < 30 {
		monsterSpeedRatio = 9.0 / 8.0
	}

	if w.isFallPaused() {
		// Authentic Apple II behavior: while a monster is falling through holes,
		// player movement, other monsters, and outer-loop timers pause until the
		// falling monster finishes its drop.
		w.accumMonsterPass += dt * outerRate * monsterSpeedRatio
		for w.accumMonsterPass >= 1.0 && w.Phase == PhasePlaying {
			w.accumMonsterPass -= 1.0
			w.StepMonsters()
			if !w.isFallPaused() {
				break
			}
		}
		return
	}

	if w.Accessibility.ToggleControls && w.Phase == PhasePlaying {
		w.consumeTogglePresses(&in)
	}

	w.accumPlayerSteps += dt * playerStepRate
	w.accumMonsterPass += dt * outerRate * monsterSpeedRatio

	for w.accumPlayerSteps >= 1.0 && w.Phase == PhasePlaying {
		if w.isFallPaused() {
			break
		}
		w.accumPlayerSteps -= 1.0
		w.StepPlayer(in)
		w.playerHalfStep++
		if w.playerHalfStep >= 2 {
			w.playerHalfStep = 0
			w.stepOuterLoopTimers()
		}
	}

	for w.accumMonsterPass >= 1.0 && w.Phase == PhasePlaying {
		w.accumMonsterPass -= 1.0
		w.StepMonsters()
	}
}

// HasFallingMonster reports whether any active monster is currently falling through holes.
func (w *World) HasFallingMonster() bool {
	for i := range w.Monsters {
		if w.Monsters[i].Active && w.Monsters[i].State == MonsterFalling {
			return true
		}
	}
	return false
}

func (w *World) isFallPaused() bool {
	return w.Accessibility.PauseWhileMonsterFalls && w.HasFallingMonster()
}

func (w *World) stepOuterLoopTimers() {
	if w.Phase != PhasePlaying {
		return
	}
	// Bonus countdown: -20 every 33 outer loops
	w.BonusLoopCount++
	if w.BonusLoopCount >= BonusLoopsPerTick {
		w.BonusLoopCount = 0
		w.Bonus -= BonusStep
		if w.Bonus < 0 {
			w.Bonus = 0
		}
	}

	// Oxygen bar countdown: -1 every 32 outer loops
	w.OxygenLoopCount++
	if w.OxygenLoopCount >= OxygenLoopsPerTick {
		w.OxygenLoopCount = 0
		w.Oxygen--
		if w.Oxygen <= 0 {
			w.Oxygen = 0
			w.KillPlayer()
			return
		}
	}

	w.checkLevelClear()
}

func (w *World) checkLevelClear() {
	if w.Phase != PhasePlaying {
		return
	}
	for i := range w.Monsters {
		if w.Monsters[i].Active {
			return
		}
	}
	// All monsters defeated: award remaining bonus and advance to the next level.
	if w.Bonus > 0 {
		w.AddScore(w.Bonus)
		w.Bonus = 0
	}
	w.Phase = PhaseLevelClear
	w.PhaseTimer = 0.90
	w.emit(audio.SfxExtraLife)
}

// AddScore increments Score, updates HighScore, and awards the one-time extra life at 4,000 points.
func (w *World) AddScore(pts int) {
	if pts <= 0 {
		return
	}
	w.Score += pts
	if w.Score > w.HighScore {
		w.HighScore = w.Score
	}
	if !w.ExtraLifeAwarded && w.Score >= ExtraLifeScore {
		w.ExtraLifeAwarded = true
		w.Lives++
		w.emit(audio.SfxExtraLife)
	}
}

// KillPlayer initiates player death (e.g. oxygen depletion).
func (w *World) KillPlayer() {
	w.killPlayerWithMonster(-1)
}

// KillPlayerByMonster initiates player death caused by the monster in slot catching the player.
func (w *World) KillPlayerByMonster(slot int) {
	w.killPlayerWithMonster(slot)
}

func (w *World) killPlayerWithMonster(slot int) {
	if w.Phase != PhasePlaying {
		return
	}
	w.Phase = PhaseDying
	w.PhaseTimer = DeathDuration
	w.CatchAnimElapsed = 0
	if slot >= 0 && slot < len(w.Monsters) && w.Monsters[slot].Active {
		w.CatchingMonsterSlot = slot
		w.CatchStartX = float64(w.Monsters[slot].X)
		w.CatchStartY = float64(w.Monsters[slot].Y)
	} else {
		w.CatchingMonsterSlot = -1
	}
	w.emit(audio.SfxPlayerDeath)
}

// CatchingMonsterPos returns the catching monster's interpolated logical (X, Y)
// during the PhaseDying jump-onto-head animation, plus whether it has landed on the player's head.
func (w *World) CatchingMonsterPos() (x, y float64, landed bool, ok bool) {
	if w.Phase != PhaseDying || w.CatchingMonsterSlot < 0 || w.CatchingMonsterSlot >= len(w.Monsters) {
		return 0, 0, false, false
	}
	targetX := float64(w.Player.X)
	targetY := float64(w.Player.Y)
	t := w.CatchAnimElapsed
	if t >= CatchJumpDuration {
		return targetX, targetY, true, true
	}
	u := t / CatchJumpDuration
	if u < 0 {
		u = 0
	}
	x = w.CatchStartX + (targetX-w.CatchStartX)*u
	baseY := w.CatchStartY + (targetY-w.CatchStartY)*u
	arc := CatchJumpArcHeight * 4.0 * u * (1.0 - u)
	y = baseY - arc
	if y < 0 {
		y = 0
	}
	return x, y, false, true
}

// LadderMask returns bit 0 (can climb down) and bit 1 (can climb up) for a vertical
// position yHole (e.g. player.Y+15 or monster.Y+9) at exact logical X.
func (w *World) LadderMask(x, yHole int) int {
	mask := 0
	for _, lad := range w.Ladders {
		if lad.X != x {
			continue
		}
		if yHole >= lad.YTop && yHole < lad.YBot {
			mask |= 1 // Can go down
		}
		if yHole > lad.YTop && yHole <= lad.YBot {
			mask |= 2 // Can go up
		}
	}
	return mask
}

func floorIndexForPlayerY(py int) int {
	if py&0x1F != 0 || py < 0 || py >= 0xA0 {
		return -1
	}
	return py >> 5 // 0..4 for brick floors at Y = 0, 32, 64, 96, 128
}

func floorIndexForMonsterY(my int) int {
	rel := my - 6
	if rel < 0 || rel&0x1F != 0 || my >= 0xA6 {
		return -1
	}
	return rel >> 5 // 0..4 for brick floors at monster Y = 6, 38, 70, 102, 134
}

// FindHoleAt returns a pointer to the hole at exact (floorIdx, holeX), or nil.
func (w *World) FindHoleAt(floorIdx, holeX int) *Hole {
	if floorIdx < 0 || floorIdx >= 5 {
		return nil
	}
	for i := range w.Holes {
		if w.Holes[i].FloorIdx == floorIdx && w.Holes[i].X == holeX {
			return &w.Holes[i]
		}
	}
	return nil
}

func (w *World) removeHoleAt(floorIdx, holeX int) {
	for i := range w.Holes {
		if w.Holes[i].FloorIdx == floorIdx && w.Holes[i].X == holeX {
			w.Holes = append(w.Holes[:i], w.Holes[i+1:]...)
			return
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func oppositeDir(d Dir) Dir {
	switch d {
	case DirLeft:
		return DirRight
	case DirRight:
		return DirLeft
	case DirUp:
		return DirDown
	case DirDown:
		return DirUp
	default:
		return DirNone
	}
}

func dxForDir(d Dir) int {
	if d == DirRight {
		return 1
	}
	if d == DirLeft {
		return -1
	}
	return 0
}

func (w *World) consumeTogglePresses(in *InputSnapshot) {
	if in.PressStop {
		w.Player.LatchedDir = DirNone
		w.Player.QueuedDir = DirNone
		w.Player.MovedInLatchedDir = false
		w.Player.LatchedAct = LatchNone
		in.PressStop = false
	}
	if in.PressDig {
		w.Player.LatchedDir = DirNone
		w.Player.QueuedDir = DirNone
		w.Player.MovedInLatchedDir = false
		if (w.Player.Y & 0x1F) == 0 {
			w.Player.LatchedAct = LatchDig
		}
		in.PressDig = false
	}
	if in.PressFill {
		w.Player.LatchedDir = DirNone
		w.Player.QueuedDir = DirNone
		w.Player.MovedInLatchedDir = false
		if (w.Player.Y & 0x1F) == 0 {
			w.Player.LatchedAct = LatchFill
		}
		in.PressFill = false
	}
	if in.PressUp {
		w.onToggleDirPressed(DirUp)
		in.PressUp = false
	}
	if in.PressDown {
		w.onToggleDirPressed(DirDown)
		in.PressDown = false
	}
	if in.PressLeft {
		w.onToggleDirPressed(DirLeft)
		in.PressLeft = false
	}
	if in.PressRight {
		w.onToggleDirPressed(DirRight)
		in.PressRight = false
	}
}

func (w *World) onToggleDirPressed(d Dir) {
	p := &w.Player
	p.LatchedAct = LatchNone

	if p.State == PlayerFalling {
		if d == DirLeft || d == DirRight {
			p.LatchedDir = d
			p.QueuedDir = DirNone
			p.MovedInLatchedDir = true
			p.FacingRight = (d == DirRight)
		} else {
			p.QueuedDir = d
		}
		return
	}

	if p.LatchedDir == DirNone {
		if d == DirLeft || d == DirRight {
			if (p.Y & 0x1F) == 0 {
				p.FacingRight = (d == DirRight)
				if w.canPlayerWalk(dxForDir(d)) {
					p.LatchedDir = d
					p.QueuedDir = DirNone
					p.MovedInLatchedDir = false
				}
			}
		} else if d == DirUp || d == DirDown {
			if w.canPlayerClimb(d == DirUp, true) {
				p.LatchedDir = d
				p.QueuedDir = DirNone
				p.MovedInLatchedDir = false
			}
		}
		return
	}

	if d == p.LatchedDir {
		p.QueuedDir = DirNone
		return
	}

	if d == oppositeDir(p.LatchedDir) {
		p.LatchedDir = d
		p.QueuedDir = DirNone
		p.MovedInLatchedDir = false
		if d == DirLeft || d == DirRight {
			p.FacingRight = (d == DirRight)
		}
		return
	}

	// Perpendicular direction while moving: queue a single direction change!
	p.QueuedDir = d
}

// StepPlayer executes one player update tick.
func (w *World) StepPlayer(in InputSnapshot) {
	if w.isFallPaused() {
		return
	}
	p := &w.Player

	if w.Accessibility.ToggleControls {
		w.consumeTogglePresses(&in)
	} else {
		p.LatchedDir = DirNone
		p.QueuedDir = DirNone
		p.MovedInLatchedDir = false
		p.LatchedAct = LatchNone
	}

	// 1. Handle falling through an open hole.
	if p.State == PlayerFalling {
		if p.Y&0x1F != 0 {
			p.Y += 2
			p.AnimStep++
			if p.Y&0x1F == 0 {
				// Reached a floor: check if there is another aligned open hole directly below!
				fIdx := floorIndexForPlayerY(p.Y)
				if fIdx >= 0 {
					if h := w.FindHoleAt(fIdx, p.X); h != nil && h.Stage == 5 && !h.Occupied {
						// Keep falling straight through the aligned hole!
						return
					}
				}
				// Landed on solid floor: nudge clear of any nearby partial hole within <6 units.
				p.State = PlayerIdle
				if fIdx >= 0 {
					for _, h := range w.Holes {
						if h.FloorIdx == fIdx && h.Stage > 0 && h.Stage < 5 && abs(p.X-h.X) < 6 {
							if h.X < p.X {
								p.X = clamp(h.X+7, PlayerMinX, PlayerMaxX)
							} else {
								p.X = clamp(h.X-7, PlayerMinX, PlayerMaxX)
							}
							break
						}
					}
				}
			}
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
		p.Y += 2
		p.AnimStep++
		w.checkAllMonsterCollisionsWithPlayer()
		return
	}

	if w.Accessibility.ToggleControls {
		w.stepPlayerToggle()
		return
	}

	onFloor := (p.Y & 0x1F) == 0

	// 2. Digging a hole ahead of the player
	if in.Dig && !in.Fill && onFloor {
		if w.tryPlayerDig() {
			return
		}
	} else if in.Fill && !in.Dig && onFloor {
		// 3. Filling a hole ahead of the player
		if w.tryPlayerFill() {
			return
		}
	}

	// Reset scoop sub-counter whenever not actively digging/filling.
	p.DigCounter = 0
	p.ShovelDown = false

	// 4. Vertical ladder climbing (Up / Down)
	if in.Up && !in.Down {
		if w.tryPlayerClimb(true, true) {
			w.playFootstep()
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
	} else if in.Down && !in.Up {
		if w.tryPlayerClimb(false, true) {
			w.playFootstep()
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
	}

	// 5. Horizontal movement (Right / Left) - only when aligned with a floor
	if onFloor {
		if in.Right && !in.Left {
			p.FacingRight = true
			w.tryPlayerWalk(1)
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
		if in.Left && !in.Right {
			p.FacingRight = false
			w.tryPlayerWalk(-1)
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
	}

	if onFloor {
		p.State = PlayerIdle
	}
}

func (w *World) stepPlayerToggle() {
	p := &w.Player
	onFloor := (p.Y & 0x1F) == 0

	// 1. Latched Dig or Fill runs until hole completes (or is obstructed/cancelled).
	if p.LatchedAct != LatchNone {
		if onFloor {
			if p.LatchedAct == LatchDig && w.tryPlayerDig() {
				return
			}
			if p.LatchedAct == LatchFill && w.tryPlayerFill() {
				return
			}
		}
		p.LatchedAct = LatchNone
	}

	p.DigCounter = 0
	p.ShovelDown = false

	// 2. Promote QueuedDir at the next available junction!
	if p.QueuedDir != DirNone && (p.LatchedDir == DirNone || p.MovedInLatchedDir) {
		switch p.QueuedDir {
		case DirLeft, DirRight:
			if onFloor && w.canPlayerWalk(dxForDir(p.QueuedDir)) {
				p.LatchedDir = p.QueuedDir
				p.QueuedDir = DirNone
				p.MovedInLatchedDir = false
			}
		case DirUp, DirDown:
			allowSnap := (p.LatchedDir == DirNone)
			if onFloor && w.canPlayerClimb(p.QueuedDir == DirUp, allowSnap) {
				p.LatchedDir = p.QueuedDir
				p.QueuedDir = DirNone
				p.MovedInLatchedDir = false
			}
		}
	}

	// 3. Advance along LatchedDir until hitting an obstacle!
	switch p.LatchedDir {
	case DirUp, DirDown:
		allowSnap := !p.MovedInLatchedDir
		if w.tryPlayerClimb(p.LatchedDir == DirUp, allowSnap) {
			p.MovedInLatchedDir = true
			w.playFootstep()
			w.checkAllMonsterCollisionsWithPlayer()
			return
		}
		// Hit top/bottom end of ladder
		p.LatchedDir = DirNone
		p.QueuedDir = DirNone
		p.MovedInLatchedDir = false

	case DirLeft, DirRight:
		if onFloor {
			dx := dxForDir(p.LatchedDir)
			p.FacingRight = (dx > 0)
			if w.tryPlayerWalk(dx) {
				if p.State == PlayerFalling {
					p.QueuedDir = DirNone
				}
				p.MovedInLatchedDir = true
				w.checkAllMonsterCollisionsWithPlayer()
				return
			}
		}
		// Hit wall or blocking hole
		p.LatchedDir = DirNone
		p.QueuedDir = DirNone
		p.MovedInLatchedDir = false
	}

	if (p.Y & 0x1F) == 0 {
		p.State = PlayerIdle
	}
}

func (w *World) playFootstep() {
	w.footstepToggle = !w.footstepToggle
	if w.footstepToggle {
		w.emit(audio.SfxFootstepA)
	} else {
		w.emit(audio.SfxFootstepB)
	}
}

func (w *World) canPlayerClimb(up bool, allowSnap bool) bool {
	p := &w.Player
	needBit := 1
	if up {
		needBit = 2
	}
	yHole := p.Y + 15
	if w.LadderMask(p.X, yHole)&needBit != 0 {
		return true
	}
	if !allowSnap || p.Y&0x1F != 0 {
		return false
	}
	for _, delta := range []int{-1, 1, -2, 2} {
		if w.LadderMask(p.X+delta, yHole)&needBit != 0 {
			return true
		}
	}
	return false
}

func (w *World) tryPlayerClimb(up bool, allowSnap bool) bool {
	p := &w.Player
	needBit := 1 // Down
	dy := 2
	if up {
		needBit = 2 // Up
		dy = -2
	}

	yHole := p.Y + 15
	// Check exact X first, or within ±2 logical units when standing on a floor if allowSnap is true.
	if w.LadderMask(p.X, yHole)&needBit == 0 {
		if !allowSnap || p.Y&0x1F != 0 {
			return false
		}
		snapped := false
		for _, delta := range []int{-1, 1, -2, 2} {
			candX := p.X + delta
			if w.LadderMask(candX, yHole)&needBit != 0 {
				p.X = candX
				snapped = true
				break
			}
		}
		if !snapped {
			return false
		}
	}

	p.Y += dy
	p.State = PlayerClimbing
	p.AnimStep++
	return true
}

func (w *World) canPlayerWalk(dx int) bool {
	p := &w.Player
	if p.Y&0x1F != 0 || dx == 0 {
		return false
	}
	fIdx := floorIndexForPlayerY(p.Y)
	if fIdx >= 0 {
		if h := w.FindHoleAt(fIdx, p.X); h != nil {
			return h.Stage == 5 && !h.Occupied
		}
		if p.X&1 == 1 {
			aheadX := p.X + dx*7
			if h := w.FindHoleAt(fIdx, aheadX); h != nil {
				if h.Stage != 5 || h.Occupied {
					return false
				}
			}
		}
	}
	nx := p.X + dx
	return nx >= PlayerMinX && nx <= PlayerMaxX
}

func (w *World) tryPlayerWalk(dx int) bool {
	p := &w.Player
	fIdx := floorIndexForPlayerY(p.Y)

	// Check holes underfoot and ahead on diggable brick floors:
	if fIdx >= 0 {
		// If currently standing on the exact center of an empty open hole, fall in!
		if h := w.FindHoleAt(fIdx, p.X); h != nil {
			if h.Stage == 5 && !h.Occupied {
				p.State = PlayerFalling
				p.Y += 2
				return true
			}
			// Partial or occupied hole blocks movement.
			p.State = PlayerIdle
			return false
		}
		// When on odd X, check the digging distance ahead (p.X + dx*7):
		// partial holes (1..4) and occupied holes block the player right at
		// digging/filling distance so the player stops ready to Fill without stepping onto a monster!
		if p.X&1 == 1 {
			aheadX := p.X + dx*7
			if h := w.FindHoleAt(fIdx, aheadX); h != nil {
				if h.Stage != 5 || h.Occupied {
					p.State = PlayerIdle
					return false
				}
			}
		}
	}

	nx := p.X + dx
	if nx < PlayerMinX || nx > PlayerMaxX {
		p.State = PlayerIdle
		return false
	}
	p.X = nx
	p.State = PlayerRunning
	p.AnimStep++
	w.playFootstep()

	// Check if stepping onto an empty open hole triggers an immediate fall.
	if fIdx >= 0 {
		if h := w.FindHoleAt(fIdx, p.X); h != nil && h.Stage == 5 && !h.Occupied {
			p.State = PlayerFalling
		}
	}
	return true
}

func (w *World) ensureOddPlayerXForAction(forFill bool) bool {
	p := &w.Player
	if p.X&1 == 1 {
		return true
	}
	if !w.Accessibility.AutoAlignDigging {
		return false
	}
	fIdx := floorIndexForPlayerY(p.Y)
	if fIdx < 0 {
		return false
	}
	dir := -1
	if p.FacingRight {
		dir = 1
	}
	// If filling, prefer whichever ±1 odd X aligns with an existing hole!
	if forFill {
		for _, cand := range []int{p.X + dir, p.X - dir} {
			if cand >= PlayerMinX && cand <= PlayerMaxX && cand&1 == 1 {
				hx := cand + dir*7
				if w.FindHoleAt(fIdx, hx) != nil {
					p.X = cand
					return true
				}
			}
		}
	}
	for _, cand := range []int{p.X + dir, p.X - dir} {
		if cand >= PlayerMinX && cand <= PlayerMaxX && cand&1 == 1 {
			p.X = cand
			return true
		}
	}
	return false
}

func (w *World) tryPlayerDig() bool {
	p := &w.Player
	fIdx := floorIndexForPlayerY(p.Y)
	if fIdx < 0 {
		return false // Cannot dig on the solid ground floor (Y=160)
	}
	if !w.ensureOddPlayerXForAction(false) {
		return false
	}
	holeX := p.X - 7
	if p.FacingRight {
		holeX = p.X + 7
	}
	if holeX < 2 || holeX > 130 {
		return false
	}

	existing := w.FindHoleAt(fIdx, holeX)
	if existing == nil && w.Accessibility.AutoAlignDigging {
		dir := -1
		if p.FacingRight {
			dir = 1
		}
		// Snap onto a nearby unfinished hole (within ±3 units) if continuing a dig!
		for i := range w.Holes {
			h := &w.Holes[i]
			if h.FloorIdx == fIdx && h.Stage > 0 && h.Stage < 5 && !h.Occupied && abs(h.X-holeX) <= 3 {
				candX := h.X - dir*7
				if candX >= PlayerMinX && candX <= PlayerMaxX {
					p.X = candX
					holeX = h.X
					existing = h
					break
				}
			}
		}
	}
	if existing != nil {
		if (existing.Stage >= 5 && p.DigCounter == 0) || existing.Occupied {
			return false
		}
	} else {
		// Max 15 holes per floor
		countOnFloor := 0
		for _, h := range w.Holes {
			if h.FloorIdx == fIdx {
				countOnFloor++
				// Require at least 9 logical X units of spacing between adjacent holes
				if abs(holeX-h.X) < 9 {
					return false
				}
			}
		}
		if countOnFloor >= 15 {
			return false
		}
		// Require at least 9 logical X units of clearance from any ladder touching this floor
		yHole := BrickFloorYs[fIdx] - 1
		for _, lad := range w.Ladders {
			if yHole >= lad.YTop && yHole <= lad.YBot && abs(holeX-lad.X) < 9 {
				return false
			}
		}
	}

	p.State = PlayerDigging
	p.DigCounter++
	switch p.DigCounter {
	case 2:
		if existing != nil {
			existing.Stage++
		} else {
			w.Holes = append(w.Holes, Hole{
				FloorIdx: fIdx,
				X:        holeX,
				Stage:    1,
			})
		}
	case 4:
		w.emit(audio.SfxDigScrape)
	case 8:
		p.ShovelDown = true
	case 10:
		p.DigCounter = 0
		p.ShovelDown = false
	}
	return true
}

func (w *World) tryPlayerFill() bool {
	p := &w.Player
	fIdx := floorIndexForPlayerY(p.Y)
	if fIdx < 0 {
		return false
	}
	if !w.ensureOddPlayerXForAction(true) {
		return false
	}
	holeX := p.X - 7
	if p.FacingRight {
		holeX = p.X + 7
	}
	existing := w.FindHoleAt(fIdx, holeX)
	if existing == nil && w.Accessibility.AutoAlignDigging {
		dir := -1
		if p.FacingRight {
			dir = 1
		}
		for i := range w.Holes {
			h := &w.Holes[i]
			if h.FloorIdx == fIdx && h.Stage > 0 && abs(h.X-holeX) <= 3 {
				candX := h.X - dir*7
				if candX >= PlayerMinX && candX <= PlayerMaxX {
					p.X = candX
					holeX = h.X
					existing = h
					break
				}
			}
		}
	}
	if existing == nil || existing.Stage <= 0 {
		return false
	}

	p.State = PlayerFilling
	p.DigCounter++
	switch p.DigCounter {
	case 2:
		existing.Stage--
		if existing.Stage <= 0 {
			w.removeHoleAt(fIdx, holeX)
		}
	case 4:
		w.emit(audio.SfxDigScrape)
	case 8:
		p.ShovelDown = true
	case 10:
		p.DigCounter = 0
		p.ShovelDown = false
	}
	return true
}

// StepMonsters runs one full monster update pass across slots 0..7.
func (w *World) StepMonsters() {
	fallPaused := w.isFallPaused()
	for i := range w.Monsters {
		if w.Phase != PhasePlaying {
			return
		}
		m := &w.Monsters[i]
		if !m.Active {
			continue
		}
		if fallPaused {
			if m.State != MonsterFalling {
				continue
			}
		} else if w.isFallPaused() && m.State != MonsterFalling {
			// Another monster in this pass just began falling; pause the remaining monsters.
			continue
		}
		w.stepSingleMonster(m)
	}
	w.checkLevelClear()
}

func (w *World) stepSingleMonster(m *Monster) {
	switch m.State {
	case MonsterFalling:
		w.stepFallingMonster(m)
		return

	case MonsterTrapped:
		// If the player finished filling in the hole, knock the monster down!
		h := w.FindHoleAt(m.TrappedFloor, m.TrappedHoleX)
		if h == nil {
			w.startMonsterFall(m)
			return
		}
		m.AnimFrame ^= 1
		m.WiggleTicks--
		if m.WiggleTicks <= 0 {
			m.State = MonsterEscaping
		}
		return

	case MonsterEscaping:
		// Climb out 1 scanline per monster pass, then repair/close the hole.
		m.Y--
		m.AnimFrame ^= 1
		if (m.Y-6)&0x1F == 0 {
			w.removeHoleAt(m.TrappedFloor, m.TrappedHoleX)
			m.State = MonsterPatrol
		}
		return
	}

	// State == MonsterPatrol:
	if w.monsterCollidesWithPlayer(m) {
		w.KillPlayerByMonster(m.Slot)
		return
	}

	// Check if walking onto an unoccupied hole:
	if fIdx := floorIndexForMonsterY(m.Y); fIdx >= 0 {
		if h := w.FindHoleAt(fIdx, m.X); h != nil && !h.Occupied {
			h.Occupied = true
			m.TrappedFloor = fIdx
			m.TrappedHoleX = h.X
			if h.Stage == 5 {
				m.State = MonsterTrapped
				m.WiggleTicks = TrappedWiggleTicks
				m.Y += 9
			} else {
				// Partial hole (1..4): dip in (+2..+5 px) and immediately climb back out,
				// destroying the player's unfinished hole.
				m.State = MonsterEscaping
				m.WiggleTicks = 0
				m.Y += h.Stage + 1
			}
			return
		}
	}

	// Move monster according to m.Dir
	w.movePatrollingMonster(m)

	if w.monsterCollidesWithPlayer(m) {
		w.KillPlayerByMonster(m.Slot)
		return
	}

	// Choose next patrol direction at junctions or when stopped
	w.updateMonsterAI(m)
}

func (w *World) movePatrollingMonster(m *Monster) {
	switch m.Dir {
	case DirNone:
		m.AnimFrame ^= 1
		return
	case DirRight:
		if m.X >= MonsterMaxX {
			m.Dir = DirNone
			return
		}
	case DirLeft:
		if m.X <= MonsterMinX {
			m.Dir = DirNone
			return
		}
	case DirUp:
		lMask := w.LadderMask(m.X, m.Y+9)
		if lMask&2 == 0 || m.Y <= 0x06 {
			m.Dir = DirNone
			return
		}
	case DirDown:
		lMask := w.LadderMask(m.X, m.Y+9)
		if lMask&1 == 0 || m.Y >= 0xA6 {
			m.Dir = DirNone
			return
		}
	}

	// Stop if the next step would overlap another active monster.
	nx, ny := m.X, m.Y
	switch m.Dir {
	case DirRight:
		nx += 2
	case DirLeft:
		nx -= 2
	case DirUp:
		ny -= 4
	case DirDown:
		ny += 4
	}
	for i := range w.Monsters {
		other := &w.Monsters[i]
		if !other.Active || other.Slot == m.Slot || other.State == MonsterFalling {
			continue
		}
		if monstersOverlap(nx, ny, other.X, other.Y) {
			m.Dir = DirNone
			return
		}
	}

	// Look 8 logical X units ahead so monsters turn away from already-occupied holes.
	if m.Dir == DirRight || m.Dir == DirLeft {
		if fIdx := floorIndexForMonsterY(m.Y); fIdx >= 0 {
			lookX := m.X + 8
			if m.Dir == DirLeft {
				lookX = m.X - 8
			}
			if h := w.FindHoleAt(fIdx, lookX); h != nil && h.Occupied {
				m.Dir = DirNone
				return
			}
		}
	}

	m.X = nx
	m.Y = ny
	m.AnimFrame ^= 1
}

func (w *World) updateMonsterAI(m *Monster) {
	onFloor := (m.Y-6)&0x1F == 0
	yHole := m.Y + 9
	lMask := w.LadderMask(m.X, yHole)

	// Decide whether to re-evaluate movement direction at this position:
	switch m.Dir {
	case DirNone:
		// Always choose a new direction
	case DirRight, DirLeft:
		// Only reconsider when passing a ladder!
		if lMask == 0 {
			return
		}
	case DirUp, DirDown:
		// Only reconsider when aligned with a floor!
		if !onFloor {
			return
		}
	}

	r := w.rng.Intn(256)
	switch lMask {
	case 0: // Not at a ladder: patrol horizontally
		if !onFloor {
			m.Dir = DirNone
		} else if r < 0x50 {
			m.Dir = DirRight
		} else {
			m.Dir = DirLeft
		}

	case 1: // Top of a ladder: can go Right, Left, or Down
		if !onFloor {
			m.Dir = DirDown
		} else if r < 0x50 {
			m.Dir = DirRight
		} else if r < 0xA0 {
			m.Dir = DirLeft
		} else {
			m.Dir = DirDown
		}

	case 2: // Bottom of a ladder: can go Right, Left, or Up
		if !onFloor {
			m.Dir = DirUp
		} else if r < 0x50 {
			m.Dir = DirRight
		} else if r < 0xA0 {
			m.Dir = DirLeft
		} else {
			m.Dir = DirUp
		}

	case 3: // Middle of a ladder: can go Right, Left, Up, or Down
		if !onFloor {
			if r < 0x80 {
				m.Dir = DirUp
			} else {
				m.Dir = DirDown
			}
		} else if r < 0x18 {
			m.Dir = DirRight
		} else if r < 0x90 {
			m.Dir = DirLeft
		} else if r < 0xF0 {
			m.Dir = DirUp
		} else {
			m.Dir = DirDown
		}
	}
}

func (w *World) startMonsterFall(m *Monster) {
	// Starting from m.TrappedFloor, snap Y back to the floor's base monster Y
	// so it falls cleanly down 32 scanlines to the next floor below.
	baseY := BrickFloorYs[m.TrappedFloor] - 10
	m.Y = baseY
	m.State = MonsterFalling
	m.HolesFallen = 1
	m.ChainKill = false
	m.Passengers = nil
	w.emit(audio.SfxMonsterFall)
	w.stepFallingMonster(m)
}

func (w *World) stepFallingMonster(m *Monster) {
	// Advance 8 scanlines per monster pass (4 passes = ~0.5s per floor, crisp visual fall).
	for sub := 0; sub < 8; sub++ {
		m.Y++
		rel := m.Y - 6
		if rel&0x1F != 0 {
			// Mid-air crush check against other walking/climbing monsters below
			for i := range w.Monsters {
				other := &w.Monsters[i]
				if !other.Active || other.Slot == m.Slot || other.State == MonsterFalling {
					continue
				}
				// Do not double-crush a trapped monster in a hole (handled as a chain knockdown at floor level).
				if other.State == MonsterTrapped {
					continue
				}
				if monstersOverlap(m.X, m.Y, other.X, other.Y) {
					pts := other.Type.BasePoints()
					w.AddScore(pts)
					w.Popups = append(w.Popups, ScorePopup{X: other.X, Y: other.Y, Points: pts, Timer: 0.75})
					w.emit(audio.SfxPointAward)
					other.Active = false
					other.KilledInThisLevel = true
				}
			}
			continue
		}

		// Reached the next floor down ((m.Y - 6) & 31 == 0)!
		fIdx := floorIndexForMonsterY(m.Y)
		if fIdx >= 0 {
			h := w.FindHoleAt(fIdx, m.X)
			if h != nil && h.Stage == 5 {
				wasOccupied := h.Occupied
				w.removeHoleAt(fIdx, m.X) // Closes aligned lower hole as monster crashes through
				m.HolesFallen++
				for pIdx := range m.Passengers {
					m.Passengers[pIdx].HolesFallen++
				}
				if wasOccupied {
					// Knocked out a second trapped monster (chain knockdown)
					m.ChainKill = true
					for i := range w.Monsters {
						other := &w.Monsters[i]
						if other.Active && other.Slot != m.Slot &&
							other.State == MonsterTrapped &&
							other.TrappedFloor == fIdx && other.TrappedHoleX == m.X {
							other.Active = false
							other.KilledInThisLevel = true
							m.Passengers = append(m.Passengers, FallPassenger{
								BasePoints:  other.Type.BasePoints(),
								HolesFallen: 1,
							})
							break
						}
					}
				}
				w.emit(audio.SfxMonsterFall)
				continue
			}
		}

		// Landed on a solid floor (or bottom ground at Y=166): evaluate survival vs death.
		dies := m.ChainKill || m.HolesFallen >= m.Type.HolesRequiredToKill()
		if dies {
			totalPts := m.Type.BasePoints() * m.HolesFallen
			for _, pass := range m.Passengers {
				totalPts += pass.BasePoints * pass.HolesFallen
			}
			w.AddScore(totalPts)
			w.Popups = append(w.Popups, ScorePopup{
				X:      m.X,
				Y:      m.Y,
				Points: totalPts,
				Timer:  0.85,
			})
			w.emit(audio.SfxPointAward)
			m.Active = false
			m.KilledInThisLevel = true
			return
		}

		// Survived! (Butterfly fell 1 floor, or Mask fell 1-2 floors)
		if fIdx >= 0 {
			for _, h := range w.Holes {
				if h.FloorIdx == fIdx && h.Stage > 0 && h.Stage < 5 && abs(m.X-h.X) < 6 {
					if h.X < m.X {
						m.X = clampEven(h.X+8, MonsterMinX, MonsterMaxX)
					} else {
						m.X = clampEven(h.X-8, MonsterMinX, MonsterMaxX)
					}
					break
				}
			}
		}
		m.State = MonsterPatrol
		m.Dir = DirNone
		w.Phase = PhaseSirenFreeze
		w.PhaseTimer = 0.60
		w.emit(audio.SfxSurviveSiren)
		return
	}
}

// monstersOverlap checks bounding-box collision between two monsters (width 6, height 9).
func monstersOverlap(x1, y1, x2, y2 int) bool {
	return x1+6 >= x2 && x2+6 >= x1 && y1+9 >= y2 && y2+9 >= y1
}

// monsterCollidesWithPlayer checks collision between a monster (4x7) and the player (4x14).
func (w *World) monsterCollidesWithPlayer(m *Monster) bool {
	if !m.Active || m.State == MonsterTrapped || m.State == MonsterFalling {
		return false
	}
	p := &w.Player
	return m.X+4 >= p.X && p.X+4 >= m.X && m.Y+7 >= p.Y && p.Y+14 >= m.Y
}

func (w *World) checkAllMonsterCollisionsWithPlayer() {
	if w.Phase != PhasePlaying {
		return
	}
	for i := range w.Monsters {
		if w.monsterCollidesWithPlayer(&w.Monsters[i]) {
			w.KillPlayerByMonster(w.Monsters[i].Slot)
			return
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampEven(v, lo, hi int) int {
	v &= ^1
	return clamp(v, lo, hi)
}
