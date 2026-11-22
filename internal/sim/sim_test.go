package sim

import (
	"slices"
	"testing"

	"apple-panic/internal/config"
	"apple-panic/internal/levels"
)

func newActiveWorld(acc config.Accessibility) *World {
	w := NewWorld(42, 0, acc)
	w.Ladders = nil // Clear ladders when testing pure hole physics unless needed
	w.Phase = PhasePlaying
	for i := range w.Monsters {
		w.Monsters[i].Active = false
	}
	return w
}

func TestDigAndFillHoleStages(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	acc.ToggleControls = false
	w := newActiveWorld(acc)

	// Place player on Floor 1 (Y = 0x80 -> brick girder at Y = 0x90, floorIdx = 4)
	w.Player.X = 44 // Even X: AutoAlignDigging should snap to odd X=45 facing right -> holeX=52!
	w.Player.Y = 0x80
	w.Player.FacingRight = true

	// Cannot dig on ground floor (Y = 0xA0)
	w.Player.Y = 0xA0
	for i := 0; i < 15; i++ {
		w.StepPlayer(InputSnapshot{Dig: true})
	}
	if len(w.Holes) != 0 {
		t.Fatalf("should not dig on ground floor, got %d holes", len(w.Holes))
	}

	// Move back to brick Floor 1 (Y = 0x80) and dig 5 scoops (50 player ticks)
	w.Player.Y = 0x80
	for i := 0; i < 50; i++ {
		w.StepPlayer(InputSnapshot{Dig: true})
	}
	h := w.FindHoleAt(4, 52)
	if h == nil || h.Stage != 5 {
		t.Fatalf("expected Stage 5 hole at floor 4 X=52, got %+v", h)
	}

	// Fill 5 scoops (50 player ticks) -> hole should be completely removed!
	for i := 0; i < 50; i++ {
		w.StepPlayer(InputSnapshot{Fill: true})
	}
	if got := w.FindHoleAt(4, 52); got != nil {
		t.Fatalf("expected hole to be completely filled and removed, got %+v", got)
	}
}

