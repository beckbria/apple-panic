package assets

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	// BaseWidth and BaseHeight define the canonical logical screen space
	// matching the Apple II 280x192 Hi-Res coordinate system.
	BaseWidth  = 280
	BaseHeight = 192
)

// Animation describes a sprite animation with arbitrary resolution, frame count,
// and playback framerate so classic 2-frame sprites can be replaced seamlessly
// with high-resolution, high-framerate sprite sheets.
type Animation struct {
	Frames        []*ebiten.Image
	FPS           float64 // Playback rate in frames/second when time-driven
	LogicalWidth  float64 // Target draw width in 280x192 base units
	LogicalHeight float64 // Target draw height in 280x192 base units
}

// FrameAt returns the animation frame for elapsed seconds t.
func (a *Animation) FrameAt(t float64) *ebiten.Image {
	if a == nil || len(a.Frames) == 0 {
		return nil
	}
	if len(a.Frames) == 1 || a.FPS <= 0 {
		return a.Frames[0]
	}
	idx := int(t*a.FPS) % len(a.Frames)
	if idx < 0 {
		idx += len(a.Frames)
	}
	return a.Frames[idx]
}

// FrameByStep returns frame step % len(Frames) for step-synchronized classic animations,
// or time-driven FrameAt(t) if the animation has >2 frames (HD high-FPS pack).
func (a *Animation) FrameByStep(step int, t float64) *ebiten.Image {
	if a == nil || len(a.Frames) == 0 {
		return nil
	}
	if len(a.Frames) > 2 && a.FPS > 0 {
		return a.FrameAt(t)
	}
	idx := step % len(a.Frames)
	if idx < 0 {
		idx += len(a.Frames)
	}
	return a.Frames[idx]
}

// TitleArcStep represents one keyframe of the tumbling apple on the title screen.
type TitleArcStep struct {
	X        float64 // Base 280-px X coordinate
	Y        float64 // Base 192-px Y coordinate
	FrameIdx int     // Index into TitleTumblingApple frames
}

// Pack holds all visual assets needed to render the game at ScaleFactor * (280x192).
type Pack struct {
	ScaleFactor float64 // Pixels per base unit (2.0 = 560x384 classic NTSC; 4.0 = 1120x768 HD, etc.)

	Animations map[string]*Animation

	// Environment & HUD textures
	BrickGirder   *ebiten.Image   // 280 x 6 logical
	GroundFloor   *ebiten.Image   // 280 x 2 logical
	LadderRail2   *ebiten.Image   // 14 x 2 logical (2-row upright rails)
	LadderRung2   *ebiten.Image   // 14 x 2 logical (2-row horizontal rung + rails)
	HoleStages    []*ebiten.Image // Stage 1..N progressive hole cutouts (14 x 6 logical)
	HUDBanner     *ebiten.Image   // 280 x 8 logical static bottom labels (SC-, BONUS-, HI-)
	LifeIcon      *ebiten.Image   // 7 x 8 logical stick-figure reserve life icon
	Digits        [11]*ebiten.Image // 0..9 + 10=blank (7 x 8 logical)

	// Title screen choreography assets
	TitleBackdrop      *ebiten.Image
	TitleAppleLogo     *ebiten.Image
	TitleSparkle       *ebiten.Image
	TitleMan           *ebiten.Image
	TitlePanicLetters  [5]*ebiten.Image
	TitleTumblingApple []*ebiten.Image
	TitleArc           []TitleArcStep
}

// CanvasSize returns the pixel dimensions of the render target for this Pack.
func (p *Pack) CanvasSize() (int, int) {
	sf := p.ScaleFactor
	if sf <= 0 {
		sf = 2.0
	}
	return int(BaseWidth * sf), int(BaseHeight * sf)
}

