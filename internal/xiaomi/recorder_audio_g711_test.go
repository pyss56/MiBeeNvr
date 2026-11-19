// SPDX-License-Identifier: MIT
//
// The RTSP output needs the G.711 law, which AudioConfig cannot provide for
// Xiaomi cameras (it is always nil). AudioG711 must therefore map the MISS
// codec id precisely — and must decline raw PCM, which the hub labels "g711"
// for the recording path but which is not PCMU/PCMA on the wire.

package xiaomi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAudioG711ReportsLaw(t *testing.T) {
	for _, tc := range []struct {
		name      string
		codecID   uint32
		wantMULaw bool
		wantOK    bool
	}{
		{"PCMU is mu-law", missCodecPCMU, true, true},
		{"PCMA is a-law", missCodecPCMA, false, true},
		{"raw PCM cannot be carried as G.711", missCodecPCM, false, false},
		{"Opus is not G.711", missCodecOPUS, false, false},
		{"codec not detected yet", 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &XiaomiRecorder{}
			r.audioCodecID.Store(tc.codecID)

			muLaw, ok := r.AudioG711()
			require.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.Equal(t, tc.wantMULaw, muLaw)
			}
		})
	}
}