func TestSpeciesMortalityAndAlignedHoles(t *testing.T) {
	acc := config.DefaultConfig().Accessibility

	// 1. Apple dies through 1 hole (100 pts)
	{
		w := newActiveWorld(acc)
		w.Holes = []Hole{{FloorIdx: 4, X: 40, Stage: 5}}
		w.Monsters[0] = Monster{Slot: 0, Active: true, Type: levels.MonsterApple, X: 40, Y: 0x86, State: MonsterPatrol}
		w.Monsters[1] = Monster{Slot: 1, Active: true, Type: levels.MonsterApple, X: 100, Y: 0x06, State: MonsterPatrol}

		w.StepMonsters() // Apple falls into hole (State = MonsterTrapped)
		if w.Monsters[0].State != MonsterTrapped {
			t.Fatalf("expected Apple trapped, got %v", w.Monsters[0].State)
		}
		// Player fills hole completely
		w.removeHoleAt(4, 40)
		for i := 0; i < 10 && w.Monsters[0].Active; i++ {
			w.StepMonsters()
		}
		if w.Monsters[0].Active {
			t.Fatalf("expected Apple to die after 1 hole")
		}
		if w.Score != 100 {
			t.Fatalf("expected 100 pts for 1-hole Apple, got %d", w.Score)
		}
	}

	// 2. Butterfly survives 1 hole (0 pts, triggers SirenFreeze), dies through 2 aligned holes (400 pts)
	{
		w := newActiveWorld(acc)
		w.Holes = []Hole{{FloorIdx: 3, X: 40, Stage: 5}} // Floor Y=0x70
		w.Monsters[0] = Monster{Slot: 0, Active: true, Type: levels.MonsterButterfly, X: 40, Y: 0x66, State: MonsterPatrol}
		w.Monsters[1] = Monster{Slot: 1, Active: true, Type: levels.MonsterApple, X: 100, Y: 0x06, State: MonsterPatrol}

		w.StepMonsters() // Trapped
		w.removeHoleAt(3, 40)
		for i := 0; i < 10 && w.Monsters[0].State == MonsterFalling; i++ {
			w.StepMonsters()
		}
		// First StepMonsters transitions Trapped -> Falling and steps 8px, run 4 more passes:
		for i := 0; i < 5 && w.Phase == PhasePlaying; i++ {
			w.StepMonsters()
		}
		if !w.Monsters[0].Active {
			t.Fatalf("Butterfly should SURVIVE a 1-hole drop!")
		}
		if w.Phase != PhaseSirenFreeze || w.Score != 0 {
			t.Fatalf("expected PhaseSirenFreeze and 0 pts, got phase=%v score=%d", w.Phase, w.Score)
		}

		// Now align 2 holes (Floor 3 Y=0x70 and Floor 4 Y=0x90) at exact X=40!
		w.Phase = PhasePlaying
		w.Monsters[0].Y = 0x66
		w.Monsters[0].X = 40
		w.Holes = []Hole{
			{FloorIdx: 3, X: 40, Stage: 5},
			{FloorIdx: 4, X: 40, Stage: 5},
		}
		w.StepMonsters() // Trapped in upper hole
		w.removeHoleAt(3, 40)
		for i := 0; i < 15 && w.Monsters[0].Active; i++ {
			w.StepMonsters()
		}
		if w.Monsters[0].Active {
			t.Fatalf("Butterfly should DIE after falling through 2 aligned holes!")
		}
		if w.Score != 400 {
			t.Fatalf("expected 400 pts (200 * 2 holes) for Butterfly, got %d", w.Score)
		}
		if w.FindHoleAt(4, 40) != nil {
			t.Fatalf("lower aligned hole should close after monster crashes through it!")
		}
	}

	// 3. Mask survives 2 aligned holes, dies through 3 aligned holes (900 pts)
	{
		w := newActiveWorld(acc)
		w.Monsters[0] = Monster{Slot: 0, Active: true, Type: levels.MonsterMask, X: 60, Y: 0x46, State: MonsterPatrol}
		w.Monsters[1] = Monster{Slot: 1, Active: true, Type: levels.MonsterApple, X: 100, Y: 0x06, State: MonsterPatrol}

		// 2 aligned holes (Floor 2 & Floor 3): Mask must survive!
		w.Holes = []Hole{
			{FloorIdx: 2, X: 60, Stage: 5},
			{FloorIdx: 3, X: 60, Stage: 5},
		}
		w.StepMonsters()
		w.removeHoleAt(2, 60)
		for i := 0; i < 15 && w.Phase == PhasePlaying; i++ {
			w.StepMonsters()
		}
		if !w.Monsters[0].Active || w.Phase != PhaseSirenFreeze || w.Score != 0 {
			t.Fatalf("Mask should survive 2 holes! active=%v phase=%v score=%d", w.Monsters[0].Active, w.Phase, w.Score)
		}

		// 3 aligned holes (Floor 2, Floor 3, Floor 4): Mask must die for 900 pts!
		w.Phase = PhasePlaying
		w.Monsters[0].X = 60
		w.Monsters[0].Y = 0x46
		w.Holes = []Hole{
			{FloorIdx: 2, X: 60, Stage: 5},
			{FloorIdx: 3, X: 60, Stage: 5},
			{FloorIdx: 4, X: 60, Stage: 5},
		}
		w.StepMonsters()
		w.removeHoleAt(2, 60)
		for i := 0; i < 20 && w.Monsters[0].Active; i++ {
			w.StepMonsters()
		}
		if w.Monsters[0].Active {
			t.Fatalf("Mask should die after 3 aligned holes!")
		}
		if w.Score != 900 {
			t.Fatalf("expected 900 pts (300 * 3 holes) for Mask, got %d", w.Score)
		}
	}
}

