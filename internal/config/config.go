package config

import (
	"encoding/json"

	"github.com/hajimehoshi/ebiten/v2"
)

// Action identifies a bindable gameplay control.
type Action string

const (
	ActionUp    Action = "up"
	ActionDown  Action = "down"
	ActionLeft  Action = "left"
	ActionRight Action = "right"
	ActionDig   Action = "dig"
	ActionFill  Action = "fill"
	ActionPause Action = "pause"
)

// AllActions defines the display order in the Controls customization menu.
var AllActions = []Action{
	ActionUp,
	ActionLeft,
	ActionDown,
	ActionRight,
	ActionDig,
	ActionFill,
	ActionPause,
}

// ActionLabel returns a human-readable label for each bindable action.
func ActionLabel(a Action) string {
	switch a {
	case ActionUp:
		return "Climb Up"
	case ActionDown:
		return "Climb Down"
	case ActionLeft:
		return "Move Left"
	case ActionRight:
		return "Move Right"
	case ActionDig:
		return "Dig Hole"
	case ActionFill:
		return "Fill Hole"
	case ActionPause:
		return "Pause Game"
	default:
		return string(a)
	}
}

// VideoMode selects the display color output mode.
type VideoMode string

const (
	VideoModeColor      VideoMode = "color"
	VideoModeMonochrome VideoMode = "monochrome"
)

// Accessibility holds gameplay tweak settings.
type Accessibility struct {
	// ToggleControls enables latch/toggle movement instead of held keys, with a
	// single-slot direction queue for turning at the next available junction.
	ToggleControls bool `json:"toggle_controls"`

	// KeepKilledMonstersDead prevents monsters already killed in the current
	// level from respawning if the player loses a life (unlike the Apple II
	// original, which completely resets the level with a fresh set of monsters).
	KeepKilledMonstersDead bool `json:"keep_killed_monsters_dead"`

	// PauseWhileMonsterFalls pauses player and other monster movement while any
	// monster is falling through holes, matching the original Apple II game.
	PauseWhileMonsterFalls bool `json:"pause_while_monster_falls"`

	// AutoAlignDigging nudges the player by 1 logical X unit onto an odd X
	// coordinate when initiating a dig/fill if standing on an even X coordinate.
	AutoAlignDigging bool `json:"auto_align_digging"`

	// ShowAlignmentGuides draws subtle vertical alignment markers beneath open
	// holes to assist lining up 2- and 3-hole traps for Butterflies and Masks.
	ShowAlignmentGuides bool `json:"show_alignment_guides"`

	// GameSpeedPercent scales overall simulation speed (e.g. 75, 100, 125, 150).
	GameSpeedPercent int `json:"game_speed_percent"`

	// VideoMode switches between full color ("color") and 1980s green phosphor
	// monochrome monitor simulation ("monochrome").
	VideoMode VideoMode `json:"video_mode"`
}

// Config stores persistent player preferences and high score.
type Config struct {
	Bindings      map[Action]string `json:"bindings"`
	Accessibility Accessibility     `json:"accessibility"`
	HighScore     int               `json:"high_score"`
	Muted         bool              `json:"muted"`
}

// DefaultConfig returns the modern default configuration (WASD + K/J).
func DefaultConfig() Config {
	cfg := Config{
		Bindings: make(map[Action]string),
		Accessibility: Accessibility{
			ToggleControls:         true,
			KeepKilledMonstersDead: false,
			PauseWhileMonsterFalls: true,
			AutoAlignDigging:       true,
			ShowAlignmentGuides:    false,
			GameSpeedPercent:       100,
			VideoMode:              VideoModeColor,
		},
		HighScore: 0,
	}
	cfg.ApplyModernDefaults()
	return cfg
}

// ApplyModernDefaults sets WASD for movement, K/J for dig/fill, and P for pause.
func (c *Config) ApplyModernDefaults() {
	if c.Bindings == nil {
		c.Bindings = make(map[Action]string)
	}
	c.Bindings[ActionUp] = ebiten.KeyW.String()
	c.Bindings[ActionLeft] = ebiten.KeyA.String()
	c.Bindings[ActionDown] = ebiten.KeyS.String()
	c.Bindings[ActionRight] = ebiten.KeyD.String()
	c.Bindings[ActionDig] = ebiten.KeyK.String()
	c.Bindings[ActionFill] = ebiten.KeyJ.String()
	c.Bindings[ActionPause] = ebiten.KeyP.String()
}

// ApplyClassicApplePresets sets the classic Apple II controls (IJKL + A/S).
func (c *Config) ApplyClassicApplePresets() {
	if c.Bindings == nil {
		c.Bindings = make(map[Action]string)
	}
	c.Bindings[ActionUp] = ebiten.KeyI.String()
	c.Bindings[ActionLeft] = ebiten.KeyJ.String()
	c.Bindings[ActionDown] = ebiten.KeyK.String()
	c.Bindings[ActionRight] = ebiten.KeyL.String()
	c.Bindings[ActionDig] = ebiten.KeyA.String()
	c.Bindings[ActionFill] = ebiten.KeyS.String()
	c.Bindings[ActionPause] = ebiten.KeyP.String()
}

// KeyFor returns the ebiten.Key bound to an action.
func (c *Config) KeyFor(a Action) (ebiten.Key, bool) {
	name, ok := c.Bindings[a]
	if !ok {
		return 0, false
	}
	return ParseKey(name)
}

// SetBinding assigns key k to action target, swapping with any conflicting action
// so all controls remain uniquely bound.
func (c *Config) SetBinding(target Action, k ebiten.Key) {
	if c.Bindings == nil {
		c.Bindings = make(map[Action]string)
	}
	newName := k.String()
	oldName := c.Bindings[target]
	for act, existing := range c.Bindings {
		if act != target && existing == newName {
			c.Bindings[act] = oldName
		}
	}
	c.Bindings[target] = newName
}

var allKeys []ebiten.Key

func init() {
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if k.String() != "" {
			allKeys = append(allKeys, k)
		}
	}
}

// ParseKey converts an ebiten.Key.String() name back into an ebiten.Key.
func ParseKey(name string) (ebiten.Key, bool) {
	for _, k := range allKeys {
		if k.String() == name {
			return k, true
		}
	}
	return 0, false
}

// AllBindableKeys returns all keyboard keys eligible for control customization.
func AllBindableKeys() []ebiten.Key {
	return allKeys
}

// DefaultConfigPath returns the user's config file path (or browser storage key on js/wasm).
func DefaultConfigPath() string {
	return defaultConfigPath()
}

// Load reads Config from path, falling back to DefaultConfig if missing or invalid.
func Load(path string) Config {
	cfg := DefaultConfig()
	data, err := readConfigData(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig()
	}
	// Ensure any missing bindings are populated with defaults.
	def := DefaultConfig()
	if cfg.Bindings == nil {
		cfg.Bindings = def.Bindings
	} else {
		for _, a := range AllActions {
			if _, ok := cfg.KeyFor(a); !ok {
				cfg.Bindings[a] = def.Bindings[a]
			}
		}
	}
	if cfg.Accessibility.GameSpeedPercent < 50 || cfg.Accessibility.GameSpeedPercent > 200 {
		cfg.Accessibility.GameSpeedPercent = 100
	}
	if cfg.Accessibility.VideoMode != VideoModeColor && cfg.Accessibility.VideoMode != VideoModeMonochrome {
		cfg.Accessibility.VideoMode = VideoModeColor
	}
	return cfg
}

// Save writes Config to path atomically (or browser localStorage on js/wasm).
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeConfigData(path, data)
}
