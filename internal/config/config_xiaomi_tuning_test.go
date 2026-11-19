package config

// Coverage for the `xiaomi:` transport-tuning keys. They were documented in
// docs/{zh,en}/xiaomi-setup.md for a long time while the struct had only
// user_id/token/region — and because Load() is not a strict decoder, the keys
// were silently dropped instead of rejected.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestXiaomiLinkTuningDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	require.Equal(t, 5*time.Second, cfg.Xiaomi.ConnectionTimeout)
	require.Equal(t, 15*time.Second, cfg.Xiaomi.ReadTimeout)
	require.Equal(t, 30*time.Second, cfg.Xiaomi.IdleTimeout)
	require.Equal(t, 5, cfg.Xiaomi.RetryAttempts)
	require.Equal(t, time.Duration(0), cfg.Xiaomi.RetryDelay,
		"retry_delay 0 must stay 0: it means the tiered 1s/5s/10s/60s schedule")
}

func TestXiaomiLinkTuningLoad(t *testing.T) {
	yaml := `
xiaomi:
  user_id: "123"
  token: "tok"
  region: "de"
  connection_timeout: 20s
  read_timeout: 45s
  idle_timeout: 1m
  retry_attempts: 2
  retry_delay: 5s
`
	path := writeTempYAML(t, yaml)
	cfg, err := Load(path)
	require.NoError(t, err)

	require.Equal(t, "123", cfg.Xiaomi.UserID)
	require.Equal(t, "tok", cfg.Xiaomi.Token)
	require.Equal(t, "de", cfg.Xiaomi.Region)
	require.Equal(t, 20*time.Second, cfg.Xiaomi.ConnectionTimeout)
	require.Equal(t, 45*time.Second, cfg.Xiaomi.ReadTimeout)
	require.Equal(t, time.Minute, cfg.Xiaomi.IdleTimeout)
	require.Equal(t, 2, cfg.Xiaomi.RetryAttempts)
	require.Equal(t, 5*time.Second, cfg.Xiaomi.RetryDelay)
	require.NoError(t, Validate(cfg))
}

func TestXiaomiLinkTuningUntouchedStaysValid(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()
	require.NoError(t, Validate(cfg), "a config without the tuning keys must validate unchanged")
}

func TestXiaomiLinkTuningValidation(t *testing.T) {
	tuned := func(mut func(*XiaomiConfig)) *Config {
		cfg := &Config{}
		cfg.ApplyDefaults()
		mut(&cfg.Xiaomi)
		return cfg
	}

	cases := []struct {
		name    string
		mut     func(*XiaomiConfig)
		wantErr string
	}{
		{"negative connection_timeout", func(x *XiaomiConfig) { x.ConnectionTimeout = -time.Second }, "xiaomi.connection_timeout"},
		{"connection_timeout too long", func(x *XiaomiConfig) { x.ConnectionTimeout = 6 * time.Minute }, "xiaomi.connection_timeout"},
		{"negative read_timeout", func(x *XiaomiConfig) { x.ReadTimeout = -time.Second }, "xiaomi.read_timeout"},
		{"read_timeout too long", func(x *XiaomiConfig) { x.ReadTimeout = time.Hour }, "xiaomi.read_timeout"},
		{"negative idle_timeout", func(x *XiaomiConfig) { x.IdleTimeout = -time.Second }, "xiaomi.idle_timeout"},
		{"idle_timeout too long", func(x *XiaomiConfig) { x.IdleTimeout = 2 * time.Hour }, "xiaomi.idle_timeout"},
		{"negative retry_attempts", func(x *XiaomiConfig) { x.RetryAttempts = -1 }, "xiaomi.retry_attempts"},
		{"retry_attempts too high", func(x *XiaomiConfig) { x.RetryAttempts = 101 }, "xiaomi.retry_attempts"},
		{"negative retry_delay", func(x *XiaomiConfig) { x.RetryDelay = -time.Second }, "xiaomi.retry_delay"},
		{"retry_delay too long", func(x *XiaomiConfig) { x.RetryDelay = 11 * time.Minute }, "xiaomi.retry_delay"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tuned(tc.mut))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
