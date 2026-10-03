package mediaconvert

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSavingsThresholdBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy SavingsThresholds
		before int64
		after  int64
		pass   bool
	}{
		{"disabled minima still require savings", SavingsThresholds{}, 100, 99, true},
		{"larger", SavingsThresholds{}, 100, 101, false},
		{"equal", SavingsThresholds{}, 100, 100, false},
		{"zero input", SavingsThresholds{}, 0, 1, false},
		{"zero output", SavingsThresholds{}, 100, 0, false},
		{"bytes boundary", SavingsThresholds{MinimumSavedBytes: 20}, 100, 80, true},
		{"bytes below", SavingsThresholds{MinimumSavedBytes: 21}, 100, 80, false},
		{"percent boundary", SavingsThresholds{MinimumSavedPercent: 20}, 100, 80, true},
		{"percent below", SavingsThresholds{MinimumSavedPercent: 20.01}, 100, 80, false},
		{"both boundary", SavingsThresholds{20, 20}, 100, 80, true},
		{"only bytes pass", SavingsThresholds{20, 21}, 100, 80, false},
		{"only percent passes", SavingsThresholds{21, 20}, 100, 80, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.policy.Reason(tc.before, tc.after); (got == "") != tc.pass {
				t.Fatalf("eligibility=%v, reason=%q", tc.pass, got)
			}
		})
	}
}

func trialFixture(t *testing.T, output string) (Store, *memoryRepository, Client, *Trial) {
	t.Helper()
	s, repo, client, _ := fixture(t, output, false)
	options := Options{Format: "jxl", Quality: 80, Effort: 7, Hardware: "cpu", AllowLarger: true}
	trial, err := s.CreateTrial(context.Background(), 12, "trial-batch", TrialTarget{"image", 42}, client, options,
		Format{ID: "jxl", Family: "image", Extension: "jxl", Label: "JPEG XL"}, SavingsThresholds{}, "fixture-v1", true)
	if err != nil {
		t.Fatal(err)
	}
	return s, repo, client, trial
}

