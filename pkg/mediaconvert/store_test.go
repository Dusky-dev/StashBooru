package mediaconvert

import (
	"context"
	"crypto/md5" //nolint:gosec // Content identifier fixtures.
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode/utf8"

	"github.com/stashapp/stash/pkg/models"
)

type memoryRepository struct {
	f              models.File
	failSwap       bool
	commitThenFail bool
	afterSwap      func(before, after models.File)
}

func (r *memoryRepository) Get(context.Context, models.FileID) (models.File, error) {
	return r.f.Clone(), nil
}
func (r *memoryRepository) Swap(_ context.Context, before, after models.File) error {
	if !SameFile(r.f, before) {
		return errors.New("compare-and-swap mismatch")
	}
	if r.failSwap {
		return errors.New("injected DB failure")
	}
	r.f = after.Clone()
	if r.afterSwap != nil {
		r.afterSwap(before, after)
	}
	if r.commitThenFail {
		return errors.New("injected interruption after commit")
	}
	return nil
}

func fixture(t *testing.T, output string, corrupt bool) (Store, *memoryRepository, Client, string) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	contents := strings.Repeat("original bytes", 30)
	if err := os.WriteFile(source, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := MD5(source)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	r := &memoryRepository{f: &models.ImageFile{BaseFile: &models.BaseFile{ID: 12, Path: source, Basename: "source.png", Size: stat.Size(),
		DirEntry: models.DirEntry{ModTime: stat.ModTime()}, Fingerprints: models.Fingerprints{
			{Type: "md5", Fingerprint: hash}, {Type: "phash", Fingerprint: int64(-9223372036854775701)},
		}}, Width: 64, Height: 64, Format: "png"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing worker token")
		}
		payload := Result{Width: 64, Height: 64, Frames: 1, Size: int64(len(output)), Format: "jxl", VideoCodec: "jpegxl", Encoder: "fixture", Signature: "fixture-v1"}
		data, err := json.Marshal(payload)
		if err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(output)))
		w.Header().Set("X-Stash-Conversion", base64.StdEncoding.EncodeToString(data))
		checksum := fmt.Sprintf("%x", md5.Sum([]byte(output))) //nolint:gosec // Transport checksum fixture.
		if corrupt {
			checksum = "incorrect"
		}
		w.Header().Set("X-Stash-Content-MD5", checksum)
		_, _ = w.Write([]byte(output))
	}))
	t.Cleanup(server.Close)
	return Store{Root: filepath.Join(dir, "cache"), Repository: r}, r, Client{URL: server.URL, Token: "fixture-token"}, contents
}