func TestAccessibilityKeepKilledMonstersDead(t *testing.T) {
	// Case A: Classic mode (KeepKilledMonstersDead = false) -> all 3 monsters respawn on death!
	accClassic := config.DefaultConfig().Accessibility
	accClassic.KeepKilledMonstersDead = false
	wClassic := NewWorld(1, 0, accClassic)
	wClassic.Phase = PhasePlaying

	// Kill monsters 0 and 1 on Level 1 (leaving monster 2 alive)
	wClassic.Monsters[0].Active = false
	wClassic.Monsters[0].KilledInThisLevel = true
	wClassic.Monsters[1].Active = false
	wClassic.Monsters[1].KilledInThisLevel = true

	// Player dies and respawns
	wClassic.KillPlayer()
	for i := 0; i < 30 && wClassic.Phase == PhaseDying; i++ {
		wClassic.Update(0.05, InputSnapshot{})
	}
	activeCountClassic := 0
	for _, m := range wClassic.Monsters {
		if m.Active {
			activeCountClassic++
		}
	}
	if activeCountClassic != 3 {
		t.Fatalf("Classic mode expected all 3 monsters to respawn on death, got %d", activeCountClassic)
	}

	// Case B: Accessibility mode (KeepKilledMonstersDead = true) -> killed monsters stay dead!
	accKeep := config.DefaultConfig().Accessibility
	accKeep.KeepKilledMonstersDead = true
	wKeep := NewWorld(1, 0, accKeep)
	wKeep.Phase = PhasePlaying

	wKeep.Monsters[0].Active = false
	wKeep.Monsters[0].KilledInThisLevel = true
	wKeep.Monsters[1].Active = false
	wKeep.Monsters[1].KilledInThisLevel = true

	wKeep.KillPlayer()
	for i := 0; i < 30 && wKeep.Phase == PhaseDying; i++ {
		wKeep.Update(0.05, InputSnapshot{})
	}
	activeCountKeep := 0
	for _, m := range wKeep.Monsters {
		if m.Active {
			activeCountKeep++
		}
	}
	if activeCountKeep != 1 || !wKeep.Monsters[2].Active {
		t.Fatalf("KeepKilledMonstersDead=true expected only monster #2 to respawn (1 active), got %d", activeCountKeep)
	}
}

func TestPartialHoleDestroyedByMonsterAndExtraLife(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	w := newActiveWorld(acc)
	w.Holes = []Hole{{FloorIdx: 4, X: 30, Stage: 3}}
	w.Monsters[0] = Monster{Slot: 0, Active: true, Type: levels.MonsterApple, X: 30, Y: 0x86, State: MonsterPatrol}
	w.Monsters[1] = Monster{Slot: 1, Active: true, Type: levels.MonsterApple, X: 100, Y: 0x06, State: MonsterPatrol}

	w.StepMonsters() // Enters MonsterEscaping (+4 px dip)
	if w.Monsters[0].State != MonsterEscaping {
		t.Fatalf("expected MonsterEscaping on partial hole, got %v", w.Monsters[0].State)
	}
	for i := 0; i < 6 && w.Monsters[0].State == MonsterEscaping; i++ {
		w.StepMonsters()
	}
	if w.FindHoleAt(4, 30) != nil {
		t.Fatalf("expected unfinished partial hole to be destroyed by escaping monster")
	}

	// Extra life awarded once at 4,000 points
	initialLives := w.Lives
	w.AddScore(3900)
	if w.Lives != initialLives {
		t.Fatalf("should not award extra life before 4000")
	}
	w.AddScore(100)
	if w.Lives != initialLives+1 {
		t.Fatalf("expected +1 extra life at 4000 pts, got %d", w.Lives)
	}
	w.AddScore(5000)
	if w.Lives != initialLives+1 {
		t.Fatalf("extra life should only be awarded once per game")
	}
}

