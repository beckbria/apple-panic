package assets

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed title_bin.gz
var embeddedTitleBinGZ []byte

// LoadTitleBinary decompresses the embedded classic asset binary payload.
func LoadTitleBinary() ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(embeddedTitleBinGZ))
	if err != nil {
		return nil, fmt.Errorf("gunzip embedded title.bin: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Apple II NTSC Hi-Res artifact colors.
var (
	colPurple = color.NRGBA{R: 0xFF, G: 0x44, B: 0xFD, A: 0xFF}
	colGreen  = color.NRGBA{R: 0x14, G: 0xF5, B: 0x3C, A: 0xFF}
	colBlue   = color.NRGBA{R: 0x14, G: 0xCF, B: 0xF5, A: 0xFF}
	colOrange = color.NRGBA{R: 0xFF, G: 0x6A, B: 0x3C, A: 0xFF}
	colWhite  = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colBlack  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
)

func hgrLineOffset(y int) int {
	return ((y & 7) << 10) | (((y >> 3) & 7) << 7) | ((y >> 6) * 40)
}

// renderHGRRegion decodes a rectangular region [x0..x0+w) x [y0..y0+h) (in 280x192
// base pixels) from an 8KB Apple II Hi-Res buffer into a 2x-scaled (2w x 2h) image.
// If transparentBlack is true, unlit pixels become transparent instead of opaque black.
func renderHGRRegion(hgr []byte, x0, y0, w, h int, transparentBlack bool) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w*2, h*2))
	var bits [280]byte
	var hibits [280]byte

	for row := 0; row < h; row++ {
		y := y0 + row
		if y < 0 || y >= 192 {
			continue
		}
		base := hgrLineOffset(y)
		for bx := 0; bx < 40; bx++ {
			v := hgr[base+bx]
			hb := (v >> 7) & 1
			for bit := 0; bit < 7; bit++ {
				px := bx*7 + bit
				bits[px] = (v >> bit) & 1
				hibits[px] = hb
			}
		}
		for dx := 0; dx < w; dx++ {
			x := x0 + dx
			if x < 0 || x >= 280 {
				continue
			}
			bCur := bits[x]
			var bPrev, bNext byte
			if x > 0 {
				bPrev = bits[x-1]
			}
			if x < 279 {
				bNext = bits[x+1]
			}
			hb := hibits[x]
			var c color.NRGBA
			lit := false
			if bCur == 1 {
				lit = true
				if bPrev == 1 || bNext == 1 {
					c = colWhite
				} else if hb == 0 {
					if x%2 == 0 {
						c = colPurple
					} else {
						c = colGreen
					}
				} else {
					if x%2 == 0 {
						c = colBlue
					} else {
						c = colOrange
					}
				}
			} else if bPrev == 1 && bNext == 1 && hibits[x-1] == hibits[x+1] {
				lit = true
				if hibits[x-1] == 0 {
					if (x-1)%2 == 0 {
						c = colPurple
					} else {
						c = colGreen
					}
				} else {
					if (x-1)%2 == 0 {
						c = colBlue
					} else {
						c = colOrange
					}
				}
			} else if !transparentBlack {
				c = colBlack
			}
			if lit || !transparentBlack {
				out.SetNRGBA(2*dx, 2*row, c)
				out.SetNRGBA(2*dx+1, 2*row, c)
				out.SetNRGBA(2*dx, 2*row+1, c)
				out.SetNRGBA(2*dx+1, 2*row+1, c)
			}
		}
	}
	return out
}

// addSpriteOutline adds a 1-subpixel opaque black silhouette halo around lit pixels
// so sprites stand out crisply over purple ladders.
func addSpriteOutline(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(b)
	copy(dst.Pix, src.Pix)
	dirs := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if src.NRGBAAt(x, y).A != 0 {
				continue
			}
			for _, d := range dirs {
				nx, ny := x+d[0], y+d[1]
				if nx >= 0 && nx < w && ny >= 0 && ny < h && src.NRGBAAt(nx, ny).A != 0 {
					dst.SetNRGBA(x, y, colBlack)
					break
				}
			}
		}
	}
	return dst
}

