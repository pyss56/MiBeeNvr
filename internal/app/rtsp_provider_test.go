// SPDX-License-Identifier: MIT
//
// Tests for the RTSP provider's optional G.711 track probe: the law must come
// from whichever convention the recorder implements, and anything that is not
// verbatim 8 kHz G.711 (Opus, raw PCM, 16 kHz) must be declined so the SDP does
// not promise a track the payload cannot honour.

package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The fakes below implement only a subset of the probes, mirroring recorders
// that expose just some of them; rtspG711Track takes `any`, so no full
// model.Recorder stand-in is needed.

type codecOnly struct{ codec string }

func (c codecOnly) AudioCodec() string { return c.codec }

type codecAndRate struct {
	codec string
	rate  int
}

func (c codecAndRate) AudioCodec() string   { return c.codec }
func (c codecAndRate) AudioSampleRate() int { return c.rate }

type configProbe struct {
	codec string
	rate  int
	cfg   []byte
}

func (c configProbe) AudioCodec() string   { return c.codec }
func (c configProbe) AudioSampleRate() int { return c.rate }
func (c configProbe) AudioConfig() []byte  { return c.cfg }

type g711Probe struct {
	codec string
	mulaw bool
	ok    bool
}

func (g g711Probe) AudioCodec() string      { return g.codec }
func (g g711Probe) AudioG711() (bool, bool) { return g.mulaw, g.ok }

func TestRTSPG711Track(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rec       any
		wantG711  bool
		wantMULaw bool
	}{
		{"no audio probe at all", struct{}{}, false, false},
		{"recorder without audio", codecOnly{}, false, false},
		{"opus is not emitted", codecOnly{codec: "opus"}, false, false},

		// Xiaomi: AudioConfig is nil, the dedicated probe carries the law.
		{"xiaomi pcmu", g711Probe{codec: "g711", mulaw: true, ok: true}, true, true},
		{"xiaomi pcma", g711Probe{codec: "g711", mulaw: false, ok: true}, true, false},
		{"xiaomi raw pcm is declined", g711Probe{codec: "g711", ok: false}, false, false},

		// Generic recorders: law lives in the muxer config's first byte.
		{"config says mu-law", configProbe{codec: "g711", rate: 8000, cfg: []byte{1, 0, 0, 0x1f, 0x40}}, true, true},
		{"config says a-law", configProbe{codec: "g711", rate: 8000, cfg: []byte{0, 0, 0, 0x1f, 0x40}}, true, false},
		{"config with unknown rate is allowed", configProbe{codec: "g711", cfg: []byte{1}}, true, true},
		{"no config defaults to a-law", configProbe{codec: "g711", rate: 8000}, true, false},

		// 16 kHz G.711 would be mislabelled by the 8 kHz SDP declaration.
		{"16 kHz is declined", configProbe{codec: "g711", rate: 16000, cfg: []byte{1}}, false, false},
		{"16 kHz from a bare rate probe", codecAndRate{codec: "g711", rate: 16000}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g711, muLaw := rtspG711Track(tc.rec)
			require.Equal(t, tc.wantG711, g711)
			if tc.wantG711 {
				require.Equal(t, tc.wantMULaw, muLaw)
			}
		})
	}
}