func TestToggleControlsObstacleAndReversal(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	acc.ToggleControls = true
	w := newActiveWorld(acc)
	w.Player.X = 20
	w.Player.Y = 0xA0

	// Single tap Left moves continuously until hitting PlayerMinX (X = 1)
	w.StepPlayer(InputSnapshot{PressLeft: true})
	if w.Player.X != 19 || w.Player.LatchedDir != DirLeft {
		t.Fatalf("expected X=19 LatchedDir=DirLeft, got X=%d dir=%v", w.Player.X, w.Player.LatchedDir)
	}

	// Mid-walk tap Right immediately reverses direction
	w.StepPlayer(InputSnapshot{PressRight: true})
	if w.Player.X != 20 || w.Player.LatchedDir != DirRight {
		t.Fatalf("expected immediate reversal to X=20 DirRight, got X=%d dir=%v", w.Player.X, w.Player.LatchedDir)
	}

	// Tap Left again and run with empty InputSnapshots until hitting the left wall
	w.StepPlayer(InputSnapshot{PressLeft: true})
	for i := 0; i < 30; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	if w.Player.X != PlayerMinX || w.Player.LatchedDir != DirNone || w.Player.State != PlayerIdle {
		t.Fatalf("expected player to stop at left wall X=%d, got X=%d dir=%v state=%v",
			PlayerMinX, w.Player.X, w.Player.LatchedDir, w.Player.State)
	}

	// Blocking obstacle test: occupied hole at X=40 on Floor 4 (Y=0x80) stops rightward walk at X=33 (40 - 7)
	w.Player.X = 20
	w.Player.Y = 0x80
	w.Holes = []Hole{{FloorIdx: 4, X: 40, Stage: 5, Occupied: true}}
	w.StepPlayer(InputSnapshot{PressRight: true})
	for i := 0; i < 30; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	if w.Player.X != 33 || w.Player.LatchedDir != DirNone {
		t.Fatalf("expected player to stop at filling stance X=33 before occupied hole, got X=%d dir=%v",
			w.Player.X, w.Player.LatchedDir)
	}
}

func TestToggleControlsLadderJunctionQueueing(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	acc.ToggleControls = true
	w := newActiveWorld(acc)
	w.Ladders = []levels.Ladder{
		{X: 32, YTop: 0x0F, YBot: 0xAF}, // Full-height ladder at X=32
	}
	w.Player.X = 24
	w.Player.Y = 0xA0 // Ground floor (160)

	// 1. Tap Right at X=24, then immediately queue Up while walking toward ladder at X=32
	w.StepPlayer(InputSnapshot{PressRight: true}) // X=25
	w.StepPlayer(InputSnapshot{PressUp: true})    // Queues DirUp, advances to X=26
	if w.Player.QueuedDir != DirUp {
		t.Fatalf("expected QueuedDir=DirUp, got %v", w.Player.QueuedDir)
	}
	// Run until player reaches X=32 and climbs partway between Floor 0 (160) and Floor 1 (128)
	for i := 0; i < 10; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	if w.Player.X != 32 || w.Player.Y >= 0xA0 || w.Player.Y <= 0x80 || w.Player.LatchedDir != DirUp {
		t.Fatalf("expected player climbing ladder at X=32 between 128..160, got (%d, %d) dir=%v",
			w.Player.X, w.Player.Y, w.Player.LatchedDir)
	}

	// 2. While mid-ladder, tap Left, then change mind and tap Right (single-slot queue replacement!)
	w.StepPlayer(InputSnapshot{PressLeft: true})
	if w.Player.QueuedDir != DirLeft {
		t.Fatalf("expected QueuedDir=DirLeft, got %v", w.Player.QueuedDir)
	}
	w.StepPlayer(InputSnapshot{PressRight: true})
	if w.Player.QueuedDir != DirRight {
		t.Fatalf("expected QueuedDir replaced with DirRight, got %v", w.Player.QueuedDir)
	}

	// 3. Continue stepping: player must exit the ladder to the RIGHT at Floor 1 (Y = 0x80 = 128)!
	for i := 0; i < 16; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	if w.Player.Y != 0x80 || w.Player.X <= 32 || w.Player.LatchedDir != DirRight || w.Player.QueuedDir != DirNone {
		t.Fatalf("expected player to exit ladder right onto Y=0x80, got (%d, %d) latched=%v queued=%v",
			w.Player.X, w.Player.Y, w.Player.LatchedDir, w.Player.QueuedDir)
	}
}