func reviewSnapshot(t *testing.T, repo *memoryRepository) []byte {
	t.Helper()
	data, err := json.Marshal(repo.f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertNoActivation(t *testing.T, s Store, repo *memoryRepository, before []byte, source string) {
	t.Helper()
	current, err := json.Marshal(repo.f)
	if err != nil || string(current) != string(before) {
		t.Fatalf("native file metadata changed: %v", err)
	}
	data, err := os.ReadFile(repo.f.Base().Path)
	if err != nil || string(data) != source {
		t.Fatalf("source changed: %v", err)
	}
	records, err := s.History()
	if err != nil || len(records) != 0 {
		t.Fatalf("trials must not create restore/history entries: %v, %v", records, err)
	}
	backups, err := filepath.Glob(filepath.Join(s.Root, "*.original"))
	if err != nil || len(backups) > 0 {
		t.Fatalf("trial consumed original backup: %v", backups)
	}
}

func TestTrialApplyAndRestoreReuseNativeIdentity(t *testing.T) {
	s, repo, client, original := fixture(t, "compressed", false)
	before := reviewSnapshot(t, repo)
	options := Options{Format: "jxl", Hardware: "cpu", Quality: 80, Effort: 7}
	trial, err := s.CreateTrial(context.Background(), 12, "trial", TrialTarget{"image", 42}, client, options,
		Format{ID: "jxl", Family: "image", Extension: "jxl"}, SavingsThresholds{}, "fixture-v1", true)
	if err != nil || trial.Status != "verified" || !trial.Retained || !trial.Verified {
		t.Fatalf("verified trial: %+v, %v", trial, err)
	}
	assertNoActivation(t, s, repo, before, original)
	stats := SummarizeTrials([]*Trial{trial})
	if stats.Verified != 1 || stats.PotentialSavedBytes != int64(len(original)-len("compressed")) || stats.CacheBytes != 10 {
		t.Fatalf("trial summary: %+v", stats)
	}
	// Apply cannot depend on calling the worker again; this token would fail the fixture.
	client.Token = "unused-during-apply"
	record, err := s.ApplyTrial(context.Background(), trial.ID, "apply", client, options, trial.Savings, "fixture-v1")
	if err != nil || record.Status != "complete" || !record.Cached || repo.f.Base().ID != 12 {
		t.Fatalf("apply: %+v, %v", record, err)
	}
	if _, err := os.Stat(s.trialOutput(trial)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("applied output must leave trial cache")
	}
	if err := s.Restore(context.Background(), record.ID); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(repo.f.Base().Path)
	if err != nil || string(data) != original || repo.f.Base().Fingerprints.For("phash").Int64() != -9223372036854775701 {
		t.Fatalf("restore lost original bytes or fingerprints: %v", err)
	}
}

func TestSavingsGatePrecedesBackupAndCannotBeOverridden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
		policy SavingsThresholds
	}{
		{"larger", strings.Repeat("large", 100), SavingsThresholds{}},
		{"minimum", "compressed", SavingsThresholds{MinimumSavedBytes: 1000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, repo, client, original := fixture(t, tc.output, false)
			before := reviewSnapshot(t, repo)
			record, err := s.ConvertWithSavings(context.Background(), 12, "batch", client, Options{Format: "jxl", AllowLarger: true}, Format{Extension: "jxl"}, tc.policy)
			if err != nil || record.Status != "skipped" || record.Cached {
				t.Fatalf("compression must skip before backup: %+v, %v", record, err)
			}
			current := reviewSnapshot(t, repo)
			data, err := os.ReadFile(repo.f.Base().Path)
			if err != nil || string(data) != original || string(current) != string(before) {
				t.Fatal("skipped conversion changed source metadata")
			}
			if _, err := os.Stat(s.backup(record.ID)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("skipped conversion created a backup")
			}
			stats := Summarize([]*Record{record})
			if stats.Skipped != 1 || stats.Converted != 0 || stats.SavedBytes != 0 || stats.CacheBytes != 0 {
				t.Fatalf("skipped is not realized savings: %+v", stats)
			}
		})
	}
	// Existing upscaling activation retains its separate intentional-larger policy.
	s, _, client, _ := fixture(t, strings.Repeat("large", 100), false)
	record, err := s.Convert(context.Background(), 12, "upscale", client, Options{Format: "jxl", Upscaler: "waifu2x", AllowLarger: true}, Format{Extension: "jxl"})
	if err != nil || record.Status != "complete" {
		t.Fatalf("compression policy leaked into upscaling: %+v, %v", record, err)
	}
}