func convertFixture(t *testing.T, s Store, c Client) *Record {
	t.Helper()
	r, err := s.Convert(context.Background(), 12, "batch", c, Options{Format: "jxl"}, Format{Extension: "jxl"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestConvertRestorePreservesSourceIdentity(t *testing.T) {
	s, repo, client, original := fixture(t, "compressed", false)
	sourcePath := repo.f.Base().Path
	originalHash := repo.f.Base().Fingerprints.GetString("md5")
	r := convertFixture(t, s, client)
	if r.Status != "complete" || !r.Cached {
		t.Fatalf("unexpected conversion: %+v", r)
	}
	if _, err := os.Stat(sourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("original path should be retired")
	}
	if repo.f.Base().ID != 12 || repo.f.Base().Fingerprints.GetString("source_md5") != originalHash {
		t.Fatal("source identity lost")
	}
	if repo.f.Base().Fingerprints.GetString("md5") == originalHash {
		t.Fatal("active checksum must describe converted bytes")
	}
	if repo.f.Base().Fingerprints.GetString("source_phash") != "800000000000006b" {
		t.Fatal("source pHash rounded or lost")
	}
	// Simulate a new pHash generated after conversion; it must not prevent restore.
	repo.f.Base().SetFingerprint(models.Fingerprint{Type: "phash", Fingerprint: int64(7)})
	if err := s.Restore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != original || repo.f.Base().Fingerprints.For("phash").Int64() != -9223372036854775701 {
		t.Fatal("restore did not preserve exact bytes and pHash")
	}
	reloaded, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != "restored" || reloaded.Cached {
		t.Fatal("restore cache not released")
	}
}

func TestConvertTruncatesLongUnicodeBasename(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	base := repo.f.Base()
	longName := strings.Repeat("界", 79) + ".png"
	longPath := filepath.Join(filepath.Dir(base.Path), longName)
	if err := os.Rename(base.Path, longPath); err != nil {
		t.Fatal(err)
	}
	base.Path, base.Basename = longPath, longName

	r := convertFixture(t, s, client)
	convertedPath := repo.f.Base().Path
	convertedName := filepath.Base(convertedPath)
	if len(convertedName) > maxFilenameComponentBytes {
		t.Fatalf("converted basename is %d bytes, filesystem limit is %d", len(convertedName), maxFilenameComponentBytes)
	}
	if !utf8.ValidString(convertedName) {
		t.Fatalf("converted basename contains invalid UTF-8: %q", convertedName)
	}
	if r.Status != "complete" {
		t.Fatalf("conversion did not complete: %+v", r)
	}
	if _, err := os.Stat(convertedPath); err != nil {
		t.Fatalf("converted file missing at %q: %v", convertedPath, err)
	}
	if _, err := os.Stat(longPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source path should be retired after conversion: %v", err)
	}
}

func TestEvictionPreservesProvenance(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	if err := s.Configure(Config{CacheLimitBytes: 0}); err != nil {
		t.Fatal(err)
	}
	r := convertFixture(t, s, client)
	r, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cached {
		t.Fatal("zero cache should evict original")
	}
	if _, err := os.Stat(repo.f.Base().Path); err != nil {
		t.Fatal("converted file was removed")
	}
	if repo.f.Base().Fingerprints.GetString("source_md5") == "" {
		t.Fatal("provenance must outlive cache eviction")
	}
	if err := s.Restore(context.Background(), r.ID); err == nil {
		t.Fatal("evicted original cannot be restored")
	}
}

func TestFailedTransferAndLargerOutputKeepSource(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		corrupt      bool
		status       string
	}{
		{"corrupt", "small", true, "failed"}, {"larger", strings.Repeat("x", 1000), false, "skipped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, repo, client, original := fixture(t, tc.output, tc.corrupt)
			source := repo.f.Base().Path
			r, err := s.Convert(context.Background(), 12, "batch", client, Options{Format: "jxl"}, Format{Extension: "jxl"})
			if tc.corrupt && err == nil {
				t.Fatal("corrupted transfer succeeded")
			}
			if r.Status != tc.status {
				t.Fatalf("status %s", r.Status)
			}
			data, err := os.ReadFile(source)
			if err != nil || string(data) != original {
				t.Fatal("source was changed")
			}
			if repo.f.Base().Path != source {
				t.Fatal("failed output was activated")
			}
		})
	}
}

func TestRecoverSkipsLegacyOverlongOutputPath(t *testing.T) {
	s, repo, _, _ := fixture(t, "compressed", false)
	before := repo.f.Clone()
	sourcePath := before.Base().Path
	checksum, err := MD5(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	before.Base().SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: checksum})

	after := before.Clone()
	overlongPath := filepath.Join(filepath.Dir(sourcePath), strings.Repeat("x", maxFilenameComponentBytes+1)+".webp")
	if _, err := os.Lstat(overlongPath); !errors.Is(err, syscall.ENAMETOOLONG) {
		t.Skipf("filesystem did not report an overlong component: %v", err)
	}
	after.Base().Path = overlongPath
	after.Base().Basename = filepath.Base(overlongPath)

	r := &Record{ID: NewID(), Status: "prepared", Before: snapshot(before), After: snapshot(after)}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatalf("recovery should discard the inaccessible legacy output path: %v", err)
	}
	recovered, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "failed" {
		t.Fatalf("status = %q; want failed", recovered.Status)
	}
	if repo.f.Base().Path != sourcePath || !matches(sourcePath, checksum) {
		t.Fatal("recovery changed the active source")
	}
}

