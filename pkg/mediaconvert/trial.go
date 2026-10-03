package mediaconvert

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type TrialTarget struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}

type Trial struct {
	Version        int               `json:"version"`
	ID             string            `json:"id"`
	Batch          string            `json:"batch"`
	Target         TrialTarget       `json:"target"`
	CreatedAt      time.Time         `json:"createdAt"`
	ExpiresAt      time.Time         `json:"expiresAt"`
	Status         string            `json:"status"`
	Error          string            `json:"error,omitempty"`
	Before         Snapshot          `json:"before"`
	Options        Options           `json:"options"`
	Format         Format            `json:"format"`
	Savings        SavingsThresholds `json:"savings"`
	Result         Result            `json:"result"`
	Backend        string            `json:"backend"`
	WorkerIdentity string            `json:"workerIdentity"`
	SourceSHA256   string            `json:"sourceSHA256"`
	SourceModTime  time.Time         `json:"sourceModTime"`
	OutputSHA256   string            `json:"outputSHA256"`
	OutputMD5      string            `json:"outputMD5"`
	Verified       bool              `json:"verified"`
	Retained       bool              `json:"retained"`
	RecordID       string            `json:"recordID,omitempty"`
	Notices        []string          `json:"notices"`
}

func workerIdentity(client Client, signature string) string {
	value := sha256.Sum256([]byte(client.URL + "\x00" + signature))
	return hex.EncodeToString(value[:])
}

func sha256File(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buffer)
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s Store) trialDir(id string) string { return filepath.Join(s.Root, "trials", id) }
func (s Store) trialOutput(t *Trial) string {
	return filepath.Join(s.trialDir(t.ID), "output."+t.Format.Extension)
}
func (s Store) saveTrial(t *Trial) error {
	return atomicJSON(filepath.Join(s.trialDir(t.ID), "trial.json"), t)
}

func (s Store) TrialRecord(id string) (*Trial, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid trial ID")
	}
	var t Trial
	if err := readJSON(filepath.Join(s.trialDir(id), "trial.json"), &t); err != nil {
		return nil, err
	}
	knownFormat := false
	for _, format := range OutputFormats {
		knownFormat = knownFormat || (format.ID == t.Format.ID && format.Extension == t.Format.Extension && format.Family == t.Format.Family)
	}
	if t.Version != 1 || t.ID != id || t.Before.File() == nil || t.Before.File().Base().ID <= 0 || t.Before.File().Base().Path == "" || !knownFormat {
		return nil, fmt.Errorf("invalid trial manifest")
	}
	switch t.Status {
	case "encoding", "verified", "skipped", "failed", "cancelled", "applying", "applied", "expired":
	default:
		return nil, fmt.Errorf("invalid trial status")
	}
	return &t, nil
}

func (s Store) Trials() ([]*Trial, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "trials"))
	if errors.Is(err, os.ErrNotExist) {
		return []*Trial{}, nil
	}
	if err != nil {
		return nil, err
	}
	ret := []*Trial{}
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) {
			continue
		}
		t, err := s.TrialRecord(entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		ret = append(ret, t)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].CreatedAt.Equal(ret[j].CreatedAt) {
			return ret[i].ID < ret[j].ID
		}
		return ret[i].CreatedAt.Before(ret[j].CreatedAt)
	})
	return ret, nil
}

type TrialStats struct {
	Verified             int     `json:"verified"`
	Skipped              int     `json:"skipped"`
	Failed               int     `json:"failed"`
	Applied              int     `json:"applied"`
	SourceBytes          int64   `json:"sourceBytes"`
	OutputBytes          int64   `json:"outputBytes"`
	PotentialSavedBytes  int64   `json:"potentialSavedBytes"`
	WeightedSavedPercent float64 `json:"weightedSavedPercent"`
	CacheBytes           int64   `json:"cacheBytes"`
}

func SummarizeTrials(trials []*Trial) TrialStats {
	var ret TrialStats
	for _, t := range trials {
		if t.Retained {
			ret.CacheBytes += t.Result.Size
		}
		switch t.Status {
		case "verified":
			if !t.Retained || !t.ExpiresAt.After(time.Now()) {
				continue
			}
			ret.Verified++
			ret.SourceBytes += t.Before.File().Base().Size
			ret.OutputBytes += t.Result.Size
		case "skipped":
			ret.Skipped++
		case "failed", "cancelled":
			ret.Failed++
		case "applied":
			ret.Applied++
		}
	}
	ret.PotentialSavedBytes = ret.SourceBytes - ret.OutputBytes
	if ret.SourceBytes > 0 {
		ret.WeightedSavedPercent = 100 * float64(ret.PotentialSavedBytes) / float64(ret.SourceBytes)
	}
	return ret
}

