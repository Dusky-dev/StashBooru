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

func TestBooruImportSkipsDisabledConversionWithoutFailing(t *testing.T) {
	repository := p07Fixture(t)
	store := mediaconvert.Store{Root: t.TempDir(), Repository: mediaconvert.ModelRepository{Repository: repository}}
	config, err := store.Config()
	require.NoError(t, err)

	file, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	input := mediaconvert.SourceFormat(file)
	require.NotEmpty(t, input)
	config.FormatRulesEnabled = map[string]bool{input: false}
	require.NoError(t, store.Configure(config))
	config, err = store.Config()
	require.NoError(t, err)

	// The file was already scanned and metadata-linked. Disabled format rules
	// must be treated as a successful import without touching any converter worker.
	notice, err := booruDisabledConversionNotice(context.Background(), store, config, 1)
	require.NoError(t, err)
	require.Contains(t, notice, "Conversion skipped")
	require.Contains(t, notice, "Original imported without conversion")
	require.Equal(t, "", config.DefaultOutput(input, false))

	// Re-enabling the rule restores conversion eligibility, with no forced
	// retries and with the saved output mapping unchanged.
	config.FormatRulesEnabled[input] = true
	require.NoError(t, store.Configure(config))
	notice, err = booruDisabledConversionNotice(context.Background(), store, config, 1)
	require.NoError(t, err)
	require.Empty(t, notice)
	require.NotEmpty(t, config.DefaultOutput(input, false))

	// A disabled video format must likewise retain the imported MP4 rather than
	// treating the intentionally disabled conversion as an import failure.
	config.FormatRulesEnabled["mp4"] = false
	require.NoError(t, store.Configure(config))
	require.Empty(t, config.DefaultOutput("mp4", true))
}

func TestAutoJXLEffortResolvesToConcreteEffort(t *testing.T) {
	repository := p07Fixture(t)
	store := mediaconvert.Store{Root: t.TempDir(), Repository: mediaconvert.ModelRepository{Repository: repository}}
	config, err := store.Config()
	require.NoError(t, err)
	file, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	input := mediaconvert.SourceFormat(file)
	config.FormatDefaults[input] = "ajxl"
	config.EncodingDefaults[input] = mediaconvert.EncodingDefaults{Quality: 90, Effort: 3, AutoEffort: true}
	require.NoError(t, store.Configure(config))
	saved, err := store.Config()
	require.NoError(t, err)
	require.True(t, saved.EncodingDefaults[input].AutoEffort)
	want, ok := mediaconvert.AutoJXLEffort(file, mediaconvert.JXLDistanceFromQuality(90), 0)
	require.True(t, ok, "fixture must have a known size and frame count")
	caps := mediaconvert.Capabilities{Formats: []mediaconvert.Format{{ID: "ajxl", Extension: "jxl", Family: "animation", Available: true, Controls: []string{"quality", "effort", "decodingSpeed"}, DecodingSpeedLevels: 4}}}
	target := conversionTarget{Kind: "image", ID: 1}
	options, _, err := resolveConversionOptions(context.Background(), store, saved, 1, target,
		conversionRequest{Options: mediaconvert.Options{Format: "ajxl"}, UseEncodingDefaults: true}, caps)
	require.NoError(t, err)
	require.Equal(t, want, options.Effort)
	require.NotEqual(t, 3, want, "fixture should resolve away from the manual effort")
	require.GreaterOrEqual(t, options.Effort, 1)
	require.LessOrEqual(t, options.Effort, 9)

	// A temporary override applies Auto without saved defaults; without it the
	// requested manual effort is sent unchanged.
	options, _, err = resolveConversionOptions(context.Background(), store, saved, 1, target,
		conversionRequest{Options: mediaconvert.Options{Format: "ajxl", Quality: 90, Effort: 2}, UseQuality: true, AutoEffort: true}, caps)
	require.NoError(t, err)
	require.Equal(t, want, options.Effort)
	options, _, err = resolveConversionOptions(context.Background(), store, saved, 1, target,
		conversionRequest{Options: mediaconvert.Options{Format: "ajxl", Quality: 90, Effort: 2}, UseQuality: true}, caps)
	require.NoError(t, err)
	require.Equal(t, 2, options.Effort)
}

// JPEG XL and PNG are one choice each: the server sends the animated variant
// for an animated source, so older remote workers keep the frames too.
func TestStillChoicesFollowAnimatedSource(t *testing.T) {
	repository := p07Fixture(t)
	store := mediaconvert.Store{Root: t.TempDir(), Repository: mediaconvert.ModelRepository{Repository: repository}}
	config, err := store.Config()
	require.NoError(t, err)
	file, err := store.Repository.Get(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "apng", mediaconvert.SourceFormat(file), "fixture is a three-frame PNG")
	caps := mediaconvert.Capabilities{Formats: []mediaconvert.Format{
		{ID: "ajxl", Extension: "jxl", Family: "animation", Available: true, Controls: []string{"quality", "effort", "decodingSpeed"}, DecodingSpeedLevels: 4},
		{ID: "apng", Extension: "png", Family: "animation", Available: true},
	}}
	target := conversionTarget{Kind: "image", ID: 1}
	for choice, want := range map[string]string{"jxl": "ajxl", "png": "apng", "ajxl": "ajxl"} {
		options, format, err := resolveConversionOptions(context.Background(), store, config, 1, target,
			conversionRequest{Options: mediaconvert.Options{Format: choice}}, caps)
		require.NoError(t, err, choice)
		require.Equal(t, want, options.Format, choice)
		require.Equal(t, want, format.ID, choice)
	}
	// A saved Animated PNG rule from before the merge reads back as PNG and
	// still produces APNG for this source.
	config.FormatDefaults["apng"] = "apng"
	require.NoError(t, store.Configure(config))
	saved, err := store.Config()
	require.NoError(t, err)
	require.Equal(t, "png", saved.FormatDefaults["apng"])
	options, _, err := resolveConversionOptions(context.Background(), store, saved, 1, target,
		conversionRequest{Options: mediaconvert.Options{Format: "auto"}}, caps)
	require.NoError(t, err)
	require.Equal(t, "apng", options.Format)
}