func TestRecoverInterruptedActivation(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			s, repo, client, _ := fixture(t, "compressed", false)
			repo.failSwap, repo.commitThenFail = !committed, committed
			r, err := s.Convert(context.Background(), 12, "batch", client, Options{Format: "jxl"}, Format{Extension: "jxl"})
			if err == nil || r.Status != "prepared" {
				t.Fatal("fault was not injected at activation")
			}
			if err := s.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			reloaded, err := s.Record(r.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := "failed"
			if committed {
				want = "complete"
			}
			if reloaded.Status != want {
				t.Fatalf("got %s, want %s", reloaded.Status, want)
			}
			if _, err := os.Stat(repo.f.Base().Path); err != nil {
				t.Fatal("recovery left DB pointing to missing file")
			}
		})
	}
}

func TestConvertKeepsChangedSourceAfterActivation(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	sourcePath := repo.f.Base().Path
	changed := []byte("edited after the database switched to the converted file")
	repo.afterSwap = func(_, _ models.File) {
		if err := os.WriteFile(sourcePath, changed, 0600); err != nil {
			t.Error(err)
		}
	}

	r, err := s.Convert(context.Background(), 12, "batch", client, Options{Format: "jxl"}, Format{Extension: "jxl"})
	if err != nil {
		t.Fatalf("conversion should complete with a warning: %v", err)
	}
	if r.Status != "complete" || !strings.Contains(r.Error, "file changed; refusing to remove") || !strings.Contains(r.Error, "(file kept)") {
		t.Fatalf("conversion warning was not recorded: %+v", r)
	}
	if repo.f.Base().Path != r.After.File().Base().Path {
		t.Fatal("converted file is not active")
	}
	actual, err := os.ReadFile(sourcePath)
	if err != nil || string(actual) != string(changed) {
		t.Fatal("changed source copy was not preserved")
	}
}

func TestRecoverCompletesWithWarningWhenInactiveSourceChanged(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	sourcePath := repo.f.Base().Path
	changed := []byte("edited after the database switched to the converted file")
	repo.commitThenFail = true
	repo.afterSwap = func(_, _ models.File) {
		if err := os.WriteFile(sourcePath, changed, 0600); err != nil {
			t.Error(err)
		}
	}
	r, err := s.Convert(context.Background(), 12, "batch", client, Options{Format: "jxl"}, Format{Extension: "jxl"})
	if err == nil || r.Status != "prepared" {
		t.Fatal("expected an interruption immediately after activation")
	}
	repo.commitThenFail = false
	if err := s.Recover(context.Background()); err != nil {
		t.Fatalf("recovery should finalize while preserving changed file: %v", err)
	}
	reloaded, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != "complete" || !strings.Contains(reloaded.Error, "file changed; refusing to remove") {
		t.Fatalf("recovery warning was not recorded: %+v", reloaded)
	}
	actual, err := os.ReadFile(sourcePath)
	if err != nil || string(actual) != string(changed) {
		t.Fatal("recovery removed the changed source copy")
	}
}

