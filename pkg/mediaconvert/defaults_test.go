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
	if loaded.DefaultOutput("bmp", false) != "webp" || loaded.DefaultOutput("avi", true) != "hevc" || loaded.DefaultOutput("gif", false) != "apng" {
		t.Fatal("custom and fallback rules were not applied")
	}
	// Removing a built-in rule must not resurrect it on the next read.
	if _, exists := loaded.FormatDefaults["png"]; exists {
		t.Fatal("removed PNG rule reappeared")
	}
	for _, invalid := range []map[string]string{{"mp4": "jpeg"}, {"gif": "png"}, {"image": "shell"}, {"../../file": "jxl"}} {
		if err := ValidateFormatDefaults(invalid); err == nil {
			t.Fatalf("accepted invalid rule %v", invalid)
		}
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
		{"input.jxl", "ajxl", "ajxl", nil, false},
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
	if legacy.DefaultOutput("ajxl", false) != "ajxl" {
		t.Fatal("legacy config would unnecessarily transcode AJXL")
	}
	if legacy.DefaultOutput("gif", false) != "webp" || legacy.DefaultOutput("animated-avif", false) != "webp" {
		t.Fatal("legacy config should use animated WebP for animation subtypes without saved rules")
	}
}