// LoadClassicPack builds the default Asset Pack from the embedded classic
// binary, extracting all sprites, environment tiles, HUD glyphs, and
// the animated title sequence.
func LoadClassicPack() (*Pack, error) {
	tb, err := LoadTitleBinary()
	if err != nil {
		return nil, err
	}

	// Relocate embedded binary sections into gameplay memory layout.
	var memGame [65536]byte
	copy(memGame[0x26FD:], tb)
	copy(memGame[0x0400:0x0500], memGame[0x2700:0x2800])
	copy(memGame[0x0800:0x2000], memGame[0x4800:0x6000])

	// Relocate embedded binary sections into title-screen memory layout.
	var memTitle [65536]byte
	copy(memTitle[0x26FD:], tb)
	copy(memTitle[0x0400:0x0500], memTitle[0x2700:0x2800])
	copy(memTitle[0x0800:0x2000], memTitle[0x2800:0x4000])

	p := &Pack{
		ScaleFactor: 2.0,
		Animations:  make(map[string]*Animation),
	}

	// 1. Decode Player Sprites (10 frames, 21x16 base, col=0, shift=0).
	decodePlayerFrame := func(baseAddr int) *ebiten.Image {
		var hgr [0x2000]byte
		for r := 0; r < 16; r++ {
			lo := hgrLineOffset(r)
			for k := 0; k < 3; k++ {
				hgr[lo+k] = memGame[baseAddr+r*3+k]
			}
		}
		im := addSpriteOutline(renderHGRRegion(hgr[:], 0, 0, 21, 16, true))
		return ebiten.NewImageFromImage(im)
	}

	runR0 := decodePlayerFrame(0x6000)
	runR1 := decodePlayerFrame(0x6180)
	runL0 := decodePlayerFrame(0x6300)
	runL1 := decodePlayerFrame(0x6480)
	climb0 := decodePlayerFrame(0x6600)
	climb1 := decodePlayerFrame(0x6780)
	digR0 := decodePlayerFrame(0x6900) // shovel down
	digR1 := decodePlayerFrame(0x6A80) // shovel up
	digL0 := decodePlayerFrame(0x6C00) // shovel down
	digL1 := decodePlayerFrame(0x6D80) // shovel up

	p.Animations["player_idle_right"] = &Animation{Frames: []*ebiten.Image{runR0}, FPS: 0, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_idle_left"] = &Animation{Frames: []*ebiten.Image{runL0}, FPS: 0, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_run_right"] = &Animation{Frames: []*ebiten.Image{runR0, runR1}, FPS: 10, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_run_left"] = &Animation{Frames: []*ebiten.Image{runL0, runL1}, FPS: 10, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_climb"] = &Animation{Frames: []*ebiten.Image{climb0, climb1}, FPS: 8, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_fall"] = &Animation{Frames: []*ebiten.Image{climb0, climb1}, FPS: 12, LogicalWidth: 21, LogicalHeight: 16}
	// Dig/fill frames alternate between shovel-down (frame 0) and shovel-up (frame 1).
	p.Animations["player_dig_right"] = &Animation{Frames: []*ebiten.Image{digR0, digR1}, FPS: 6, LogicalWidth: 21, LogicalHeight: 16}
	p.Animations["player_dig_left"] = &Animation{Frames: []*ebiten.Image{digL0, digL1}, FPS: 6, LogicalWidth: 21, LogicalHeight: 16}

	// 2. Decode Monster Sprites (21x10 base, col=0, shift=0 for authentic NTSC colors).
	decodeMonsterFrame := func(pageAddr int) *ebiten.Image {
		var hgr [0x2000]byte
		for r := 0; r < 10; r++ {
			lo := hgrLineOffset(r)
			for k := 0; k < 3; k++ {
				hgr[lo+k] = memGame[pageAddr+r*3+k]
			}
		}
		im := addSpriteOutline(renderHGRRegion(hgr[:], 0, 0, 21, 10, true))
		return ebiten.NewImageFromImage(im)
	}

	apple0, apple1 := decodeMonsterFrame(0x1000), decodeMonsterFrame(0x1100)
	bug0, bug1 := decodeMonsterFrame(0x1200), decodeMonsterFrame(0x1300)
	mask0, mask1 := decodeMonsterFrame(0x1400), decodeMonsterFrame(0x1500)

	p.Animations["apple_walk"] = &Animation{Frames: []*ebiten.Image{apple0, apple1}, FPS: 7, LogicalWidth: 21, LogicalHeight: 10}
	p.Animations["butterfly_walk"] = &Animation{Frames: []*ebiten.Image{bug0, bug1}, FPS: 7, LogicalWidth: 21, LogicalHeight: 10}
	p.Animations["mask_walk"] = &Animation{Frames: []*ebiten.Image{mask0, mask1}, FPS: 7, LogicalWidth: 21, LogicalHeight: 10}

	// 3. Brick Girder (280x6) & Solid Ground Floor (280x2).
	{
		var hgr [0x2000]byte
		for _, y := range []int{0, 1} {
			lo := hgrLineOffset(y)
			for c := 0; c < 40; c += 2 {
				hgr[lo+c] = 0x2A
				hgr[lo+c+1] = 0x54
			}
		}
		for _, y := range []int{3, 4, 5} {
			lo := hgrLineOffset(y)
			for c := 0; c < 38; c += 2 {
				hgr[lo+c] = 0x2A
				hgr[lo+c+1] = 0x15
			}
			hgr[lo+38] = 0x2A
			hgr[lo+39] = 0x55
		}
		p.BrickGirder = ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 0, 0, 280, 6, false))
	}
	{
		var hgr [0x2000]byte
		for _, y := range []int{0, 1} {
			lo := hgrLineOffset(y)
			for c := 0; c < 40; c += 2 {
				hgr[lo+c] = 0xAA
				hgr[lo+c+1] = 0xD5
			}
		}
		p.GroundFloor = ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 0, 0, 280, 2, false))
	}

	// 4. Ladder Segments (rails and rungs decoded at col=1, shift=1 for true purple NTSC phase).
	decodeLadderPart := func(oraTable int) *ebiten.Image {
		var hgr [0x2000]byte
		const col = 1
		const dd = 1 * 4 // bit-shift 1 matches logical X=4
		for r := 0; r < 2; r++ {
			lo := hgrLineOffset(r)
			for k := 0; k < 3; k++ {
				hgr[lo+col+k] = memGame[oraTable+dd+k]
			}
		}
		// Logical X=4 starts at pixel 2*4 = 8, width 14 pixels.
		im := renderHGRRegion(hgr[:], 8, 0, 14, 2, true)
		return ebiten.NewImageFromImage(im)
	}
	p.LadderRail2 = decodeLadderPart(0x6F40)
	p.LadderRung2 = decodeLadderPart(0x6F80)

	// 5. Progressive Hole Cutout Stages 1..5 (from partial divot to full open pit).
	stageOffsets := []int{0x12, 0x0F, 0x0C, 0x09, 0x00}
	p.HoleStages = make([]*ebiten.Image, len(stageOffsets))
	for idx, stOff := range stageOffsets {
		im := image.NewNRGBA(image.Rect(0, 0, 14*2, 6*2))
		for r := 0; r < 6; r++ {
			b0 := memGame[0x0800+stOff+r*3]
			b1 := memGame[0x0800+stOff+r*3+1]
			for px := 0; px < 14; px++ {
				var maskBit byte
				if px < 7 {
					maskBit = (b0 >> px) & 1
				} else {
					maskBit = (b1 >> (px - 7)) & 1
				}
				if maskBit == 0 {
					im.SetNRGBA(2*px, 2*r, colBlack)
					im.SetNRGBA(2*px+1, 2*r, colBlack)
					im.SetNRGBA(2*px, 2*r+1, colBlack)
					im.SetNRGBA(2*px+1, 2*r+1, colBlack)
				}
			}
		}
		p.HoleStages[idx] = ebiten.NewImageFromImage(im)
	}

	// 6. HUD Static Banner (8 rows * 40 bytes) & 7x8 HUD Glyphs (digits 0..9, blank, life icon).
	{
		var hgr [0x2000]byte
		for r := 0; r < 8; r++ {
			lo := hgrLineOffset(r)
			for c := 0; c < 40; c++ {
				hgr[lo+c] = memGame[0x0A00+r*0x40+c]
			}
		}
		p.HUDBanner = ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 0, 0, 280, 8, false))
	}
	decodeGlyph := func(glyphIdx int) *ebiten.Image {
		// Render crisp white/green-tinted classic 7x8 glyph at 2x scale (14x16).
		var hgr [0x2000]byte
		for r := 0; r < 8; r++ {
			hgr[hgrLineOffset(r)] = memGame[0x7003+glyphIdx*8+r]
		}
		return ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 0, 0, 7, 8, false))
	}
	for g := 0; g <= 10; g++ {
		p.Digits[g] = decodeGlyph(g)
	}
	p.LifeIcon = decodeGlyph(11)

	// 7. Extract Title Screen Backdrop & Choreography Assets from memTitle!
	extractTitleAssets(p, &memTitle)

	return p, nil
}

