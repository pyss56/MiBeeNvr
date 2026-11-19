package app

import (
	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/camera"
	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/model"
	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/recorder"
	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/rtsp"
	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/streamhub"
)

// rtspStreamProvider adapts the camera manager to the RTSP output server's
// StreamProvider (#522): resolves the camera's CURRENT recorder — restarted
// recorders own a fresh hub, and the RTSP server re-resolves on every
// DESCRIBE/SETUP — and reads its codec parameters. Mirrors the api package's
// getCodecParams/getStreamHub (ONVIF delegates unwrap to the concrete recorder
// that owns the params; the outer recorder still carries the live hub).
func rtspStreamProvider(camMgr *camera.CameraManager) rtsp.StreamProvider {
	return func(cameraID string) (rtsp.StreamInfo, bool) {
		rec := camMgr.GetRecorder(cameraID)
		if rec == nil {
			return rtsp.StreamInfo{}, false
		}
		var info rtsp.StreamInfo
		if provider, ok := unwrapRTSPDelegate(rec).(model.HLSProvider); ok {
			info.Codec, info.SPS, info.PPS, info.VPS = provider.CodecParams()
		} else {
			// MJPEG-family recorders carry no parameter sets — every JPEG
			// frame is standalone (#658).
			switch unwrapRTSPDelegate(rec).(type) {
			case *recorder.MJPEGRecorder, *recorder.HTTPJPEGRecorder:
				info.Codec = model.FormatMJPEG
			}
		}
		if h, ok := rec.(interface{ GetHub() *streamhub.StreamHub }); ok {
			info.Hub = h.GetHub()
		}
		// Optional G.711 audio track. Only G.711 is emitted over RTSP today;
		// Opus has no track here (gortsplib is 48 kHz-only, Xiaomi sends 16 kHz).
		if g711, muLaw := rtspG711Track(unwrapRTSPDelegate(rec)); g711 {
			info.AudioCodec = model.AudioG711
			info.AudioMULaw = muLaw
		}
		if info.Codec == "" || info.Hub == nil {
			return rtsp.StreamInfo{}, false
		}
		return info, true
	}
}

// rtspG711Track reports whether rec's live audio can be emitted verbatim as an
// RFC 3551 PCMU/PCMA track, and which law it uses.
//
// Recorders differ in how they expose the law, so both conventions are
// honoured: a dedicated AudioG711 probe (Xiaomi — its AudioConfig is nil) wins,
// otherwise the muxer config's first byte is used (1 = μ-law, 0 = A-law), the
// same convention the WS/WebRTC handlers rely on. Anything that is not 8 kHz
// G.711 is rejected: the SDP declares 8000 Hz, so a 16 kHz source would be
// played back at the wrong speed.
//
// rec is deliberately an `any` (the caller passes a model.Recorder) so the
// capability probes below can be unit-tested with minimal fakes.
func rtspG711Track(rec any) (g711, muLaw bool) {
	ap, ok := rec.(interface{ AudioCodec() string })
	if !ok || ap.AudioCodec() != string(model.AudioG711) {
		return false, false
	}
	if rp, ok := rec.(interface{ AudioSampleRate() int }); ok {
		if r := rp.AudioSampleRate(); r != 0 && r != 8000 {
			return false, false
		}
	}
	if gp, ok := rec.(interface{ AudioG711() (bool, bool) }); ok {
		ml, isG711 := gp.AudioG711()
		return isG711, ml
	}
	if cp, ok := rec.(interface{ AudioConfig() []byte }); ok {
		if cfg := cp.AudioConfig(); len(cfg) > 0 {
			return true, cfg[0] == 1
		}
	}
	return true, false
}

// unwrapRTSPDelegate unwraps delegate layers (e.g. ONVIF → H264/H265) to the
// recorder that implements model.HLSProvider. Same shape as the api package's
// unwrapDelegate, kept local to avoid an app→api dependency.
func unwrapRTSPDelegate(rec model.Recorder) model.Recorder {
	for {
		u, ok := rec.(interface{ Delegate() model.Recorder })
		if !ok {
			return rec
		}
		if d := u.Delegate(); d != nil {
			rec = d
			continue
		}
		return rec
	}
}
