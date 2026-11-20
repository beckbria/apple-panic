package ui

import (
	_ "embed"

	"github.com/hajimehoshi/ebiten/v2"
)

// monochromeShaderSrc simulates an authentic 1980s Apple II P31 green-phosphor
// monochrome CRT monitor: perceptual luminance conversion with midtone lift,
// subtle horizontal phosphor bloom, 192-line CRT scanlines, gentle corner
// vignette, and a deep-green -> P31-green -> pale-mint highlight color ramp.
//
//go:embed monochrome.kage
var monochromeShaderSrc []byte

func newMonochromeShader() (*ebiten.Shader, error) {
	return ebiten.NewShader(monochromeShaderSrc)
}

func applyMonochromeFilter(dst, src *ebiten.Image, shader *ebiten.Shader) {
	b := dst.Bounds()
	w, h := b.Dx(), b.Dy()
	if shader == nil {
		var op ebiten.DrawImageOptions
		op.ColorScale.Scale(0.2, 1.0, 0.3, 1.0)
		dst.DrawImage(src, &op)
		return
	}
	var op ebiten.DrawRectShaderOptions
	op.Images[0] = src
	dst.DrawRectShader(w, h, shader, &op)
}
