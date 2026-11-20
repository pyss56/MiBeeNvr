// SPDX-License-Identifier: MIT
//
// RTP timestamps are the one place where a unit slip is invisible in code review
// but makes every player render the stream ten times too slowly: the SDP says
// H.265/90000, so one second of video MUST advance the timestamp by 90000 ticks
// (9000 used to be produced, and the picture crawled). The audio clock has a
// different hazard: wallclock stamps repeat when the hub delivers two frames in
// the same millisecond, which players report as a duplicated DTS.

package rtsp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const t0 int64 = 1_700_000_000_000_000_000 // unix ns

func TestRTPTimestampRates(t *testing.T) {
	cs := &cameraStream{}

	// The first frame anchors the origin.
	require.Equal(t, uint32(0), cs.rtpTS(t0, 90000), "origin frame must be timestamp 0")

	require.Equal(t, uint32(90000), cs.rtpTS(t0+int64(time.Second), 90000),
		"one second of video is 90000 ticks at the H.26x clock rate")
	require.Equal(t, uint32(135000), cs.rtpTS(t0+int64(1500*time.Millisecond), 90000))
	require.Equal(t, uint32(8000), cs.rtpTS(t0+int64(time.Second), 8000),
		"one second of G.711 is 8000 ticks")
}

func TestRTPTimestampBeforeOriginIsZero(t *testing.T) {
	cs := &cameraStream{}
	require.Equal(t, uint32(0), cs.rtpTS(t0, 90000))
	require.Equal(t, uint32(0), cs.rtpTS(t0-int64(time.Second), 90000),
		"a stamp older than the origin must not underflow into a huge tick value")
}

func TestAudioClockAdvancesByPayload(t *testing.T) {
	cs := &cameraStream{}

	first := cs.audioTS(160) // 20 ms G.711 frame
	second := cs.audioTS(160)
	third := cs.audioTS(161)

	// Stamps mark the first sample of each frame, so each stamp advances by the
	// previous frame's payload length — never by zero, which is what produced
	// the duplicated DTS warnings.
	require.Equal(t, first+160, second, "stamps must be strictly monotonic, never repeated")
	require.Equal(t, second+160, third)
	before, after := cs.audioTS(0), cs.audioTS(0)
	require.Equal(t, before, after, "an empty frame must not move the clock")
}

func TestAudioClockStartsAtOrigin(t *testing.T) {
	cs := &cameraStream{}
	// Video opened the stream two seconds ago (recent enough that the 8 kHz
	// tick count does not wrap).
	cs.rtpTS(time.Now().UnixNano()-int64(2*time.Second), 90000)
	// Audio enabled later: the first frame must start at "now", not at 0,
	// otherwise the track plays behind the picture by the elapsed time.
	start := cs.audioTS(160)
	require.Greater(t, start, uint32(15000), "audio must anchor near the current stream position")
	require.Equal(t, start+160, cs.audioTS(160))
}
