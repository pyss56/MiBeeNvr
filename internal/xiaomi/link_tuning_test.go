package xiaomi

// Coverage for the transport tuning that backs the `xiaomi:` yaml keys:
// LinkOptions → CS2Conn, recorder defaults, and the reconnect-delay policy.
// Hermetic — nothing here dials a camera.

import (
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/MiBeeNvr/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLinkOptionsDefaults(t *testing.T) {
	o := LinkOptions{}.withDefaults()
	require.Equal(t, cs2ConnectTimeout, o.ConnectTimeout)
	require.Equal(t, cs2ReadTimeout, o.ReadTimeout)
	require.Equal(t, defaultIdleTimeout, o.IdleTimeout)

	custom := LinkOptions{
		ConnectTimeout: 20 * time.Second,
		ReadTimeout:    45 * time.Second,
		IdleTimeout:    time.Minute,
	}.withDefaults()
	require.Equal(t, 20*time.Second, custom.ConnectTimeout)
	require.Equal(t, 45*time.Second, custom.ReadTimeout)
	require.Equal(t, time.Minute, custom.IdleTimeout)
}

func TestCS2ReadWaitFallsBackToConstant(t *testing.T) {
	c := &CS2Conn{}
	require.Equal(t, cs2ReadTimeout, c.readWait(),
		"a CS2Conn built by hand (tests) must keep the historical read budget")

	c.readTimeout = 45 * time.Second
	require.Equal(t, 45*time.Second, c.readWait())
}

func TestRecorderTransportDefaults(t *testing.T) {
	r := NewXiaomiRecorder(XiaomiRecorderConfig{CameraID: "c", DID: "1"}, &noopSegmentStore{})

	require.Equal(t, cs2ConnectTimeout, r.cfg.ConnectTimeout)
	require.Equal(t, cs2ReadTimeout, r.cfg.ReadTimeout)
	require.Equal(t, defaultIdleTimeout, r.cfg.IdleTimeout)
	require.Equal(t, connectFailThreshold, r.cfg.RetryAttempts)
	require.Equal(t, time.Duration(0), r.cfg.RetryDelay)
}

func TestNextRetryDelayFixed(t *testing.T) {
	r := NewXiaomiRecorder(XiaomiRecorderConfig{
		CameraID:   "c",
		RetryDelay: 7 * time.Second,
	}, &noopSegmentStore{})

	require.Equal(t, 7*time.Second, r.nextRetryDelay(1), "retry_delay pins the interval")
	require.Equal(t, 7*time.Second, r.nextRetryDelay(50),
		"a pinned interval also wins over the persistent-failure escalation")
}

func TestNextRetryDelayTiered(t *testing.T) {
	r := NewXiaomiRecorder(XiaomiRecorderConfig{CameraID: "c"}, &noopSegmentStore{})

	d := r.nextRetryDelay(1)
	require.GreaterOrEqual(t, d, time.Second)
	require.Less(t, d, 2*time.Second, "attempt 1 stays in the 1s tier (+jitter)")

	d = r.nextRetryDelay(12)
	require.GreaterOrEqual(t, d, 10*time.Second)
	require.Less(t, d, 11*time.Second, "attempt 12 stays in the 10s tier (+jitter)")
}

func TestNextRetryDelayEscalatesAtRetryAttempts(t *testing.T) {
	r := NewXiaomiRecorder(XiaomiRecorderConfig{CameraID: "c", RetryAttempts: 2}, &noopSegmentStore{})
	require.Equal(t, 2, r.retryAttemptThreshold())

	r.connectFailCount = 1
	require.Less(t, r.nextRetryDelay(1), 2*time.Second, "below the threshold the fast tiers apply")

	r.connectFailCount = 2
	d := r.nextRetryDelay(1)
	require.GreaterOrEqual(t, d, time.Minute)
	require.Less(t, d, 70*time.Second, "at the threshold we settle on the ~60s backoff")
}

func TestRetryAttemptThresholdDefaultsToHistoricalValue(t *testing.T) {
	r := &XiaomiRecorder{}
	require.Equal(t, connectFailThreshold, r.retryAttemptThreshold())
}

func TestPluginNewRecorderCarriesLinkTuning(t *testing.T) {
	// NOT parallel: SetCloudConfig writes the package-level linkCfg that
	// concurrently-constructed recorders read.
	SetCloudConfig(config.XiaomiConfig{
		UserID:            "u",
		Token:             "t",
		Region:            "cn",
		ConnectionTimeout: 20 * time.Second,
		ReadTimeout:       45 * time.Second,
		IdleTimeout:       time.Minute,
		RetryAttempts:     2,
		RetryDelay:        5 * time.Second,
	})
	t.Cleanup(func() { SetCloudConfig(config.XiaomiConfig{}) })

	p := &XiaomiPlugin{}
	rec := p.NewRecorder(config.CameraConfig{ID: "cam-x", Protocol: "xiaomi", DID: "1"}, newXiaomiTestStore(t), nil)
	xr, ok := rec.(*XiaomiRecorder)
	require.True(t, ok)

	require.Equal(t, 20*time.Second, xr.cfg.ConnectTimeout)
	require.Equal(t, 45*time.Second, xr.cfg.ReadTimeout)
	require.Equal(t, time.Minute, xr.cfg.IdleTimeout)
	require.Equal(t, 2, xr.cfg.RetryAttempts)
	require.Equal(t, 5*time.Second, xr.cfg.RetryDelay)
}

func TestSetCloudCredentialsKeepsLinkTuning(t *testing.T) {
	// NOT parallel — same package-level state as above.
	SetCloudConfig(config.XiaomiConfig{
		UserID:      "u",
		Token:       "t",
		ReadTimeout: 45 * time.Second,
	})
	t.Cleanup(func() { SetCloudConfig(config.XiaomiConfig{}) })

	// Post sign-in path: credentials refresh, tuning must survive.
	SetCloudCredentials("u2", "t2", "de")

	require.Equal(t, "u2", cloudCfg.UserID)
	require.Equal(t, "t2", cloudCfg.Token)
	require.Equal(t, "de", cloudCfg.Region)
	require.Equal(t, 45*time.Second, linkCfg.ReadTimeout)
}