func checkDisk(path string, required int64) error {
	if required < 0 {
		return fmt.Errorf("disk reservation overflow")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	available, err := availableDiskBytes(path)
	if err != nil {
		return fmt.Errorf("check free disk space: %w", err)
	}
	if uint64(required) > available {
		return fmt.Errorf("insufficient temporary disk space: reserve %d bytes, available %d", required, available)
	}
	return nil
}

type DiskReservation struct {
	SourceBytes   int64 `json:"sourceBytes"`
	OutputBytes   int64 `json:"outputBytes"`
	StagingBytes  int64 `json:"stagingBytes"`
	BackupBytes   int64 `json:"backupBytes"`
	RequiredBytes int64 `json:"requiredBytes"`
}

// Reserve before starting a batch. Encoder sizes are unknown, so use twice the
// source for proposed output, an additional output copy, and original backups.
// Reservations are conservative planning estimates, not codec size guarantees.
func (s Store) Preflight(sources []models.File, apply bool) (DiskReservation, error) {
	var ret DiskReservation
	directories := map[string]bool{s.Root: true}
	for _, source := range sources {
		if source == nil {
			continue
		}
		size := source.Base().Size
		if size <= 0 {
			continue
		}
		if size > math.MaxInt64/6-ret.SourceBytes {
			return ret, fmt.Errorf("batch size exceeds disk reservation limits")
		}
		ret.SourceBytes += size
		if apply {
			directories[filepath.Dir(source.Base().Path)] = true
		}
	}
	ret.OutputBytes, ret.StagingBytes = 2*ret.SourceBytes, 2*ret.SourceBytes
	if apply {
		ret.BackupBytes = ret.SourceBytes
	}
	ret.RequiredBytes = ret.OutputBytes + ret.StagingBytes + ret.BackupBytes
	for directory := range directories {
		if err := checkDisk(directory, ret.RequiredBytes); err != nil {
			return ret, err
		}
	}
	return ret, nil
}

func (s Store) CreateTrial(ctx context.Context, fileID models.FileID, batch string, target TrialTarget, client Client, options Options, format Format, savings SavingsThresholds, signature string, retain bool) (trial *Trial, retErr error) {
	if err := savings.Validate(); err != nil {
		return nil, err
	}
	if retain && signature == "" {
		return nil, fmt.Errorf("saved trials require a versioned converter: update scripts/media_conversion_worker.py on the selected worker and restart it")
	}
	before, stat, err := s.conversionSource(ctx, fileID)
	if err != nil {
		return nil, err
	}
	c, err := s.Config()
	if err != nil {
		return nil, err
	}
	if retain && c.TrialCacheLimitBytes == 0 {
		return nil, fmt.Errorf("trial retention is disabled; set a positive trial cache limit in System settings")
	}
	if err := s.TrimTrials(false); err != nil {
		return nil, err
	}
	if before.Base().Size > math.MaxInt64/4 {
		return nil, fmt.Errorf("input exceeds disk reservation limits")
	}
	if err := checkDisk(filepath.Join(s.Root, "trials"), before.Base().Size*4); err != nil {
		return nil, err
	}
	sourceHash, err := sha256File(ctx, before.Base().Path)
	if err != nil {
		return nil, err
	}
	t := &Trial{Version: 1, ID: NewID(), Batch: batch, Target: target, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(c.TrialTTL()), Status: "encoding", Before: snapshot(before), Options: options, Format: format, Savings: savings, Backend: "local", WorkerIdentity: workerIdentity(client, signature), SourceSHA256: sourceHash, SourceModTime: stat.ModTime(), Notices: []string{"Verification checks decoding, dimensions and timing; it does not prove pixel equality or preservation of all embedded profiles/container metadata."}}
	if client.URL != "" {
		t.Backend = "remote"
	}
	if options.DropAudio {
		t.Notices = append(t.Notices, "Audio loss is permitted by the selected options.")
	}
	if options.AllowAlphaLoss {
		t.Notices = append(t.Notices, "Transparency loss is permitted by the selected options.")
	}
	trial = t
	if err := s.saveTrial(t); err != nil {
		return t, err
	}
	output := s.trialOutput(t)
	defer func() {
		if !retain {
			defer os.RemoveAll(s.trialDir(t.ID))
		}
		if retErr != nil {
			t.Status, t.Error = "failed", retErr.Error()
			if ctx.Err() != nil {
				t.Status = "cancelled"
			}
			_ = os.Remove(output)
			t.Retained = false
			_ = s.saveTrial(t)
		}
	}()
	t.Result, err = client.Convert(ctx, before.Base().Path, output, options)
	if err != nil {
		return t, err
	}
	outputStat, err := os.Lstat(output)
	if err != nil {
		return t, err
	}
	if !outputStat.Mode().IsRegular() || t.Result.Size <= 0 || t.Result.Size != outputStat.Size() || t.Result.Format != format.Extension || t.Result.Width <= 0 || t.Result.Height <= 0 || t.Result.Frames <= 0 || t.Result.Frames > 2147483647 {
		return t, fmt.Errorf("encoder returned invalid verified output metadata")
	}
	if signature != "" && t.Result.Signature != signature {
		return t, fmt.Errorf("converter version changed during encoding; run a new trial")
	}
	if t.Result.Frames > 1 {
		t.Notices = append(t.Notices, "Animated media: frame count/duration are checked, and per-frame timing/loops when exposed by the decoder. Still-image comparison cannot establish whole-animation equality.")
	}
	if hash, err := sha256File(ctx, before.Base().Path); err != nil || hash != sourceHash {
		return t, fmt.Errorf("source changed during trial; source kept")
	}
	if current, err := os.Stat(before.Base().Path); err != nil || current.Size() != stat.Size() || !current.ModTime().Equal(stat.ModTime()) {
		return t, fmt.Errorf("source identity changed during trial")
	}
	t.OutputSHA256, err = sha256File(ctx, output)
	if err != nil {
		return t, err
	}
	t.OutputMD5, err = MD5(output)
	if err != nil {
		return t, err
	}
	t.Verified = true
	t.Status, t.Error = "verified", savings.Reason(before.Base().Size, t.Result.Size)
	if t.Error != "" {
		t.Status = "skipped"
	}
	if retain {
		all, err := s.Trials()
		if err != nil {
			return t, err
		}
		if t.Result.Size > c.TrialCacheLimitBytes-SummarizeTrials(all).CacheBytes {
			return t, fmt.Errorf("trial cache limit would be exceeded; discard reviewed trials or increase the limit")
		}
		t.Retained = true
	} else {
		if err := os.Remove(output); err != nil {
			return t, err
		}
	}
	if err := ctx.Err(); err != nil {
		return t, err
	}
	return t, s.saveTrial(t)
}

// TrialFile serves only canonical manifest-owned paths. Source previews become
// unavailable when native identity/stat changes; full checksums are rechecked at apply.
func (s Store) TrialFile(ctx context.Context, id string, source bool) (string, error) {
	t, err := s.TrialRecord(id)
	if err != nil {
		return "", err
	}
	if !t.Retained || !t.ExpiresAt.After(time.Now()) || !t.Verified {
		return "", fmt.Errorf("trial output is unavailable or expired")
	}
	path := s.trialOutput(t)
	if source {
		current, err := s.Repository.Get(ctx, t.Before.File().Base().ID)
		if err != nil {
			return "", err
		}
		if !SameFile(current, t.Before.File()) {
			return "", fmt.Errorf("source changed; run a new trial")
		}
		path = t.Before.File().Base().Path
	}
	stat, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", fmt.Errorf("trial preview is not a regular file")
	}
	if source && (stat.Size() != t.Before.File().Base().Size || !stat.ModTime().Equal(t.SourceModTime)) {
		return "", fmt.Errorf("source changed; run a new trial")
	}
	return path, nil
}

