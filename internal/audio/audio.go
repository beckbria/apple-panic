package audio

import (
	"bytes"
	"math"
	"os"
	"path/filepath"

	ebitaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const SampleRate = 44100

// Effect identifies a game sound effect.
type Effect string

const (
	SfxFootstepA     Effect = "footstep_a"
	SfxFootstepB     Effect = "footstep_b"
	SfxDigScrape     Effect = "dig_scrape"
	SfxMonsterFall   Effect = "monster_fall"
	SfxPointAward    Effect = "point_award"
	SfxSurviveSiren  Effect = "survive_siren"
	SfxPlayerDeath   Effect = "player_death"
	SfxLevelStart    Effect = "level_start"
	SfxExtraLife     Effect = "extra_life"
	SfxTitleWhistle  Effect = "title_whistle"
	SfxTitleBonk     Effect = "title_bonk"
	SfxMenuMove      Effect = "menu_move"
	SfxMenuSelect    Effect = "menu_select"
)

// Bank manages synthesized or custom WAV sound effects.
type Bank struct {
	ctx     *ebitaudio.Context
	pcmData map[Effect][]byte
	Muted   bool
}

// NewBank creates an audio Bank with synthesized Apple II 1-bit speaker waveforms.
func NewBank(enableContext bool) *Bank {
	b := &Bank{
		pcmData: make(map[Effect][]byte),
	}
	if enableContext {
		if existing := ebitaudio.CurrentContext(); existing != nil {
			b.ctx = existing
		} else {
			b.ctx = ebitaudio.NewContext(SampleRate)
		}
	}
	b.synthesizeDefaults()
	return b
}

// Play triggers a non-blocking playback of sfx unless muted, headless, or
// the audio context is not yet ready (e.g. suspended by browser autoplay policy).
func (b *Bank) Play(sfx Effect) {
	if b == nil || b.Muted || b.ctx == nil || !b.ctx.IsReady() {
		return
	}
	pcm, ok := b.pcmData[sfx]
	if !ok || len(pcm) == 0 {
		return
	}
	p := b.ctx.NewPlayerFromBytes(pcm)
	p.SetVolume(0.35)
	p.Play()
}

// LoadExternalOverrides replaces any matching "<effect>.wav" files found in dir/sfx.
func (b *Bank) LoadExternalOverrides(dir string) {
	sfxDir := filepath.Join(dir, "sfx")
	for sfx := range b.pcmData {
		wavPath := filepath.Join(sfxDir, string(sfx)+".wav")
		raw, err := os.ReadFile(wavPath)
		if err != nil {
			continue
		}
		stream, err := wav.DecodeWithSampleRate(SampleRate, bytes.NewReader(raw))
		if err != nil {
			continue
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(stream); err == nil && buf.Len() > 0 {
			b.pcmData[sfx] = buf.Bytes()
		}
	}
}

func (b *Bank) synthesizeDefaults() {
	// Synthesize classic Apple II 1-bit speaker square-wave sound effects.
	b.pcmData[SfxFootstepA] = renderTones([]toneSeg{{freq: 920, durSec: 0.012, amp: 0.35}})
	b.pcmData[SfxFootstepB] = renderTones([]toneSeg{{freq: 660, durSec: 0.012, amp: 0.35}})

	// 5-click shovel scrape while digging or filling
	var scrape []toneSeg
	for i := 0; i < 5; i++ {
		scrape = append(scrape,
			toneSeg{freq: 380 + float64(i*45), durSec: 0.008, amp: 0.45},
			toneSeg{freq: 0, durSec: 0.004, amp: 0},
		)
	}
	b.pcmData[SfxDigScrape] = renderTones(scrape)

	// Descending slide as a monster falls through a hole
	var fall []toneSeg
	for i := 0; i < 16; i++ {
		fall = append(fall, toneSeg{freq: 1100 - float64(i)*42, durSec: 0.014, amp: 0.40})
	}
	b.pcmData[SfxMonsterFall] = renderTones(fall)

	// Up/down warble when points are awarded
	b.pcmData[SfxPointAward] = renderWarble(1)

	// Descending multi-warble when the player loses a life
	var death []byte
	for i := 0; i < 4; i++ {
		death = append(death, renderSweep(900-float64(i)*120, 220, 0.11, 0.45)...)
		death = append(death, renderSweep(220, 720-float64(i)*100, 0.09, 0.45)...)
	}
	b.pcmData[SfxPlayerDeath] = death

	// Angry dual-tone siren when a Butterfly or Mask survives a fall
	var siren []toneSeg
	for i := 0; i < 4; i++ {
		siren = append(siren,
			toneSeg{freq: 780, durSec: 0.075, amp: 0.45},
			toneSeg{freq: 1180, durSec: 0.075, amp: 0.45},
		)
	}
	b.pcmData[SfxSurviveSiren] = renderTones(siren)

	// Level start ready fanfare
	b.pcmData[SfxLevelStart] = renderTones([]toneSeg{
		{freq: 523.25, durSec: 0.08, amp: 0.4},
		{freq: 659.25, durSec: 0.08, amp: 0.4},
		{freq: 783.99, durSec: 0.12, amp: 0.45},
	})

	// Extra life fanfare at 4,000 points
	b.pcmData[SfxExtraLife] = renderTones([]toneSeg{
		{freq: 523.25, durSec: 0.07, amp: 0.45},
		{freq: 659.25, durSec: 0.07, amp: 0.45},
		{freq: 783.99, durSec: 0.07, amp: 0.45},
		{freq: 1046.50, durSec: 0.18, amp: 0.50},
	})

	// Title screen ascending whistle
	b.pcmData[SfxTitleWhistle] = renderSweep(320, 1850, 0.35, 0.40)

	// Title screen bonk warble when the tumbling apple lands
	var bonk []byte
	for i := 0; i < 5; i++ {
		bonk = append(bonk, renderSweep(520, 1400, 0.035, 0.42)...)
		bonk = append(bonk, renderSweep(1400, 480, 0.035, 0.42)...)
	}
	b.pcmData[SfxTitleBonk] = bonk

	b.pcmData[SfxMenuMove] = renderTones([]toneSeg{{freq: 740, durSec: 0.018, amp: 0.30}})
	b.pcmData[SfxMenuSelect] = renderTones([]toneSeg{
		{freq: 660, durSec: 0.03, amp: 0.35},
		{freq: 990, durSec: 0.05, amp: 0.35},
	})
}

type toneSeg struct {
	freq   float64
	durSec float64
	amp    float64
}

func renderTones(segs []toneSeg) []byte {
	var out []byte
	var phase float64
	for _, s := range segs {
		samples := int(s.durSec * SampleRate)
		for i := 0; i < samples; i++ {
			var v float64
			if s.freq > 0 {
				phase += s.freq / SampleRate
				if phase >= 1.0 {
					phase -= math.Floor(phase)
				}
				if phase < 0.5 {
					v = s.amp
				} else {
					v = -s.amp
				}
				// Soften last 32 samples to avoid clicks
				if samples-i < 32 {
					v *= float64(samples-i) / 32.0
				}
			}
			out = appendStereoS16(out, v)
		}
	}
	return out
}

func renderSweep(fStart, fEnd, durSec, amp float64) []byte {
	samples := int(durSec * SampleRate)
	out := make([]byte, 0, samples*4)
	var phase float64
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(samples)
		freq := fStart + (fEnd-fStart)*t
		phase += freq / SampleRate
		if phase >= 1.0 {
			phase -= math.Floor(phase)
		}
		v := amp
		if phase >= 0.5 {
			v = -amp
		}
		if samples-i < 32 {
			v *= float64(samples-i) / 32.0
		}
		out = appendStereoS16(out, v)
	}
	return out
}

func renderWarble(reps int) []byte {
	var out []byte
	for r := 0; r < reps; r++ {
		out = append(out, renderSweep(420, 1320, 0.09, 0.42)...)
		out = append(out, renderSweep(1320, 420, 0.09, 0.42)...)
	}
	return out
}

func appendStereoS16(buf []byte, sample float64) []byte {
	if sample > 1.0 {
		sample = 1.0
	} else if sample < -1.0 {
		sample = -1.0
	}
	iv := int16(sample * 32767)
	lo := byte(iv)
	hi := byte(iv >> 8)
	return append(buf, lo, hi, lo, hi)
}
