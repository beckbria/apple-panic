package levels

// MonsterType identifies the three enemy species in Apple Panic.
type MonsterType int

const (
	MonsterApple     MonsterType = 0 // Falls through 1 hole to die (100 pts base)
	MonsterButterfly MonsterType = 1 // Falls through 2 aligned holes to die (200 pts base)
	MonsterMask      MonsterType = 2 // Falls through 3 aligned holes to die (300 pts base)
)

// HolesRequiredToKill returns the minimum number of aligned holes a monster
// must fall through to die (unless involved in a multi-monster chain knockdown
// or crushed in mid-air).
func (m MonsterType) HolesRequiredToKill() int {
	switch m {
	case MonsterApple:
		return 1
	case MonsterButterfly:
		return 2
	case MonsterMask:
		return 3
	default:
		return 1
	}
}

// BasePoints returns the base score (in points) for 1 hole fallen through.
func (m MonsterType) BasePoints() int {
	return (int(m) + 1) * 100
}

// Name returns the display name of the monster species.
func (m MonsterType) Name() string {
	switch m {
	case MonsterApple:
		return "Apple"
	case MonsterButterfly:
		return "Butterfly"
	case MonsterMask:
		return "Mask"
	default:
		return "Unknown"
	}
}

// MaxLevel is the final playable level; clearing it triggers the victory screen.
const MaxLevel = 48

// SubLevelCounts defines how many monsters spawn across the 3 sub-levels in each
// difficulty group: the first sub-level spawns 3 monsters, the second spawns 5,
// and the third spawns all 8.
var SubLevelCounts = [3]int{3, 5, 8}

// MonstersForLevel returns the active monster roster for level (1..48).
//
// The 48 levels are divided into 16 difficulty groups of 3 sub-levels each
// (spawning 3, 5, then 8 monsters). Tougher species are front-loaded so they
// appear even in 3- and 5-monster sub-levels:
//   - Group 0 (Levels 1-3):       8 Apples
//   - Group 1 (Levels 4-6):       1 Butterfly, 7 Apples
//   - Groups 2-8 (Levels 7-27):   1 Mask, (group-1) Butterflies, remaining Apples
//   - Groups 9-15 (Levels 28-48): (group-7) Masks, remaining Butterflies (8 Masks at Group 15)
func MonstersForLevel(level int) []MonsterType {
	if level < 1 {
		level = 1
	}
	if level > MaxLevel {
		level = MaxLevel
	}
	idx := level - 1
	group := idx / 3
	sub := idx % 3
	count := SubLevelCounts[sub]

	masks := 0
	if group >= 9 {
		masks = group - 7
	} else if group >= 2 {
		masks = 1
	}

	out := make([]MonsterType, count)
	for i := 0; i < count; i++ {
		switch {
		case i < masks:
			out[i] = MonsterMask
		case i < group:
			out[i] = MonsterButterfly
		default:
			out[i] = MonsterApple
		}
	}
	return out
}

// StartingBonus returns the initial level bonus, starting at 2,000 for Levels 1-3
// and increasing by 1,000 every 4 levels up to a cap of 9,000 at Level 28+.
func StartingBonus(level int) int {
	if level < 1 {
		level = 1
	}
	q := level >> 2
	if q > 7 {
		q = 7
	}
	return (q + 2) * 1000
}

// Ladder describes one vertical ladder segment in logical coordinates.
// YTop and YBot are the scanlines immediately above the top and bottom floors
// connected by the ladder (floorY - 1: 15, 47, 79, 111, 143, or 175).
type Ladder struct {
	X    int
	YTop int
	YBot int
}