// DrawLogical draws img into dst scaled to (logicalW, logicalH) at (logicalX, logicalY)
// in the 280x192 base coordinate space.
func (p *Pack) DrawLogical(dst, img *ebiten.Image, logicalX, logicalY, logicalW, logicalH float64) {
	if img == nil {
		return
	}
	b := img.Bounds()
	iw, ih := float64(b.Dx()), float64(b.Dy())
	if iw == 0 || ih == 0 {
		return
	}
	sf := p.ScaleFactor
	if sf <= 0 {
		sf = 2.0
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale((logicalW*sf)/iw, (logicalH*sf)/ih)
	op.GeoM.Translate(logicalX*sf, logicalY*sf)
	dst.DrawImage(img, &op)
}

// DrawAnimation renders the active frame of animKey at (logicalX, logicalY).
func (p *Pack) DrawAnimation(dst *ebiten.Image, animKey string, step int, elapsedSec, logicalX, logicalY float64) {
	anim, ok := p.Animations[animKey]
	if !ok || anim == nil {
		return
	}
	frame := anim.FrameByStep(step, elapsedSec)
	p.DrawLogical(dst, frame, logicalX, logicalY, anim.LogicalWidth, anim.LogicalHeight)
}

// ExternalManifest describes an optional JSON manifest for replacing classic assets
// with HD / high-FPS PNG sprite strips in a directory.
type ExternalManifest struct {
	ScaleFactor float64                         `json:"scale_factor"`
	Animations  map[string]ExternalAnimSpec     `json:"animations"`
	Sprites     map[string]string               `json:"sprites"`
	HoleStages  []string                        `json:"hole_stages"`
}

// ExternalAnimSpec describes a horizontal PNG sprite strip override.
type ExternalAnimSpec struct {
	File          string  `json:"file"`
	FrameCount    int     `json:"frame_count"`
	FPS           float64 `json:"fps"`
	LogicalWidth  float64 `json:"logical_width"`
	LogicalHeight float64 `json:"logical_height"`
}

// ApplyExternalOverrides overlays any custom HD PNG sprites/animations defined in
// filepath.Join(dir, "manifest.json") onto an existing Pack.
func (p *Pack) ApplyExternalOverrides(dir string) error {
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var mf ExternalManifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return fmt.Errorf("parse %s: %w", manifestPath, err)
	}
	if mf.ScaleFactor > 0 {
		p.ScaleFactor = mf.ScaleFactor
	}
	loadImg := func(rel string) (*ebiten.Image, error) {
		f, err := os.Open(filepath.Join(dir, rel))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		im, _, err := image.Decode(f)
		if err != nil {
			return nil, err
		}
		return ebiten.NewImageFromImage(im), nil
	}

	for key, spec := range mf.Animations {
		sheet, err := loadImg(spec.File)
		if err != nil {
			return fmt.Errorf("animation %s: %w", key, err)
		}
		count := spec.FrameCount
		if count < 1 {
			count = 1
		}
		sb := sheet.Bounds()
		fw := sb.Dx() / count
		fh := sb.Dy()
		frames := make([]*ebiten.Image, count)
		for i := 0; i < count; i++ {
			sub := sheet.SubImage(image.Rect(i*fw, 0, (i+1)*fw, fh)).(*ebiten.Image)
			frames[i] = sub
		}
		lw, lh := spec.LogicalWidth, spec.LogicalHeight
		if orig, ok := p.Animations[key]; ok && orig != nil {
			if lw <= 0 {
				lw = orig.LogicalWidth
			}
			if lh <= 0 {
				lh = orig.LogicalHeight
			}
		}
		p.Animations[key] = &Animation{
			Frames:        frames,
			FPS:           spec.FPS,
			LogicalWidth:  lw,
			LogicalHeight: lh,
		}
	}

	if path, ok := mf.Sprites["brick_girder"]; ok {
		if im, err := loadImg(path); err == nil {
			p.BrickGirder = im
		}
	}
	if path, ok := mf.Sprites["ground_floor"]; ok {
		if im, err := loadImg(path); err == nil {
			p.GroundFloor = im
		}
	}
	if path, ok := mf.Sprites["ladder_rail"]; ok {
		if im, err := loadImg(path); err == nil {
			p.LadderRail2 = im
		}
	}
	if path, ok := mf.Sprites["ladder_rung"]; ok {
		if im, err := loadImg(path); err == nil {
			p.LadderRung2 = im
		}
	}
	if path, ok := mf.Sprites["life_icon"]; ok {
		if im, err := loadImg(path); err == nil {
			p.LifeIcon = im
		}
	}
	if path, ok := mf.Sprites["title_backdrop"]; ok {
		if im, err := loadImg(path); err == nil {
			p.TitleBackdrop = im
		}
	}

	if len(mf.HoleStages) > 0 {
		stages := make([]*ebiten.Image, 0, len(mf.HoleStages))
		for _, rel := range mf.HoleStages {
			im, err := loadImg(rel)
			if err != nil {
				return fmt.Errorf("hole stage %s: %w", rel, err)
			}
			stages = append(stages, im)
		}
		p.HoleStages = stages
	}
	return nil
}