func (s Store) ApplyTrial(ctx context.Context, id, batch string, client Client, options Options, savings SavingsThresholds, signature string) (record *Record, retErr error) {
	t, err := s.TrialRecord(id)
	if err != nil {
		return nil, err
	}
	if t.Status != "verified" || !t.Retained || !t.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("trial is not eligible, retained, or unexpired; run a new trial")
	}
	stored, err := json.Marshal(t.Options)
	if err != nil {
		return nil, fmt.Errorf("invalid saved trial options: %w", err)
	}
	expected, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("invalid apply options: %w", err)
	}
	if string(stored) != string(expected) || savings != t.Savings || signature == "" || t.WorkerIdentity != workerIdentity(client, signature) {
		return nil, fmt.Errorf("trial options, savings policy or converter version changed; run a new trial")
	}
	before, stat, err := s.conversionSource(ctx, t.Before.File().Base().ID)
	if err != nil {
		return nil, err
	}
	if !SameFile(before, t.Before.File()) || !stat.ModTime().Equal(t.SourceModTime) {
		return nil, fmt.Errorf("trial source identity changed; run a new trial")
	}
	if hash, err := sha256File(ctx, before.Base().Path); err != nil || hash != t.SourceSHA256 {
		return nil, fmt.Errorf("trial source content changed; run a new trial")
	}
	output := s.trialOutput(t)
	if hash, err := sha256File(ctx, output); err != nil || hash != t.OutputSHA256 {
		return nil, fmt.Errorf("trial output changed or is missing; run a new trial")
	}
	if reason := savings.Reason(before.Base().Size, t.Result.Size); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	if before.Base().Size > math.MaxInt64/4 || t.Result.Size > math.MaxInt64/4 {
		return nil, fmt.Errorf("input exceeds disk reservation limits")
	}
	if err := checkDisk(s.Root, before.Base().Size); err != nil {
		return nil, err
	}
	if err := checkDisk(filepath.Dir(before.Base().Path), t.Result.Size*2); err != nil {
		return nil, err
	}
	checksum, err := MD5(before.Base().Path)
	if err != nil {
		return nil, err
	}
	r := &Record{ID: NewID(), Batch: batch, CreatedAt: time.Now(), Status: "encoding", Before: snapshot(before), Options: options, Backend: t.Backend, Result: t.Result, Savings: &savings}
	r.Before.File().Base().SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: checksum})
	r.Staging = filepath.Join(filepath.Dir(before.Base().Path), ".stash-convert-"+r.ID)
	t.Status, t.RecordID = "applying", r.ID
	if err := s.saveTrial(t); err != nil {
		return nil, err
	}
	if err := s.save(r); err != nil {
		return nil, err
	}
	defer func() {
		_ = os.RemoveAll(r.Staging)
		if retErr != nil && r.Status == "encoding" {
			r.Status, r.Error = "failed", retErr.Error()
			_ = s.save(r)
		}
	}()
	if err := os.Mkdir(r.Staging, 0700); err != nil {
		return r, err
	}
	staged := filepath.Join(r.Staging, "output."+t.Format.Extension)
	if err := copyExclusive(output, staged, stat.Mode().Perm()); err != nil {
		return r, err
	}
	if hash, err := sha256File(ctx, staged); err != nil || hash != t.OutputSHA256 {
		return r, fmt.Errorf("trial staging checksum mismatch")
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	record, retErr = s.activate(ctx, r, before, staged, t.OutputMD5, stat)
	if retErr != nil {
		return record, retErr
	}
	t.Status, t.Error, t.Retained = "applied", "", false
	if err := s.saveTrial(t); err != nil {
		return record, err
	}
	return record, os.Remove(output)
}