func TestToggleControlsSkipsBlockedJunctionToNextAvailable(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	acc.ToggleControls = true
	w := newActiveWorld(acc)
	// Ladder at odd X=5 so X+7=12 can hold a blocking partial hole on Floor 1 (Y=0x80, floorIdx=4)
	w.Ladders = []levels.Ladder{
		{X: 5, YTop: 0x0F, YBot: 0xAF},
	}
	w.Holes = []Hole{
		{FloorIdx: 4, X: 12, Stage: 3}, // Blocks exiting Right on Floor 1 (Y=0x80)!
	}
	w.Player.X = 5
	w.Player.Y = 0xA0

	w.StepPlayer(InputSnapshot{PressUp: true})    // Y=158
	w.StepPlayer(InputSnapshot{PressRight: true}) // Queues Right; Y=156

	// Advance enough ticks to pass Floor 1 (Y=0x80) and reach Floor 2 (Y=0x60) + walk right
	for i := 0; i < 36; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	if w.Player.Y != 0x60 || w.Player.X <= 5 || w.Player.LatchedDir != DirRight {
		t.Fatalf("expected player to skip blocked Floor 1 and exit Right on Floor 2 (Y=0x60), got (%d, %d) dir=%v",
			w.Player.X, w.Player.Y, w.Player.LatchedDir)
	}
}

func TestToggleControls60HzSubTickAndDigLatch(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	acc.ToggleControls = true
	w := newActiveWorld(acc)
	// Keep 1 dummy monster far away so level does not auto-clear during Update()
	w.Monsters[0] = Monster{Slot: 0, Active: true, Type: levels.MonsterApple, X: 120, Y: 0x06, State: MonsterPatrol}
	w.Ladders = []levels.Ladder{{X: 40, YTop: 0x0F, YBot: 0xAF}}
	w.Player.X = 40
	w.Player.Y = 0xA0

	// Rapid 60Hz taps on consecutive 1/60s frames (before a 15Hz StepPlayer tick even fires!)
	w.Update(1.0/60.0, InputSnapshot{PressUp: true})
	w.Update(1.0/60.0, InputSnapshot{PressRight: true})
	for i := 0; i < 120; i++ {
		w.Update(1.0/60.0, InputSnapshot{})
	}
	if w.Player.Y != 0x80 || w.Player.X <= 40 {
		t.Fatalf("expected 60Hz sub-tick Up+Right taps to climb and exit right at Y=0x80, got (%d, %d)",
			w.Player.X, w.Player.Y)
	}

	// Single PressDig tap stops horizontal walking, digs a complete Stage-5 hole, and stops
	w.Player.X = 61
	w.Player.Y = 0x80
	w.Player.FacingRight = true
	w.StepPlayer(InputSnapshot{PressRight: true}) // Walking right (X=62)
	w.StepPlayer(InputSnapshot{PressDig: true})   // Cancels walk, snaps odd X=63, starts digging at holeX=70
	for i := 0; i < 60; i++ {
		w.StepPlayer(InputSnapshot{})
	}
	h := w.FindHoleAt(4, 70)
	if h == nil || h.Stage != 5 {
		t.Fatalf("expected single PressDig tap in Toggle mode to complete Stage 5 hole, got %+v", h)
	}
	if w.Player.LatchedAct != LatchNone || w.Player.State != PlayerIdle {
		t.Fatalf("expected player to return to idle after completing latched dig, got act=%v state=%v",
			w.Player.LatchedAct, w.Player.State)
	}
}

