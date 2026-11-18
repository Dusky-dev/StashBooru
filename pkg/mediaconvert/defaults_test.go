package mediaconvert

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stashapp/stash/pkg/models"
)

func intPtr(value int) *int { return &value }

func TestFormatDefaultsPersistAcrossUpgrade(t *testing.T) {
	s := Store{Root: t.TempDir()}
	if err := os.WriteFile(filepath.Join(s.Root, "config.json"), []byte(`{"cacheLimitBytes":1234}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := s.Config()
	if err != nil || c.CacheLimitBytes != 1234 || c.DefaultOutput("gif", false) != "webp" {
		t.Fatalf("legacy config: %+v, %v", c, err)
	}
	c.FormatDefaults = map[string]string{"image": "webp", "video": "hevc", "gif": "apng", "jpeg": "jxl"}
	if err := s.Configure(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Config()
	if err != nil || !reflect.DeepEqual(c, loaded) {
		t.Fatalf("saved defaults changed: %+v, %v", loaded, err)
	}
	// A rule saved with the old separate Animated PNG choice becomes PNG;
	// the worker still receives APNG for the animated GIF source.
	if loaded.DefaultOutput("bmp", false) != "webp" || loaded.DefaultOutput("avi", true) != "hevc" || loaded.DefaultOutput("gif", false) != "png" {
		t.Fatal("custom and fallback rules were not applied")
	}
	if ConcreteOutput(loaded.DefaultOutput("gif", false), "gif") != "apng" || ConcreteOutput("png", "png") != "png" {
		t.Fatal("PNG must follow the source: animated sources need APNG")
	}
	// Removing a built-in rule must not resurrect it on the next read.
	if _, exists := loaded.FormatDefaults["png"]; exists {
		t.Fatal("removed PNG rule reappeared")
	}
	for _, invalid := range []map[string]string{{"mp4": "jpeg"}, {"gif": "jpeg"}, {"image": "shell"}, {"../../file": "jxl"}} {
		if err := ValidateFormatDefaults(invalid); err == nil {
			t.Fatalf("accepted invalid rule %v", invalid)
		}
	}
}

func TestDisabledFormatRulesPersistAndDoNotChooseAutomaticOutputs(t *testing.T) {
	s := Store{Root: t.TempDir()}
	c, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !c.FormatRuleEnabled("png") || c.DefaultOutput("png", false) != "jxl" {
		t.Fatal("legacy rules must start enabled")
	}

	c.FormatRulesEnabled = map[string]bool{
		"png":   false,
		"gif":   false,
		"image": false,
		"video": false,
		"mp4":   true,
	}
	if err := s.Configure(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, loaded) || loaded.FormatDefaults["png"] != "jxl" {
		t.Fatalf("disabled rules lost their output or settings: %+v", loaded)
	}
	for _, tc := range []struct {
		input string
		video bool
		want  string
	}{
		{"png", false, ""},
		{"gif", false, ""},
		{"bmp", false, ""},
		{"avi", true, ""},
		{"mp4", true, "av1-mp4"},
		{"jpeg", false, "jxl"},
	} {
		if got := loaded.DefaultOutput(tc.input, tc.video); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.input, got, tc.want)
		}
	}
	loaded.FormatRulesEnabled["png"] = true
	loaded.FormatRulesEnabled["image"] = true
	loaded.FormatRulesEnabled["video"] = true
	if err := s.Configure(loaded); err != nil {
		t.Fatal(err)
	}
	reopened, err := s.Config()
	if err != nil {
		t.Fatal(err)
	}
	if reopened.DefaultOutput("png", false) != "jxl" ||
		reopened.DefaultOutput("bmp", false) != "jxl" ||
		reopened.DefaultOutput("avi", true) != "av1-mp4" {
		t.Fatal("re-enabling rules did not restore saved outputs")
	}
	for _, states := range []map[string]bool{
		{"not-a-format": false},
		{"bmp": false}, // no saved BMP format rule
	} {
		if err := ValidateFormatRulesEnabled(states, reopened.FormatDefaults); err == nil {
			t.Fatalf("accepted state for missing rule: %v", states)
		}
	}
	if err := ValidateFormatRulesEnabled(nil, reopened.FormatDefaults); err != nil {
		t.Fatal("legacy enabled state should remain optional:", err)
	}
}

func TestMixedInputsUseIndividualDefaults(t *testing.T) {
	c := Config{FormatDefaults: DefaultFormatDefaults()}
	root := t.TempDir()
	pngChunk := func(name string, size int) []byte {
		b := make([]byte, 12+size)
		binary.BigEndian.PutUint32(b[:4], uint32(size))
		copy(b[4:], name)
		return b
	}
	png := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", 13)...)
	apng := append(append([]byte{}, png...), pngChunk("acTL", 8)...)
	webp := make([]byte, 30)
	copy(webp, "RIFF")
	copy(webp[8:], "WEBPVP8X")
	animatedWebp := append([]byte{}, webp...)
	animatedWebp[20] = 2
	cases := []struct {
		name, input, output string
		data                []byte
		video               bool
	}{
		{"still.JPG", "jpeg", "jxl", nil, false},
		{"still.png", "png", "jxl", append(png, pngChunk("IDAT", 0)...), false},
		{"animation.png", "apng", "webp", apng, false},
		{"still.webp", "webp", "jxl", webp, false},
		{"animation.webp", "animated-webp", "webp", animatedWebp, false},
		{"animation.gif", "gif", "webp", nil, false},
		{"input.jxl", "ajxl", "jxl", nil, false},
		{"video.M4V", "mp4", "av1-mp4", nil, true},
		{"video.mkv", "mkv", "av1-mkv", nil, true},
		{"video.webm", "webm", "av1-webm", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name)
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			f := &models.ImageFile{BaseFile: &models.BaseFile{Path: path}}
			input := SourceFormat(f)
			if input != tc.input || c.DefaultOutput(input, tc.video) != tc.output {
				t.Fatalf("got %s → %s, want %s → %s", input, c.DefaultOutput(input, tc.video), tc.input, tc.output)
			}
		})
	}
}

func TestAutomaticWorkerSelection(t *testing.T) {
	local, remote := Client{}, Client{URL: "http://worker", Token: "test-token"}
	caps := Capabilities{Formats: []Format{{ID: "jxl", Available: true}}}
	for _, tc := range []struct {
		name, mode string
		offline    bool
		unusable   bool
		wantRemote bool
		wantError  bool
		wantCalls  int
	}{
		{"prefer remote", "auto", false, false, true, false, 1},
		{"default is automatic", "", false, false, true, false, 1},
		{"offline fallback", "auto", true, false, false, false, 2},
		{"no remote codecs", "auto", false, true, false, false, 2},
		{"explicit remote fails", "remote", true, false, true, true, 1},
		{"explicit local", "local", false, false, false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client, _, notice, err := selectWorker(context.Background(), tc.mode, local, remote, func(ctx context.Context, c Client) (Capabilities, error) {
				calls++
				if c.URL != "" && tc.offline {
					return Capabilities{}, errors.New("worker offline")
				}
				if c.URL != "" && tc.unusable {
					return Capabilities{Formats: []Format{{ID: "jxl"}}}, nil
				}
				return caps, nil
			})
			if (client.URL != "") != tc.wantRemote || (err != nil) != tc.wantError || calls != tc.wantCalls {
				t.Fatalf("remote=%v error=%v probes=%d", client.URL != "", err, calls)
			}
			if tc.wantCalls == 2 && notice == "" {
				t.Fatal("local fallback was not disclosed")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	selected, available, notice, err := selectWorker(ctx, "auto", local, remote, func(context.Context, Client) (Capabilities, error) {
		calls++
		cancel()
		return Capabilities{}, context.Canceled
	})
	if !errors.Is(err, context.Canceled) || calls != 1 || selected.URL != "" || len(available.Formats) != 0 || notice != "" {
		t.Fatal("cancelled remote probe must not start local work")
	}
}

func TestJXLQualityScale(t *testing.T) {
	for _, tc := range []struct{ quality, distance float64 }{{100, 0}, {90, 1}, {80, 1.9}, {30, 6.4}, {0, 25}} {
		if math.Abs(JXLDistanceFromQuality(tc.quality)-tc.distance) > 0.00001 {
			t.Fatalf("quality %v: got %v, want %v", tc.quality, JXLDistanceFromQuality(tc.quality), tc.distance)
		}
	}
}

func TestEncodingPreferencesPersistAndResolvePerInput(t *testing.T) {
	s := Store{Root: t.TempDir()}
	c, err := s.Config()
	if err != nil || c.Backend != "auto" {
		t.Fatalf("default worker: %+v, %v", c, err)
	}
	c.Backend = "remote"
	c.EncodingDefaults = map[string]EncodingDefaults{
		"image": {Quality: 85, Effort: 6},
		"video": {Quality: 75, Effort: 4},
		"gif":   {Quality: 95, Effort: 9, FasterDecoding: intPtr(3)},
		"jxl":   {Quality: 90, Effort: 7, FasterDecoding: intPtr(0)},
	}
	if err := s.Configure(c); err != nil {
		t.Fatal(err)
	}
	c, err = s.Config()
	if err != nil || c.Backend != "remote" {
		t.Fatalf("reload: %+v %v", c, err)
	}
	for input, expected := range map[string]EncodingDefaults{
		"png": {Quality: 85, Effort: 6, FasterDecoding: intPtr(0)},
		"mp4": {Quality: 75, Effort: 4, FasterDecoding: intPtr(0)},
		"gif": {Quality: 95, Effort: 9, FasterDecoding: intPtr(3)},
		"jxl": {Quality: 90, Effort: 7, FasterDecoding: intPtr(0)},
	} {
		actual := c.DefaultEncoding(input)
		if actual.Quality != expected.Quality || actual.Effort != expected.Effort || actual.FasterDecodingValue() != expected.FasterDecodingValue() {
			t.Fatalf("%s: got %+v, expected %+v", input, actual, expected)
		}
	}
	for _, invalid := range []EncodingDefaults{
		{Quality: -1, Effort: 4}, {Quality: 101, Effort: 4},
		{Quality: 80, Effort: 0}, {Quality: 80, Effort: 10},
		{Quality: math.NaN(), Effort: 7}, {Quality: 80, Effort: 7, FasterDecoding: intPtr(5)},
	} {
		if ValidateEncodingDefaults(map[string]EncodingDefaults{"image": invalid}, "auto") == nil {
			t.Fatalf("accepted %+v", invalid)
		}
	}
	if ValidateEncodingDefaults(nil, "bad-worker") == nil {
		t.Fatal("accepted invalid worker")
	}
	legacyDefaults := Config{}.DefaultEncoding("gif")
	if legacyDefaults.FasterDecodingValue() != 2 {
		t.Fatalf("legacy defaults should favor playback, got tier %d", legacyDefaults.FasterDecodingValue())
	}
	genericDefaults := (Config{EncodingDefaults: map[string]EncodingDefaults{
		"mp4": {Quality: 80, Effort: 7, DecodingSpeed: intPtr(1)},
	}}).DefaultEncoding("mp4")
	if genericDefaults.DecodingSpeedValue() != 1 {
		t.Fatalf("generic decode speed should be retained, got %d", genericDefaults.DecodingSpeedValue())
	}
	legacyConfig := Config{EncodingDefaults: map[string]EncodingDefaults{
		"gif": {Quality: 95, Effort: 9, FasterDecoding: intPtr(3)},
	}}
	if got := legacyConfig.DefaultEncoding("gif").DecodingSpeedValue(); got != 3 {
		t.Fatalf("legacy fasterDecoding setting should migrate to decode speed, got %d", got)
	}
	if ValidateEncodingDefaults(map[string]EncodingDefaults{
		"image": {Quality: 90, Effort: 7, DecodingSpeed: intPtr(1), FasterDecoding: intPtr(2)},
	}, "auto") == nil {
		t.Fatal("accepted conflicting generic and legacy decode speed values")
	}
	if stillDefaults := (Config{}).DefaultEncoding("jxl"); stillDefaults.FasterDecodingValue() != 0 {
		t.Fatalf("still JXL defaults should retain density, got tier %d", stillDefaults.FasterDecodingValue())
	}
	for frames, input := range map[int]string{0: "ajxl", 1: "jxl", 2: "ajxl"} {
		f := &models.ImageFile{BaseFile: &models.BaseFile{Path: "image.jxl", FrameCount: frames}}
		if SourceFormat(f) != input {
			t.Fatalf("%d frames must select %s", frames, input)
		}
	}
	legacy := Config{FormatDefaults: map[string]string{"image": "jxl"}}
	if output := legacy.DefaultOutput("ajxl", false); output != "jxl" || ConcreteOutput(output, "ajxl") != "ajxl" {
		t.Fatal("legacy config would unnecessarily transcode AJXL")
	}
	if legacy.DefaultOutput("gif", false) != "webp" || legacy.DefaultOutput("animated-avif", false) != "webp" {
		t.Fatal("legacy config should use animated WebP for animation subtypes without saved rules")
	}
}

func TestJXLAndPNGAreOneChoiceEach(t *testing.T) {
	choices := map[string]Format{}
	for _, format := range ChoiceFormats(OutputFormats) {
		choices[format.ID] = format
	}
	for _, hidden := range []string{"ajxl", "apng"} {
		if _, ok := choices[hidden]; ok {
			t.Fatalf("%s must not be a separate choice", hidden)
		}
	}
	for _, id := range []string{"jxl", "png", "webp"} {
		if choices[id].Family != "animation" {
			t.Fatalf("%s must accept animated inputs, got %+v", id, choices[id])
		}
	}
	// Worker-facing catalog still validates the concrete variants.
	for _, id := range []string{"ajxl", "apng"} {
		found := false
		for _, format := range OutputFormats {
			found = found || format.ID == id
		}
		if !found {
			t.Fatalf("worker catalog lost %s", id)
		}
	}
	for input, want := range map[string]string{"jpeg": "jxl", "gif": "ajxl", "animated-webp": "ajxl", "ajxl": "ajxl", "mp4": "jxl"} {
		if got := ConcreteOutput("jxl", input); got != want {
			t.Fatalf("jxl for %s: got %s, want %s", input, got, want)
		}
	}
	saved := map[string]string{"gif": "apng", "ajxl": "ajxl", "png": "jxl"}
	NormalizeFormatDefaults(saved)
	if !reflect.DeepEqual(saved, map[string]string{"gif": "png", "ajxl": "jxl", "png": "jxl"}) {
		t.Fatalf("saved animated choices were not migrated: %v", saved)
	}
	if err := ValidateFormatDefaults(map[string]string{"gif": "png", "animated-webp": "jxl"}); err != nil {
		t.Fatalf("animated inputs must accept JPEG XL and PNG: %v", err)
	}
	if err := ValidateFormatDefaults(map[string]string{"gif": "jpeg"}); err == nil {
		t.Fatal("still-only outputs must still be rejected for animations")
	}
}

func TestChoiceNeedsBothStillAndAnimatedEncoders(t *testing.T) {
	worker := []Format{
		{ID: "png", Extension: "png", Family: "image", Available: true},
		{ID: "apng", Extension: "png", Family: "animation", Available: false},
		{ID: "jxl", Extension: "jxl", Family: "image", Available: true},
		{ID: "ajxl", Extension: "jxl", Family: "animation", Available: true},
	}
	available := map[string]bool{}
	for _, format := range ChoiceFormats(worker) {
		available[format.ID] = format.Available
	}
	if available["png"] || !available["jxl"] {
		t.Fatalf("PNG must need APNG support too: %v", available)
	}
}