// LadderLayouts defines the 4 rotating ladder configurations cycled across levels and lives.
var LadderLayouts = [4][]Ladder{
	// Layout 0: full-height outer ladders with staggered interior ladders
	{
		{X: 0x04, YTop: 0x0F, YBot: 0xAF},
		{X: 0x18, YTop: 0x0F, YBot: 0x2F},
		{X: 0x20, YTop: 0x8F, YBot: 0xAF},
		{X: 0x28, YTop: 0x4F, YBot: 0x8F},
		{X: 0x30, YTop: 0x0F, YBot: 0x4F},
		{X: 0x50, YTop: 0x2F, YBot: 0x4F},
		{X: 0x50, YTop: 0x8F, YBot: 0xAF},
		{X: 0x64, YTop: 0x2F, YBot: 0x6F},
		{X: 0x80, YTop: 0x0F, YBot: 0xAF},
	},
	// Layout 1: segmented left edge ladders with staggered middle/right ladders
	{
		{X: 0x04, YTop: 0x0F, YBot: 0x2F},
		{X: 0x04, YTop: 0x4F, YBot: 0x6F},
		{X: 0x04, YTop: 0x8F, YBot: 0xAF},
		{X: 0x30, YTop: 0x2F, YBot: 0x6F},
		{X: 0x34, YTop: 0x8F, YBot: 0xAF},
		{X: 0x60, YTop: 0x0F, YBot: 0x2F},
		{X: 0x76, YTop: 0x2F, YBot: 0x6F},
		{X: 0x82, YTop: 0x8F, YBot: 0xAF},
		{X: 0x58, YTop: 0x2F, YBot: 0xAF},
		{X: 0x16, YTop: 0x6F, YBot: 0x8F},
	},
	// Layout 2: full-height center ladder with asymmetric side ladders
	{
		{X: 0x04, YTop: 0x0F, YBot: 0x6F},
		{X: 0x14, YTop: 0x4F, YBot: 0x6F},
		{X: 0x24, YTop: 0x0F, YBot: 0x2F},
		{X: 0x28, YTop: 0x4F, YBot: 0x8F},
		{X: 0x40, YTop: 0x0F, YBot: 0xAF},
		{X: 0x58, YTop: 0x2F, YBot: 0x8F},
		{X: 0x60, YTop: 0x0F, YBot: 0x2F},
		{X: 0x82, YTop: 0x4F, YBot: 0xAF},
		{X: 0x04, YTop: 0x8F, YBot: 0xAF},
	},
	// Layout 3: sparse layout with full-height outer ladders
	{
		{X: 0x04, YTop: 0x0F, YBot: 0xAF},
		{X: 0x20, YTop: 0x0F, YBot: 0x4F},
		{X: 0x20, YTop: 0x6F, YBot: 0x8F},
		{X: 0x35, YTop: 0x8F, YBot: 0xAF},
		{X: 0x40, YTop: 0x0F, YBot: 0x2F},
		{X: 0x52, YTop: 0x4F, YBot: 0xAF},
		{X: 0x76, YTop: 0x0F, YBot: 0xAF},
	},
}

// SpawnPos holds an initial monster (X, Y) in logical coordinates.
type SpawnPos struct {
	X int
	Y int
}

// SpawnLayouts defines the 4 rotating sets of initial monster spawn positions (up to 8 slots each).
var SpawnLayouts = [4][8]SpawnPos{
	// Spawn set 0
	{
		{0x10, 0x46}, {0x40, 0x86}, {0x80, 0x66}, {0x30, 0x06},
		{0x50, 0x46}, {0x20, 0x86}, {0x70, 0x26}, {0x50, 0x26},
	},
	// Spawn set 1
	{
		{0x40, 0x26}, {0x70, 0x86}, {0x10, 0x66}, {0x30, 0x06},
		{0x50, 0x46}, {0x20, 0x86}, {0x70, 0x26}, {0x50, 0x26},
	},
	// Spawn set 2
	{
		{0x70, 0x46}, {0x20, 0x86}, {0x50, 0x66}, {0x40, 0x06},
		{0x30, 0x46}, {0x60, 0x86}, {0x30, 0x26}, {0x50, 0x06},
	},
	// Spawn set 3
	{
		{0x10, 0x46}, {0x40, 0x86}, {0x80, 0x66}, {0x30, 0x06},
		{0x50, 0x46}, {0x20, 0x86}, {0x70, 0x26}, {0x50, 0x26},
	},
}