func TestMonsterCatchJumpAnimation(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	w := newActiveWorld(acc)
	w.Player.X = 64
	w.Player.Y = 0xA0 // 160
	w.Monsters[2] = Monster{
		Slot:   2,
		Active: true,
		Type:   levels.MonsterApple,
		X:      60,
		Y:      0xA6, // 166
		Dir:    DirRight,
		State:  MonsterPatrol,
	}

	// Monster collides with player (60+4 >= 64 and 160+14 >= 166)
	w.StepMonsters()
	if w.Phase != PhaseDying {
		t.Fatalf("expected PhaseDying on monster catch, got %v", w.Phase)
	}
	if w.CatchingMonsterSlot != 2 {
		t.Fatalf("expected CatchingMonsterSlot=2, got %d", w.CatchingMonsterSlot)
	}

	// At t=0, catching monster starts at its capture position (60, 166)
	x0, y0, landed0, ok0 := w.CatchingMonsterPos()
	if !ok0 || landed0 || x0 != 60 || y0 != 166 {
		t.Fatalf("expected initial catch pos (60, 166) landed=false, got (%.2f, %.2f) landed=%v ok=%v", x0, y0, landed0, ok0)
	}

	// Mid-jump (~0.14s at 60Hz): monster arcs above both startY (166) and targetY (160)
	for i := 0; i < 8; i++ {
		w.Update(1.0/60.0, InputSnapshot{})
	}
	xm, ym, landedm, okm := w.CatchingMonsterPos()
	if !okm || landedm {
		t.Fatalf("expected mid-jump ok=true landed=false, got ok=%v landed=%v", okm, landedm)
	}
	if xm <= 60 || xm >= 64 {
		t.Fatalf("expected mid-jump X strictly between 60 and 64, got %.2f", xm)
	}
	if ym >= 160 {
		t.Fatalf("expected mid-jump Y to arc above player head (160), got %.2f", ym)
	}

	// After jump completes (~0.30s total), monster rests squarely atop player's head (64, 160)
	for i := 0; i < 12; i++ {
		w.Update(1.0/60.0, InputSnapshot{})
	}
	xl, yl, landedl, okl := w.CatchingMonsterPos()
	if !okl || !landedl || xl != 64 || yl != 160 {
		t.Fatalf("expected landed catch pos (64, 160) landed=true, got (%.2f, %.2f) landed=%v ok=%v", xl, yl, landedl, okl)
	}

	// Finish PhaseDying -> level retries and clears CatchingMonsterSlot
	for i := 0; i < 30 && w.Phase == PhaseDying; i++ {
		w.Update(0.05, InputSnapshot{})
	}
	if w.Phase != PhaseReady || w.CatchingMonsterSlot != -1 {
		t.Fatalf("expected PhaseReady and CatchingMonsterSlot=-1 after death retry, got phase=%v slot=%d",
			w.Phase, w.CatchingMonsterSlot)
	}

	// Oxygen death (KillPlayer) should not trigger a catching monster animation
	w.Phase = PhasePlaying
	w.KillPlayer()
	if _, _, _, ok := w.CatchingMonsterPos(); ok {
		t.Fatalf("expected CatchingMonsterPos ok=false for oxygen death")
	}
}