func TestTrialBindsMeasuredTimestampInsteadOfOldCatalogTimestamp(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	repo.f.Base().ModTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	trial, err := s.CreateTrial(context.Background(), 12, "trial", TrialTarget{"image", 42}, client, Options{Format: "jxl"},
		Format{ID: "jxl", Family: "image", Extension: "jxl"}, SavingsThresholds{}, "fixture-v1", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TrialFile(context.Background(), trial.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyTrial(context.Background(), trial.ID, "apply", client, trial.Options, trial.Savings, "fixture-v1"); err != nil {
		t.Fatalf("unchanged measured source rejected because of old catalog timestamp: %v", err)
	}
}

func TestApplyRejectsChangedTrialBindings(t *testing.T) {
	for _, change := range []string{"options", "policy", "worker", "endpoint", "source", "output", "expired"} {
		t.Run(change, func(t *testing.T) {
			s, repo, client, trial := trialFixture(t, "compressed")
			options, policy, signature := trial.Options, trial.Savings, "fixture-v1"
			switch change {
			case "options":
				options.Quality++
			case "policy":
				policy.MinimumSavedBytes++
			case "worker":
				signature = "fixture-v2"
			case "endpoint":
				client.URL += "/other"
			case "source":
				path, modtime := repo.f.Base().Path, repo.f.Base().ModTime
				if err := os.WriteFile(path, []byte(strings.Repeat("different data", 30)), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, modtime, modtime); err != nil {
					t.Fatal(err)
				}
			case "output":
				if err := os.WriteFile(s.trialOutput(trial), []byte("tampered!!"), 0600); err != nil {
					t.Fatal(err)
				}
			case "expired":
				trial.ExpiresAt = time.Now().Add(-time.Hour)
				if err := s.saveTrial(trial); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.ApplyTrial(context.Background(), trial.ID, "apply", client, options, policy, signature)
			if err == nil {
				t.Fatal("stale trial was applied")
			}
			if repo.f.Base().ID != 12 || !strings.HasSuffix(repo.f.Base().Path, "source.png") {
				t.Fatal("failed revalidation activated output")
			}
			history, err := s.History()
			if err != nil || len(history) != 0 {
				t.Fatal("revalidation failed after journal/backup creation")
			}
		})
	}
}

func TestZeroByteInputAndUnversionedWorkerRejectSavedTrial(t *testing.T) {
	s, repo, client, _ := fixture(t, "compressed", false)
	if _, err := s.CreateTrial(context.Background(), 12, "trial", TrialTarget{"image", 42}, client, Options{}, Format{}, SavingsThresholds{}, "", true); err == nil {
		t.Fatal("unversioned worker allowed retained trial")
	}
	if err := os.Truncate(repo.f.Base().Path, 0); err != nil {
		t.Fatal(err)
	}
	repo.f.Base().Size = 0
	if _, err := s.CreateTrial(context.Background(), 12, "trial", TrialTarget{"image", 42}, client, Options{}, Format{}, SavingsThresholds{}, "fixture-v1", true); err == nil || !strings.Contains(err.Error(), "zero-byte") {
		t.Fatalf("zero-byte input: %v", err)
	}
}

func TestEstimateDiscardsOutputAndManifest(t *testing.T) {
	s, repo, client, original := fixture(t, "compressed", false)
	before := reviewSnapshot(t, repo)
	trial, err := s.CreateTrial(context.Background(), 12, "estimate", TrialTarget{"image", 42}, client, Options{Format: "jxl"},
		Format{ID: "jxl", Family: "image", Extension: "jxl"}, SavingsThresholds{}, "fixture-v1", false)
	if err != nil || !trial.Verified || trial.Retained || trial.Result.Size != 10 {
		t.Fatalf("estimate measurement: %+v, %v", trial, err)
	}
	if _, err := os.Stat(s.trialDir(trial.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("estimate left temporary output/manifest")
	}
	assertNoActivation(t, s, repo, before, original)
}

func TestTrialApplyRecoveryAfterDatabaseFailure(t *testing.T) {
	s, repo, client, trial := trialFixture(t, "compressed")
	repo.failSwap = true
	record, err := s.ApplyTrial(context.Background(), trial.ID, "apply", client, trial.Options, trial.Savings, "fixture-v1")
	if err == nil || record.Status != "prepared" {
		t.Fatalf("expected recoverable DB failure: %+v, %v", record, err)
	}
	repo.failSwap = false
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.TrialRecord(trial.ID)
	if err != nil || recovered.Status != "verified" || !recovered.Retained {
		t.Fatalf("trial not retryable after recovery: %+v, %v", recovered, err)
	}
	record, err = s.ApplyTrial(context.Background(), trial.ID, "retry", client, trial.Options, trial.Savings, "fixture-v1")
	if err != nil || record.Status != "complete" {
		t.Fatalf("retry after recovery: %+v, %v", record, err)
	}
	// A crash after commit but before marking the trial applied must reconcile.
	recovered.Status, recovered.RecordID = "applying", record.ID
	if err := s.saveTrial(recovered); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	recovered, _ = s.TrialRecord(trial.ID)
	if recovered.Status != "applied" || recovered.Retained {
		t.Fatalf("committed apply recovery: %+v", recovered)
	}
}

func TestCancelledTrialCleansPartialOutput(t *testing.T) {
	s, repo, _, original := fixture(t, "compressed", false)
	before := reviewSnapshot(t, repo)
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, _ := json.Marshal(Result{Width: 64, Height: 64, Frames: 1, Size: 10, Format: "jxl", Signature: "fixture-v1"})
		w.Header().Set("X-Stash-Conversion", base64.StdEncoding.EncodeToString(payload))
		w.Header().Set("Content-Length", "10")
		w.Header().Set("X-Stash-Content-MD5", strings.Repeat("0", 32))
		_, _ = w.Write([]byte("x"))
		w.(http.Flusher).Flush()
		close(started)
		<-release
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	trial, err := s.CreateTrial(ctx, 12, "cancel", TrialTarget{"image", 42}, Client{URL: server.URL}, Options{Format: "jxl"}, Format{ID: "jxl", Family: "image", Extension: "jxl"}, SavingsThresholds{}, "fixture-v1", true)
	if err == nil || trial.Status != "cancelled" || trial.Retained {
		t.Fatalf("cancellation: %+v, %v", trial, err)
	}
	if _, err := os.Stat(s.trialOutput(trial)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled partial output retained")
	}
	assertNoActivation(t, s, repo, before, original)
}

func TestTrialExpiryCacheAndInterruptedRecovery(t *testing.T) {
	s, _, client, trial := trialFixture(t, "compressed")
	config, _ := s.Config()
	config.TrialCacheLimitBytes = 5
	if err := s.Configure(config); err != nil {
		t.Fatal(err)
	}
	if err := s.TrimTrials(false); err != nil {
		t.Fatal(err)
	}
	expired, err := s.TrialRecord(trial.ID)
	if err != nil || expired.Status != "expired" || expired.Retained {
		t.Fatalf("cache eviction: %+v, %v", expired, err)
	}
	config.TrialCacheLimitBytes = 1000
	if err := s.Configure(config); err != nil {
		t.Fatal(err)
	}
	next, err := s.CreateTrial(context.Background(), 12, "again", trial.Target, client, trial.Options, trial.Format, trial.Savings, "fixture-v1", true)
	if err != nil {
		t.Fatal(err)
	}
	// A crash between marking applying and writing the activation journal is retryable.
	next.Status, next.RecordID = "applying", NewID()
	if err := s.saveTrial(next); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	next, _ = s.TrialRecord(next.ID)
	if next.Status != "verified" || !next.Retained {
		t.Fatalf("retryable apply recovery: %+v", next)
	}
	next.Status = "encoding"
	if err := s.saveTrial(next); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	next, _ = s.TrialRecord(next.ID)
	if next.Status != "failed" || next.Retained {
		t.Fatalf("interrupted trial recovery: %+v", next)
	}
	next.ExpiresAt = time.Now().Add(-time.Hour)
	if err := s.saveTrial(next); err != nil {
		t.Fatal(err)
	}
	if err := s.TrimTrials(false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TrialRecord(next.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired manifest was not removed")
	}
}

func TestEstimateStratificationWeightedSavingsAndUncertainty(t *testing.T) {
	candidates := []SampleCandidate{
		{Key: "still-small", Input: "png", Size: 100}, {Key: "still-big", Input: "png", Size: 900},
		{Key: "animation", Input: "gif", Animated: true, Size: 1000}, {Key: "video", Input: "mp4", Animated: true, Size: 20 * 1024 * 1024},
	}
	sample := SelectEstimateSample(candidates, 3)
	strata := map[string]bool{}
	for _, c := range sample {
		strata[c.Stratum()] = true
	}
	if len(strata) != 3 || reflect.DeepEqual(sample, candidates[:3]) {
		t.Fatal("sample must cover formats, animation and size strata")
	}
	measurements := []EstimateMeasurement{
		{Candidate: candidates[0], OutputBytes: 50, Eligible: true},
		{Candidate: candidates[1], OutputBytes: 900}, // skipped => zero realized savings
		{Candidate: candidates[2], OutputBytes: 500, Eligible: true},
		{Candidate: candidates[3], OutputBytes: candidates[3].Size},
	}
	estimate := SummarizeEstimate(candidates, measurements)
	if !estimate.Covered || estimate.EstimatedSavedBytes != 550 || estimate.ObservedLowBytes != 500 || estimate.ObservedHighBytes != 1000 {
		t.Fatalf("weighted, eligible-only estimate: %+v", estimate)
	}
	measurements[2].Error = "unsupported animation"
	estimate = SummarizeEstimate(candidates, measurements)
	if estimate.Covered || estimate.EstimatedSavedBytes != 0 {
		t.Fatal("failed stratum projected whole-selection savings")
	}
	estimate = SummarizeEstimate(candidates, measurements[:2])
	if estimate.Covered || estimate.EstimatedSavedBytes != 0 {
		t.Fatal("uncovered strata projected whole-selection savings")
	}
}
