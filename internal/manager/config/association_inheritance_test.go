package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/require"
)

func TestApplyAssociationInheritanceSettingsPersistsReviewedDefaults(t *testing.T) {
	cfg := InitializeEmpty()
	path := filepath.Join(t.TempDir(), "config.yml")
	cfg.SetConfigFile(path)
	cfg.SetInt(Port, 9999)
	require.NoError(t, cfg.Write())
	require.NoError(t, os.Chmod(path, 0600))
	before := cfg.GetAssociationInheritanceSettings()
	proposed := before
	proposed.Artists = false
	require.NoError(t, cfg.ApplyAssociationInheritanceSettings(before, proposed))
	require.Equal(t, proposed, cfg.GetAssociationInheritanceSettings())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	loaded := &Config{main: koanf.New("."), overrides: koanf.New(".")}
	require.NoError(t, loaded.load(path))
	require.Equal(t, proposed, loaded.GetAssociationInheritanceSettings())
	require.Equal(t, 9999, loaded.GetPort(), "other settings must survive the reviewed save")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())

	require.ErrorContains(t, cfg.ApplyAssociationInheritanceSettings(before, before), "changed since preview")
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, after, "stale applies must preserve the saved configuration")
	require.Equal(t, proposed, cfg.GetAssociationInheritanceSettings())
}

func TestApplyAssociationInheritanceSettingsSaveFailureRestoresDefaults(t *testing.T) {
	cfg := InitializeEmpty()
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "missing", "config.yml"))
	before := cfg.GetAssociationInheritanceSettings()
	proposed := AssociationInheritanceSettings{}
	require.Error(t, cfg.ApplyAssociationInheritanceSettings(before, proposed))
	require.Equal(t, before, cfg.GetAssociationInheritanceSettings())
	for _, key := range []string{AssociationInheritanceCharacters, AssociationInheritanceArtists, AssociationInheritanceCopyrights, AssociationInheritanceTags} {
		require.False(t, cfg.main.Exists(key), "unset defaults must remain unset after a failed save")
	}
}

func TestApplyAssociationInheritanceSettingsRespectsOverrides(t *testing.T) {
	cfg := InitializeEmpty()
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yml"))
	require.NoError(t, cfg.overrides.Set(AssociationInheritanceArtists, true))
	before := cfg.GetAssociationInheritanceSettings()
	proposed := before
	proposed.Artists = false
	require.ErrorContains(t, cfg.ApplyAssociationInheritanceSettings(before, proposed), "overridden")
	require.Equal(t, before, cfg.GetAssociationInheritanceSettings())
}