func TestPauseWhileMonsterFallsEnabledAndDisabled(t *testing.T) {
	setupScenario := func(pauseOnFall bool) *World {
		acc := config.DefaultConfig().Accessibility
		acc.ToggleControls = false
		acc.PauseWhileMonsterFalls = pauseOnFall
		w := newActiveWorld(acc)

		// Player on ground floor walking right
		w.Player.X = 20
		w.Player.Y = 0xA0
		w.Player.FacingRight = true

		// Monster 0 (slot < falling slot): patrolling right on top floor
		w.Monsters[0] = Monster{
			Slot: 0, Active: true, Type: levels.MonsterApple,
			X: 20, Y: 0x06, Dir: DirRight, State: MonsterPatrol,
		}
		// Monster 1: trapped on Floor 3 (Y=0x70) with hole already removed so next pass starts its fall
		w.Monsters[1] = Monster{
			Slot: 1, Active: true, Type: levels.MonsterApple,
			X: 60, Y: 0x66 + 9, State: MonsterTrapped,
			TrappedFloor: 3, TrappedHoleX: 60, WiggleTicks: TrappedWiggleTicks,
		}
		// Monster 2 (slot > falling slot): patrolling right on Floor 2
		w.Monsters[2] = Monster{
			Slot: 2, Active: true, Type: levels.MonsterApple,
			X: 20, Y: 0x46, Dir: DirRight, State: MonsterPatrol,
		}
		// Monster 3: trapped in an intact hole on Floor 1
		w.Holes = []Hole{{FloorIdx: 1, X: 90, Stage: 5, Occupied: true}}
		w.Monsters[3] = Monster{
			Slot: 3, Active: true, Type: levels.MonsterApple,
			X: 90, Y: 0x26 + 9, State: MonsterTrapped,
			TrappedFloor: 1, TrappedHoleX: 90, WiggleTicks: 50,
		}
		return w
	}

	// Case A: PauseWhileMonsterFalls = true (default authentic Apple II behavior)
	{
		w := setupScenario(true)

		// Pass 1: Monster 0 moves once, Monster 1 starts falling, Monster 2 & 3 immediately freeze!
		w.StepMonsters()
		if !w.HasFallingMonster() || w.Monsters[1].State != MonsterFalling {
			t.Fatalf("expected Monster 1 to be falling, got state=%v", w.Monsters[1].State)
		}
		if w.Monsters[2].X != 20 {
			t.Fatalf("expected Monster 2 (after falling slot) to pause immediately, got X=%d", w.Monsters[2].X)
		}
		if w.Monsters[3].WiggleTicks != 50 {
			t.Fatalf("expected Monster 3 wiggle ticks to pause immediately, got %d", w.Monsters[3].WiggleTicks)
		}

		m0XDuringFall := w.Monsters[0].X
		playerXBefore := w.Player.X
		oxyCountBefore := w.OxygenLoopCount
		bonusCountBefore := w.BonusLoopCount

		// Direct StepPlayer while monster is falling must be a no-op
		w.StepPlayer(InputSnapshot{Right: true})
		if w.Player.X != playerXBefore {
			t.Fatalf("expected direct StepPlayer to pause while monster falls, got X=%d", w.Player.X)
		}

		// Advance via Update() while Monster 1 is still falling (2 more monster passes -> ~0.30s)
		for i := 0; i < 16 && w.HasFallingMonster(); i++ {
			w.Update(1.0/60.0, InputSnapshot{Right: true})
			if w.HasFallingMonster() {
				if w.Player.X != playerXBefore {
					t.Fatalf("expected player to remain paused at X=%d while falling, got X=%d", playerXBefore, w.Player.X)
				}
				if w.Monsters[0].X != m0XDuringFall {
					t.Fatalf("expected Monster 0 to remain paused at X=%d while falling, got X=%d", m0XDuringFall, w.Monsters[0].X)
				}
				if w.Monsters[2].X != 20 {
					t.Fatalf("expected Monster 2 to remain paused at X=20 while falling, got X=%d", w.Monsters[2].X)
				}
				if w.Monsters[3].WiggleTicks != 50 {
					t.Fatalf("expected Monster 3 wiggle ticks to remain 50 while falling, got %d", w.Monsters[3].WiggleTicks)
				}
				if w.OxygenLoopCount != oxyCountBefore || w.BonusLoopCount != bonusCountBefore {
					t.Fatalf("expected timers to remain paused while falling")
				}
			}
		}

		// Finish the fall
		for i := 0; i < 60 && w.HasFallingMonster(); i++ {
			w.Update(1.0/60.0, InputSnapshot{Right: true})
		}
		if w.HasFallingMonster() || w.Monsters[1].Active {
			t.Fatalf("expected Monster 1 to finish falling and die")
		}
		if w.Score != 100 {
			t.Fatalf("expected 100 pts after Monster 1 fall, got %d", w.Score)
		}

		// Subsequent Updates resume player and monster movement!
		for i := 0; i < 20; i++ {
			w.Update(1.0/60.0, InputSnapshot{Right: true})
		}
		if w.Player.X <= playerXBefore {
			t.Fatalf("expected player to resume moving after fall, still at X=%d", w.Player.X)
		}
		if w.Monsters[0].X <= m0XDuringFall || w.Monsters[2].X <= 20 {
			t.Fatalf("expected patrolling monsters to resume after fall, got m0.X=%d m2.X=%d", w.Monsters[0].X, w.Monsters[2].X)
		}
		if w.Monsters[3].WiggleTicks >= 50 {
			t.Fatalf("expected Monster 3 wiggle ticks to resume after fall, got %d", w.Monsters[3].WiggleTicks)
		}
	}

	// Case B: PauseWhileMonsterFalls = false (player and other monsters keep moving)
	{
		w := setupScenario(false)
		w.StepMonsters() // Monster 1 starts falling
		if !w.HasFallingMonster() {
			t.Fatalf("expected Monster 1 to be falling")
		}
		if w.Monsters[2].X <= 20 {
			t.Fatalf("expected Monster 2 to keep moving when PauseWhileMonsterFalls=false, got X=%d", w.Monsters[2].X)
		}
		if w.Monsters[3].WiggleTicks >= 50 {
			t.Fatalf("expected Monster 3 wiggle ticks to decrement when PauseWhileMonsterFalls=false, got %d", w.Monsters[3].WiggleTicks)
		}

		// Player also moves while Monster 1 is still mid-fall
		w.StepPlayer(InputSnapshot{Right: true})
		if w.Player.X <= 20 {
			t.Fatalf("expected player to move while monster falls when PauseWhileMonsterFalls=false, got X=%d", w.Player.X)
		}
	}
}

