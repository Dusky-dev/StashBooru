package config

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func configWithFlags(t *testing.T, args ...string) *Config {
	t.Helper()
	previous := pflag.CommandLine
	pflag.CommandLine = pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	pflag.StringP("ui-location", "u", "", "path to the webui")
	pflag.Int("port", 9999, "port to serve from")
	t.Cleanup(func() { pflag.CommandLine = previous })
	require.NoError(t, pflag.CommandLine.Parse(args))
	return InitializeEmpty()
}

func TestUILocationCommandLine(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		saved  string
		legacy string
		want   string
	}{
		{name: "long flag", args: []string{"--ui-location", "/custom UI/build"}, saved: "/saved", want: "/custom UI/build"},
		{name: "short flag", args: []string{"-u", "/short/build"}, legacy: "/legacy", want: "/short/build"},
		{name: "unchanged flag preserves config", saved: "/saved", want: "/saved"},
		{name: "unchanged flag preserves legacy", legacy: "/legacy", want: "/legacy"},
		{name: "explicit empty disables configured UI", args: []string{"--ui-location="}, saved: "/saved", want: ""},
		{name: "embedded default", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := configWithFlags(t, tc.args...)
			cfg.SetInterface(UILocation, tc.saved)
			cfg.SetInterface(LegacyCustomUILocation, tc.legacy)
			cfg.loadFromCommandLine()
			require.Equal(t, tc.want, cfg.GetUILocation())
			require.Equal(t, tc.saved, cfg.main.String(UILocation), "flags must not rewrite saved configuration")
			require.False(t, cfg.overrides.Exists("ui-location"), "do not retain an unread hyphenated key")
		})
	}
}

func TestUILocationKeepsEnvironmentAndOtherFlagPrecedence(t *testing.T) {
	cfg := configWithFlags(t, "--ui-location", "/command-line", "--port", "12345")
	cfg.SetInterface(UILocation, "/saved")
	t.Setenv("STASH_UI", "/environment")
	cfg.initOverrides()
	require.Equal(t, "/environment", cfg.GetUILocation(), "retain existing environment-over-command-line precedence")
	require.Equal(t, 12345, cfg.GetPort())
}