func TestRestoreKeepsChangedConvertedFileAfterActivation(t *testing.T) {
	s, repo, client, original := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	convertedPath := r.After.File().Base().Path
	changed := []byte("edited after the database switched back to the original")
	repo.afterSwap = func(_, _ models.File) {
		if err := os.WriteFile(convertedPath, changed, 0600); err != nil {
			t.Error(err)
		}
	}

	if err := s.ToggleRestore(context.Background(), r.ID); err != nil {
		t.Fatalf("restore should finish with a warning: %v", err)
	}
	reloaded, err := s.Record(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != "restored" || !strings.Contains(reloaded.Error, "file changed; refusing to remove") {
		t.Fatalf("restore warning was not recorded: %+v", reloaded)
	}
	if repo.f.Base().Path != reloaded.Before.File().Base().Path {
		t.Fatal("original file is not active after restore")
	}
	active, err := os.ReadFile(repo.f.Base().Path)
	if err != nil || string(active) != original {
		t.Fatal("restore did not keep the original bytes active")
	}
	kept, err := os.ReadFile(convertedPath)
	if err != nil || string(kept) != string(changed) {
		t.Fatal("changed converted file was not preserved")
	}
}

func TestRestoreRefusesOccupiedPathAndCorruptBackup(t *testing.T) {
	s, _, client, _ := fixture(t, "compressed", false)
	r := convertFixture(t, s, client)
	path := r.Before.File().Base().Path
	if err := os.WriteFile(path, []byte("unrelated"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(context.Background(), r.ID); err == nil {
		t.Fatal("occupied destination accepted")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "unrelated" {
		t.Fatal("unrelated file overwritten")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.backup(r.ID), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(context.Background(), r.ID); err == nil {
		t.Fatal("corrupt cache accepted")
	}
}

func TestStatsAccountForCacheAndNegativeSavings(t *testing.T) {
	file := func(size int64, id models.FileID) Snapshot {
		return Snapshot{Image: &models.ImageFile{BaseFile: &models.BaseFile{Size: size, ID: id}}}
	}
	stats := Summarize([]*Record{{Status: "complete", Cached: true, Before: file(100, 1), After: file(60, 1)}, {Status: "complete", Before: file(50, 2), After: file(70, 2)}})
	if stats.SavedBytes != 20 || stats.NetSavedBytes != -80 || stats.AverageSavedBytes != 10 || stats.LargerFiles != 1 {
		t.Fatalf("incorrect stats: %+v", stats)
	}
	stats = Summarize([]*Record{{Status: "complete", Before: file(100, 1), After: file(60, 1)}, {Status: "complete", Before: file(60, 1), After: file(50, 1)}})
	if stats.Files != 1 || stats.Converted != 2 || stats.SavedPercent != 50 || stats.AverageSavedBytes != 50 {
		t.Fatalf("repeated conversion counted twice: %+v", stats)
	}
}

func TestConversionWithoutScannedMD5(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	repo.f.Base().Fingerprints = repo.f.Base().Fingerprints.Remove("md5")
	r := convertFixture(t, s, client)
	if repo.f.Base().Fingerprints.GetString("source_md5") == "" {
		t.Fatal("source MD5 was not calculated")
	}
	if err := s.Restore(context.Background(), r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestConvertedAnimationFrameRate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "converted")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	before := &models.ImageFile{BaseFile: &models.BaseFile{Path: path}}
	for _, tc := range []struct {
		name, format, codec     string
		frames                  int64
		duration, nominal, want float64
	}{
		{"AJXL final hold from old worker", "jxl", "jpegxl", 120, 5.02, 25, 120 / 5.02},
		{"GIF final hold from old worker", "gif", "gif", 120, 5.02, 25, 120 / 5.02},
		{"constant AJXL", "jxl", "jpegxl", 120, 4.8, 25, 25},
		{"still image", "jxl", "jpegxl", 1, 0.04, 25, 0},
		{"video unchanged", "mp4", "h264", 120, 5.02, 25, 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := convertedFile(before, path, Result{Format: tc.format, VideoCodec: tc.codec, Frames: tc.frames, Duration: tc.duration, FrameRate: tc.nominal}, "hash")
			if err != nil {
				t.Fatal(err)
			}
			var rate float64
			switch f := f.(type) {
			case *models.ImageFile:
				rate = f.FrameRate
			case *models.VideoFile:
				rate = f.FrameRate
			default:
				t.Fatalf("unexpected file type %T", f)
			}
			if math.Abs(rate-tc.want) > 0.000001 {
				t.Fatalf("rate %v; want %v", rate, tc.want)
			}
		})
	}
}
