//go:build integration

package api

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/mediaconvert"
	"github.com/stretchr/testify/require"
)

func TestBooruConverterUsesSavedFormatAndEncodingDefaults(t *testing.T) {
	repository := p07Fixture(t)
	store := mediaconvert.Store{Root: t.TempDir(), Repository: mediaconvert.ModelRepository{Repository: repository}}
	config, err := store.Config()
	require.NoError(t, err)
	config.FormatDefaults["apng"] = "webp"
	config.EncodingDefaults["apng"] = mediaconvert.EncodingDefaults{Quality: 37, Effort: 3, Lossless: true, AllowLarger: true}
	config.Backend = "remote"
	require.NoError(t, store.Configure(config))
	saved, err := store.Config()
	require.NoError(t, err)
	caps := mediaconvert.Capabilities{Formats: []mediaconvert.Format{{ID: "webp", Extension: "webp", Family: "animation", Available: true, Controls: []string{"quality", "effort", "lossless"}}}}
	options, format, err := resolveConversionOptions(context.Background(), store, saved, 1, conversionTarget{Kind: "image", ID: 1}, conversionRequest{Options: mediaconvert.Options{Format: "auto", Hardware: "auto"}, UseEncodingDefaults: true}, caps)
	require.NoError(t, err)
	require.Equal(t, "webp", format.ID)
	require.Equal(t, float64(37), options.Quality)
	require.Equal(t, 3, options.Effort)
	require.True(t, options.Lossless)
	require.True(t, options.AllowLarger)
	require.Equal(t, "auto", options.Hardware)
	require.Equal(t, "remote", saved.Backend)

	// Disabling a saved format rule must stop auto selection without deleting
	// its configured encoder options. An explicit requested output still works.
	file, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	input := mediaconvert.SourceFormat(file)
	saved.FormatRulesEnabled = map[string]bool{input: false}
	require.NoError(t, store.Configure(saved))
	disabled, err := store.Config()
	require.NoError(t, err)
	_, _, err = resolveConversionOptions(context.Background(), store, disabled, 1,
		conversionTarget{Kind: "image", ID: 1},
		conversionRequest{Options: mediaconvert.Options{Format: "auto"}, UseEncodingDefaults: true}, caps)
	require.ErrorContains(t, err, "automatic conversion is disabled")
	options, format, err = resolveConversionOptions(context.Background(), store, disabled, 1,
		conversionTarget{Kind: "image", ID: 1},
		conversionRequest{Options: mediaconvert.Options{Format: "webp"}, UseEncodingDefaults: true}, caps)
	require.NoError(t, err)
	require.Equal(t, "webp", format.ID)
	require.Equal(t, float64(37), options.Quality)
}