func extractTitleAssets(p *Pack, memTitle *[65536]byte) {
	// Unpack the compressed Hi-Res title backdrop into hgrTitle.
	var hgrTitle [0x2000]byte
	src := 0x4070
	destIdx := 0x4600
	dest := int(memTitle[destIdx]) | (int(memTitle[destIdx+1]) << 8)
	for src < 0x4600 {
		b := memTitle[src]
		if b == 0xFE {
			break
		} else if b == 0xFF {
			src++
			destIdx += 2
			dest = int(memTitle[destIdx]) | (int(memTitle[destIdx+1]) << 8)
		} else {
			if dest >= 0x2000 && dest < 0x4000 {
				hgrTitle[dest-0x2000] = b
			}
			src++
			dest++
		}
	}
	// Clear the top header region (rows 0..63) so the publisher logo is omitted.
	for y := 0; y < 64; y++ {
		off := hgrLineOffset(y)
		clear(hgrTitle[off : off+40])
	}
	p.TitleBackdrop = ebiten.NewImageFromImage(renderHGRRegion(hgrTitle[:], 0, 0, 280, 192, false))

	// Decode a title sprite onto a clean HGR buffer at exact X alignment
	// so NTSC artifact colors (especially rainbow P-A-N-I-C and apple) are preserved.
	decodeTitleSpriteAt := func(x, sid int) (*ebiten.Image, int, int) {
		ptr := int(memTitle[0x1E00+2*sid]) | (int(memTitle[0x1E01+2*sid]) << 8)
		h := int(memTitle[ptr])
		wBytes := int(memTitle[ptr+1])
		tx := 0x1A00 + 2*x
		byteX := int(memTitle[tx])
		shift := int(memTitle[tx+1])

		buf := make([]byte, h*wBytes)
		copy(buf, memTitle[ptr+2:ptr+2+h*wBytes])
		for s := 0; s < shift; s++ {
			var carry byte
			for idx := 0; idx < len(buf); idx++ {
				b := buf[idx]
				hb := b & 0x80
				var newCarry byte
				if b&0x40 != 0 {
					newCarry = 1
				}
				buf[idx] = (((b & 0x3F) << 1) | carry) | hb
				carry = newCarry
			}
		}

		var hgr [0x2000]byte
		idx := 0
		for row := 0; row < h; row++ {
			lo := hgrLineOffset(row)
			for col := 0; col < wBytes; col++ {
				if byteX+col < 40 {
					hgr[lo+byteX+col] ^= buf[idx]
				}
				idx++
			}
		}
		x0 := byteX * 7
		wPx := wBytes * 7
		if x0+wPx > 280 {
			wPx = 280 - x0
		}
		im := renderHGRRegion(hgr[:], x0, 0, wPx, h, true)
		return ebiten.NewImageFromImage(im), x0, h
	}

	// Build both un-accented ("addie" mid-blink) and complete ("apple" after 23 XOR blinks)
	// logos at exact Apple II bit-shifts so 'p','p' descenders and 'l' ascender align pixel-perfectly!
	{
		var hgr [0x2000]byte
		xorSpr := func(y0, x, sid int) {
			ptr := int(memTitle[0x1E00+2*sid]) | (int(memTitle[0x1E01+2*sid]) << 8)
			h := int(memTitle[ptr])
			wBytes := int(memTitle[ptr+1])
			tx := 0x1A00 + 2*x
			byteX := int(memTitle[tx])
			shift := int(memTitle[tx+1])
			buf := make([]byte, h*wBytes)
			copy(buf, memTitle[ptr+2:ptr+2+h*wBytes])
			for s := 0; s < shift; s++ {
				var carry byte
				for idx := 0; idx < len(buf); idx++ {
					b := buf[idx]
					hb := b & 0x80
					var nc byte
					if b&0x40 != 0 {
						nc = 1
					}
					buf[idx] = (((b & 0x3F) << 1) | carry) | hb
					carry = nc
				}
			}
			idx := 0
			for row := 0; row < h; row++ {
				lo := hgrLineOffset(y0 + row)
				for col := 0; col < wBytes; col++ {
					if byteX+col < 40 {
						hgr[lo+byteX+col] ^= buf[idx]
					}
					idx++
				}
			}
		}
		xorSpr(0x46, 0x14, 0)
		p.TitleSparkle = ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 14, 0x41, 105, 26, true))
		xorSpr(0x41, 0x4D, 1)
		xorSpr(0x55, 0x27, 1)
		xorSpr(0x55, 0x3A, 1)
		p.TitleAppleLogo = ebiten.NewImageFromImage(renderHGRRegion(hgr[:], 14, 0x41, 105, 26, true))
	}
	p.TitleMan, _, _ = decodeTitleSpriteAt(0xE6, 7)

	panicXs := [5]int{0x00, 0x1E, 0x3C, 0x5A, 0x6A}
	for i := 0; i < 5; i++ {
		p.TitlePanicLetters[i], _, _ = decodeTitleSpriteAt(panicXs[i], 2+i)
	}

	// Sprites #8..#17 are the 10 tumbling apple rotation frames.
	p.TitleTumblingApple = make([]*ebiten.Image, 10)
	for i := 0; i < 10; i++ {
		p.TitleTumblingApple[i], _, _ = decodeTitleSpriteAt(0xE6, 8+i)
	}

	// Decode the tumbling apple's parabolic bounce trajectory table.
	// Each 3-byte entry (Y, X, sid) draws frame sid, pauses, then advances to frame sid+5.
	for addr := 0x1367; addr < 0x14BF && memTitle[addr] != 0; addr += 3 {
		y := float64(memTitle[addr])
		x := float64(memTitle[addr+1])
		sid := int(memTitle[addr+2]) // 8..12
		if sid >= 8 && sid <= 12 {
			p.TitleArc = append(p.TitleArc,
				TitleArcStep{X: x, Y: y, FrameIdx: sid - 8},
				TitleArcStep{X: x, Y: y, FrameIdx: (sid + 5) - 8},
			)
		}
	}
}