func TestPlayerDeathPreservesLadderLayout(t *testing.T) {
	acc := config.DefaultConfig().Accessibility
	w := NewWorld(1, 0, acc)
	w.Phase = PhasePlaying

	initialLadders := slices.Clone(w.Ladders)
	if len(initialLadders) == 0 {
		t.Fatalf("expected non-empty initial ladder layout")
	}

	// Dig a hole and move a monster before dying.
	w.Holes = append(w.Holes, Hole{FloorIdx: 4, X: 40, Stage: 5})
	w.Monsters[0].X = 12
	w.Monsters[0].Y = 0x86

	// Die twice (while lives remain) and verify ladder layout is unchanged,
	// holes are filled in, and monsters are reset.
	for death := 1; death <= 2; death++ {
		w.Phase = PhasePlaying
		w.KillPlayer()
		for i := 0; i < 30 && w.Phase == PhaseDying; i++ {
			w.Update(0.05, InputSnapshot{})
		}
		if w.Phase != PhaseReady {
			t.Fatalf("death %d: expected PhaseReady after death retry, got %v", death, w.Phase)
		}
		if !slices.Equal(w.Ladders, initialLadders) {
			t.Fatalf("death %d: expected ladder layout to remain unchanged after player death", death)
		}
		if len(w.Holes) != 0 {
			t.Fatalf("death %d: expected holes to be filled in after player death, got %d", death, len(w.Holes))
		}
		if !w.Monsters[0].Active || w.Monsters[0].State != MonsterPatrol {
			t.Fatalf("death %d: expected monsters to reset to active patrol", death)
		}
	}
}