func (s Store) DiscardTrial(id string) error {
	t, err := s.TrialRecord(id)
	if err != nil {
		return err
	}
	if t.Status == "encoding" || t.Status == "applying" {
		return fmt.Errorf("wait for or cancel the active trial job")
	}
	return os.RemoveAll(s.trialDir(id))
}

// Startup/job recovery reconciles interrupted apply with the existing conversion
// journal. Outputs never become the only retained original during recovery.
func (s Store) TrimTrials(recoverInterrupted bool) error {
	c, err := s.Config()
	if err != nil {
		return err
	}
	trials, err := s.Trials()
	if err != nil {
		return err
	}
	for _, t := range trials {
		if recoverInterrupted && t.Status == "encoding" {
			t.Status, t.Error = "failed", "trial interrupted; source kept"
			_ = os.Remove(s.trialOutput(t))
			t.Retained = false
		}
		if recoverInterrupted && t.Status == "applying" {
			r, err := s.Record(t.RecordID)
			switch {
			case err == nil && r.Status == "complete":
				t.Status, t.Retained = "applied", false
				_ = os.Remove(s.trialOutput(t))
			case err == nil && r.Status != "failed":
				return fmt.Errorf("trial %s still requires activation recovery", t.ID)
			default:
				t.Status, t.RecordID = "verified", ""
			}
		}
		if !t.ExpiresAt.After(time.Now()) && t.Status != "encoding" && t.Status != "applying" {
			if err := os.RemoveAll(s.trialDir(t.ID)); err != nil {
				return err
			}
			continue
		}
		if recoverInterrupted {
			if err := s.saveTrial(t); err != nil {
				return err
			}
		}
	}
	trials, err = s.Trials()
	if err != nil {
		return err
	}
	used := SummarizeTrials(trials).CacheBytes
	for _, t := range trials {
		if used <= c.TrialCacheLimitBytes {
			break
		}
		if !t.Retained || t.Status == "applying" {
			continue
		}
		if err := os.Remove(s.trialOutput(t)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		t.Retained, t.Status, t.Error = false, "expired", "trial cache limit exceeded; run a new trial"
		used -= t.Result.Size
		if err := s.saveTrial(t); err != nil {
			return err
		}
	}
	return nil
}
