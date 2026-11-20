# Apple Panic (Go + Ebitengine)

A faithful modern implementation of **Apple Panic**, combining the animated Hi-Res title screen with the classic 48-level gameplay, built in Go and [Ebitengine v2](https://ebitengine.org/).

## Building & Running

Desktop version:

```bash
make desktop
./apple-panic
```

Web (WebAssembly) version:

```bash
make web
make serve   # serves ./web on http://localhost:8080
```

Build both desktop and web targets:

```bash
make all
```

Run the test suite:

```bash
make test
```

### Command-Line Flags
- `-assets <dir>` — Optional directory containing `manifest.json` + high-resolution / high-FPS PNG sprite sheets and `sfx/*.wav` overrides.
- `-config <path>` — Custom path for persistent keybindings, accessibility tweaks, and high score JSON.

## Default Controls (Fully Customizable In-Game)

| Action | Default Key | Classic Apple II Preset |
| :--- | :---: | :---: |
| **Climb Up** | `W` | `I` |
| **Move Left** | `A` | `J` |
| **Climb Down** | `S` | `K` |
| **Move Right** | `D` | `L` |
| **Dig Hole** | `K` | `A` |
| **Fill Hole** | `J` | `S` |
| **Pause / Back** | `P` / `Escape` | `P` / `Escape` |

## Accessibility Settings
Accessible from both the **Title Menu** and the in-game **Pause Menu** (with Control Mode also toggleable in **Controls**):
- **Toggle-Based Controls**: Switches from held movement to tap-to-move controls—tapping a direction moves continuously until hitting an obstacle (wall, ladder end, or blocking hole) or receiving another command, with a single-slot turn queue so tapping a perpendicular direction (such as `Right` while climbing a ladder, or `Up` while walking toward a ladder) automatically turns at the next available junction.
- **Keep Killed Monsters Dead**: When enabled, any monsters already defeated in the current level stay dead if you lose a life (instead of resetting the level with all monsters respawning as on the Apple II).
- **Pause While Monsters Fall**: Enabled by default; pauses player movement, other monsters, and timers while a monster is falling through holes (matching the original Apple II game). Disable to keep moving freely while monsters fall.
- **Auto-Align Digging & Filling**: Automatically snaps 1 logical unit onto valid even-hole alignment when pressing Dig or Fill.
- **Hole Alignment Guides**: Displays subtle vertical plumb lines beneath dug holes to assist lining up 2-hole (Butterfly) and 3-hole (Mask) traps.
- **Game Speed**: `75%`, `100%` (authentic Apple II speed), `125%`, or `150%`.
- **Video Mode**: Switches between `Color` (authentic Apple II Hi-Res artifact colors) and `Monochrome` (1980s green-phosphor CRT monitor simulation with subtle scanlines, phosphor bloom, and vignette).

## Replacing Assets with High-Resolution / High-Framerate Packs

Pass `-assets ./my_hd_pack` with a `manifest.json` file to override resolution (`scale_factor`, e.g. `4.0` for `1120×768` or `8.0` for `2240×1536`), sprite sheets (`frame_count` & `fps`), environment tiles, progressive hole stages, and `sfx/*.wav` audio files:

```json
{
  "scale_factor": 4.0,
  "animations": {
    "player_run_right": { "file": "player_run_right.png", "frame_count": 8, "fps": 24, "logical_width": 21, "logical_height": 16 },
    "player_run_left":  { "file": "player_run_left.png",  "frame_count": 8, "fps": 24, "logical_width": 21, "logical_height": 16 },
    "player_climb":     { "file": "player_climb.png",     "frame_count": 8, "fps": 18, "logical_width": 21, "logical_height": 16 },
    "player_dig_right": { "file": "player_dig_right.png", "frame_count": 6, "fps": 15, "logical_width": 21, "logical_height": 16 },
    "player_dig_left":  { "file": "player_dig_left.png",  "frame_count": 6, "fps": 15, "logical_width": 21, "logical_height": 16 },
    "apple_walk":       { "file": "apple_walk.png",       "frame_count": 12, "fps": 24, "logical_width": 21, "logical_height": 10 },
    "butterfly_walk":   { "file": "butterfly_walk.png",   "frame_count": 12, "fps": 30, "logical_width": 21, "logical_height": 10 },
    "mask_walk":        { "file": "mask_walk.png",        "frame_count": 12, "fps": 24, "logical_width": 21, "logical_height": 10 }
  },
  "sprites": {
    "brick_girder": "brick_girder.png",
    "ground_floor": "ground_floor.png",
    "ladder_rail":  "ladder_rail.png",
    "ladder_rung":  "ladder_rung.png",
    "life_icon":    "life_icon.png"
  },
  "hole_stages": [
    "hole_1.png", "hole_2.png", "hole_3.png", "hole_4.png", "hole_5.png"
  ]
}
```
