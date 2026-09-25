package mediaconvert

import (
	"context"
	"crypto/md5" //nolint:gosec // Stash content identifier.
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/hash/oshash"
	"github.com/stashapp/stash/pkg/models"
)

// Repository.Swap must compare the current file with before and update only the
// file record, atomically. Media relationships and user metadata are untouched.
type Repository interface {
	Get(context.Context, models.FileID) (models.File, error)
	Swap(context.Context, models.File, models.File) error
}

type Snapshot struct {
	Image *models.ImageFile `json:"image,omitempty"`
	Video *models.VideoFile `json:"video,omitempty"`
}

func snapshot(f models.File) Snapshot {
	clone := f.Clone()
	clone.Base().Fingerprints = append(models.Fingerprints(nil), f.Base().Fingerprints...)
	switch v := clone.(type) {
	case *models.ImageFile:
		return Snapshot{Image: v}
	case *models.VideoFile:
		return Snapshot{Video: v}
	default:
		return Snapshot{}
	}
}

func (s Snapshot) File() models.File {
	var f models.File
	switch {
	case s.Video != nil:
		f = s.Video
	case s.Image != nil:
		f = s.Image
	default:
		return nil
	}
	// UseNumber on journal reads avoids rounding 64-bit perceptual hashes.
	for i, fp := range f.Base().Fingerprints {
		if n, ok := fp.Fingerprint.(json.Number); ok {
			if v, err := n.Int64(); err == nil {
				f.Base().Fingerprints[i].Fingerprint = v
			}
		}
	}
	return f
}

type Record struct {
	ID        string    `json:"id"`
	Batch     string    `json:"batch"`
	CreatedAt time.Time `json:"createdAt"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Before    Snapshot  `json:"before"`
	After     Snapshot  `json:"after"`
	Options   Options   `json:"options"`
	Backend   string    `json:"backend"`
	Result    Result    `json:"result"`
	Cached    bool      `json:"cached"`
	Staging   string    `json:"staging,omitempty"`
}

type Config struct {
	CacheLimitBytes  int64                       `json:"cacheLimitBytes"`
	FormatDefaults   map[string]string           `json:"formatDefaults"`
	EncodingDefaults map[string]EncodingDefaults `json:"encodingDefaults"`
	Backend          string                      `json:"backend"`
}

type Stats struct {
	Files             int     `json:"files"`
	Converted         int     `json:"converted"`
	OriginalBytes     int64   `json:"originalBytes"`
	ConvertedBytes    int64   `json:"convertedBytes"`
	SavedBytes        int64   `json:"savedBytes"`
	AverageSavedBytes int64   `json:"averageSavedBytes"`
	SavedPercent      float64 `json:"savedPercent"`
	CacheBytes        int64   `json:"cacheBytes"`
	NetSavedBytes     int64   `json:"netSavedBytes"`
	LargerFiles       int     `json:"largerFiles"`
}

func Summarize(records []*Record) Stats {
	var ret Stats
	type total struct {
		original, saved int64
		first           time.Time
	}
	files := map[models.FileID]total{}
	for _, r := range records {
		if r.Cached && r.Before.File() != nil {
			ret.CacheBytes += r.Before.File().Base().Size
		}
		if r.Status != "complete" {
			continue
		}
		ret.Converted++
		before, after := r.Before.File().Base().Size, r.After.File().Base().Size
		id := r.Before.File().Base().ID
		v, exists := files[id]
		if !exists || r.CreatedAt.Before(v.first) {
			v.original, v.first = before, r.CreatedAt
		}
		v.saved += before - after
		files[id] = v
		if after > before {
			ret.LargerFiles++
		}
	}
	for _, v := range files {
		ret.OriginalBytes += v.original
		ret.SavedBytes += v.saved
	}
	ret.Files = len(files)
	ret.ConvertedBytes = ret.OriginalBytes - ret.SavedBytes
	ret.NetSavedBytes = ret.SavedBytes - ret.CacheBytes
	if ret.Files > 0 {
		ret.AverageSavedBytes = ret.SavedBytes / int64(ret.Files)
	}
	if ret.OriginalBytes > 0 {
		ret.SavedPercent = 100 * float64(ret.SavedBytes) / float64(ret.OriginalBytes)
	}
	return ret
}

type Store struct {
	Root       string
	Repository Repository
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16
}

func (s Store) backup(id string) string { return filepath.Join(s.Root, id+".original") }

func atomicJSON(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".conversion-json-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(value); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	syncDir(filepath.Dir(path))
	return nil
}

func syncDir(path string) {
	if f, err := os.Open(path); err == nil {
		_ = f.Sync()
		f.Close()
	}
}

func readJSON(path string, value interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.UseNumber()
	return d.Decode(value)
}

func (s Store) Config() (Config, error) {
	c := Config{CacheLimitBytes: 20 * 1024 * 1024 * 1024}
	err := readJSON(filepath.Join(s.Root, "config.json"), &c)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	if c.FormatDefaults == nil {
		c.FormatDefaults = DefaultFormatDefaults()
	}
	if c.Backend == "" {
		c.Backend = "auto"
	}
	if c.EncodingDefaults == nil {
		c.EncodingDefaults = map[string]EncodingDefaults{}
	}
	if c.CacheLimitBytes < 0 {
		return c, fmt.Errorf("invalid negative restore cache limit")
	}
	return c, err
}

func (s Store) Configure(c Config) error {
	if err := ValidateEncodingDefaults(c.EncodingDefaults, c.Backend); err != nil {
		return err
	}
	if c.CacheLimitBytes < 0 {
		return fmt.Errorf("cache size cannot be negative")
	}
	if c.FormatDefaults == nil {
		c.FormatDefaults = DefaultFormatDefaults()
	}
	if err := ValidateFormatDefaults(c.FormatDefaults); err != nil {
		return err
	}
	return atomicJSON(filepath.Join(s.Root, "config.json"), c)
}

func (s Store) save(r *Record) error { return atomicJSON(filepath.Join(s.Root, r.ID+".json"), r) }

func (s Store) Record(id string) (*Record, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid conversion ID")
	}
	var r Record
	if err := readJSON(filepath.Join(s.Root, id+".json"), &r); err != nil {
		return nil, err
	}
	before := r.Before.File()
	if r.ID != id || before == nil || before.Base().ID <= 0 || before.Base().Path == "" {
		return nil, fmt.Errorf("invalid conversion journal %s", id)
	}
	if r.Staging != "" && r.Staging != filepath.Join(filepath.Dir(before.Base().Path), ".stash-convert-"+id) {
		return nil, fmt.Errorf("invalid staging path in conversion journal %s", id)
	}
	switch r.Status {
	case "encoding", "failed", "skipped":
	case "prepared", "committed", "complete", "restoring", "restored":
		after := r.After.File()
		if after == nil || after.Base().ID != before.Base().ID || after.Base().Path == before.Base().Path || filepath.Dir(after.Base().Path) != filepath.Dir(before.Base().Path) {
			return nil, fmt.Errorf("invalid output identity in conversion journal %s", id)
		}
	default:
		return nil, fmt.Errorf("invalid status in conversion journal %s", id)
	}
	return &r, nil
}

func (s Store) History() ([]*Record, error) {
	paths, err := filepath.Glob(filepath.Join(s.Root, "*.json"))
	if err != nil {
		return nil, err
	}
	ret := []*Record{}
	for _, p := range paths {
		id := strings.TrimSuffix(filepath.Base(p), ".json")
		if !validID(id) {
			continue
		}
		r, err := s.Record(id)
		if err != nil {
			return nil, err
		}
		ret = append(ret, r)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].CreatedAt.Before(ret[j].CreatedAt) })
	return ret, nil
}

func MD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New() //nolint:gosec // Stash uses MD5 for media content identity.
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyExclusive(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(destination)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	ok = true
	syncDir(filepath.Dir(destination))
	return nil
}

func matches(path, checksum string) bool {
	stat, err := os.Lstat(path)
	if err != nil || !stat.Mode().IsRegular() {
		return false
	}
	actual, err := MD5(path)
	return err == nil && checksum != "" && actual == checksum
}

func removeMatching(path, checksum string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if !matches(path, checksum) {
		return fmt.Errorf("file changed; refusing to remove %s", path)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	syncDir(filepath.Dir(path))
	return nil
}

func SameFile(a, b models.File) bool {
	return a != nil && b != nil && a.Base().ID == b.Base().ID && a.Base().Path == b.Base().Path &&
		a.Base().Size == b.Base().Size && a.Base().Fingerprints.Equals(b.Base().Fingerprints)
}

func convertedFile(before models.File, path string, result Result, hash string) (models.File, error) {
	b := *before.Base()
	b.Path, b.Basename, b.Size = path, filepath.Base(path), result.Size
	b.FrameCount = int(result.Frames)
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	b.ModTime, b.UpdatedAt = stat.ModTime(), time.Now()
	b.Fingerprints = append(models.Fingerprints(nil), b.Fingerprints...)
	for _, fp := range before.Base().Fingerprints {
		if !strings.HasPrefix(fp.Type, "source_") && b.Fingerprints.For("source_"+fp.Type) == nil {
			b.SetFingerprint(models.Fingerprint{Type: "source_" + fp.Type, Fingerprint: fp.Value()})
		}
	}
	b.Fingerprints = b.Fingerprints.Remove(models.FingerprintTypePhash).Remove(models.FingerprintTypeOshash)
	b.SetFingerprint(models.Fingerprint{Type: models.FingerprintTypeMD5, Fingerprint: hash})
	if v, err := oshash.FromFilePath(path); err == nil {
		b.SetFingerprint(models.Fingerprint{Type: models.FingerprintTypeOshash, Fingerprint: v})
	}
	// Older remote workers can return FFprobe's nominal rate. For animated
	// images, derive the average from the verified complete presentation span.
	animationRate := 0.0
	if result.Frames > 1 && result.Duration > 0 {
		animationRate = float64(result.Frames) / result.Duration
	}
	if result.VideoCodec == "gif" && animationRate > 0 {
		result.FrameRate = animationRate
	}
	video := result.Format == "mp4" || result.Format == "mkv" || result.Format == "webm" || result.Format == "mov"
	if video || result.VideoCodec == "gif" {
		return &models.VideoFile{BaseFile: &b, Format: result.Format, Width: result.Width, Height: result.Height,
			Duration: result.Duration, VideoCodec: result.VideoCodec, AudioCodec: result.AudioCodec,
			FrameRate: result.FrameRate, BitRate: result.BitRate}, nil
	}
	return &models.ImageFile{BaseFile: &b, Format: result.VideoCodec, Width: result.Width, Height: result.Height, FrameRate: animationRate}, nil
}

func (s Store) Convert(ctx context.Context, fileID models.FileID, batch string, client Client, options Options, format Format) (record *Record, retErr error) {
	before, err := s.Repository.Get(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if before == nil || snapshot(before).File() == nil {
		return nil, fmt.Errorf("media file not found")
	}
	base := before.Base()
	if base.ZipFileID != nil {
		return nil, fmt.Errorf("extract archived media before converting")
	}
	stat, err := os.Lstat(base.Path)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("only regular files can be converted")
	}
	if v, ok := before.(*models.VideoFile); ok && v.Interactive {
		return nil, fmt.Errorf("interactive video requires preserving its sidecars; conversion is unavailable")
	}
	checksum, err := MD5(base.Path)
	if err != nil {
		return nil, err
	}
	if old := base.Fingerprints.GetString(models.FingerprintTypeMD5); old != "" && old != checksum {
		return nil, fmt.Errorf("source changed; rescan before converting")
	}
	if base.Size != stat.Size() {
		return nil, fmt.Errorf("source size changed; rescan before converting")
	}
	// Do not alter the repository snapshot used by the compare-and-swap.
	before = before.Clone()
	before.Base().Fingerprints = append(models.Fingerprints(nil), base.Fingerprints...)
	record = &Record{ID: NewID(), Batch: batch, CreatedAt: time.Now(), Status: "encoding", Before: snapshot(before), Options: options, Backend: "local"}
	record.Before.File().Base().SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: checksum})
	if client.URL != "" {
		record.Backend = "remote"
	}
	r := record
	stageDir := filepath.Join(filepath.Dir(base.Path), ".stash-convert-"+r.ID)
	r.Staging = stageDir
	if err := s.save(r); err != nil {
		return r, err
	}
	defer func() {
		os.RemoveAll(stageDir)
		if retErr != nil && r.Status == "encoding" {
			r.Status, r.Error = "failed", retErr.Error()
			_ = s.save(r)
		}
	}()
	if err := os.Mkdir(stageDir, 0700); err != nil {
		return r, err
	}
	staged := filepath.Join(stageDir, "output."+format.Extension)
	result, err := client.Convert(ctx, base.Path, staged, options)
	if err != nil {
		return r, err
	}
	r.Result = result
	outputStat, err := os.Stat(staged)
	if err != nil {
		return r, err
	}
	if result.Size <= 0 || result.Size != outputStat.Size() || result.Format != format.Extension || result.Width <= 0 || result.Height <= 0 || result.Frames <= 0 || result.Frames > 2147483647 {
		return r, fmt.Errorf("encoder returned invalid output metadata")
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if !options.AllowLarger && result.Size >= base.Size {
		r.Status = "skipped"
		r.Error = "output was not smaller; source kept"
		return r, s.save(r)
	}
	outputHash, err := MD5(staged)
	if err != nil {
		return r, err
	}
	if !matches(base.Path, checksum) {
		return r, fmt.Errorf("source changed during encoding; source kept")
	}
	if err := copyExclusive(base.Path, s.backup(r.ID), stat.Mode().Perm()); err != nil {
		return r, err
	}
	if !matches(s.backup(r.ID), checksum) {
		_ = os.Remove(s.backup(r.ID))
		return r, fmt.Errorf("backup verification failed")
	}
	r.Cached = true
	// Always use a fresh basename. Never overwrite a sibling or mutate an open input.
	destination := strings.TrimSuffix(base.Path, filepath.Ext(base.Path)) + ".converted-" + r.ID[:8] + "." + format.Extension
	after, err := convertedFile(before, staged, result, outputHash)
	if err != nil {
		return r, err
	}
	if after.Base().Fingerprints.GetString("source_md5") == "" {
		after.Base().SetFingerprint(models.Fingerprint{Type: "source_md5", Fingerprint: checksum})
	}
	after.Base().Path, after.Base().Basename = destination, filepath.Base(destination)
	r.After = snapshot(after)
	// Record the actual input MD5 even if scanning had only calculated OSHash.
	r.Before.File().Base().SetFingerprint(models.Fingerprint{Type: "md5", Fingerprint: checksum})
	r.Status = "prepared"
	if err := s.save(r); err != nil {
		return r, err
	}
	if err := copyExclusive(staged, destination, stat.Mode().Perm()); err != nil {
		return r, err
	}
	if !matches(destination, outputHash) {
		return r, fmt.Errorf("published output verification failed; source retained")
	}
	if published, err := os.Stat(destination); err == nil {
		after.Base().ModTime = published.ModTime()
	} else {
		return r, err
	}
	r.After = snapshot(after)
	if err := s.save(r); err != nil {
		return r, err
	}
	// Commit remains recoverable after cancellation or a process crash.
	if err := s.Repository.Swap(context.WithoutCancel(ctx), before, after); err != nil {
		return r, err
	}
	r.Status = "committed"
	if err := s.save(r); err != nil {
		return r, err
	}
	if err := removeMatching(base.Path, checksum); err != nil {
		return r, err
	}
	r.Status = "complete"
	if err := s.save(r); err != nil {
		return r, err
	}
	return r, s.Trim()
}

// Recover is run in the job queue before any conversion, restore or eviction.
// Ambiguous external edits stop recovery, retaining both files for review.
func (s Store) Recover(ctx context.Context) error {
	records, err := s.History()
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Status == "encoding" {
			r.Status, r.Error = "failed", "conversion interrupted before activation"
			if _, err := os.Stat(s.backup(r.ID)); err == nil {
				// A crash may have happened during the backup copy, before Cached was persisted.
				if !matches(r.Before.File().Base().Path, r.Before.File().Base().Fingerprints.GetString("md5")) {
					return fmt.Errorf("conversion %s needs recovery: original path changed; cache retained", r.ID)
				}
				if err := os.Remove(s.backup(r.ID)); err != nil {
					return err
				}
				r.Cached = false
			}
			if r.Staging != "" {
				_ = os.RemoveAll(r.Staging)
			}
			if err := s.save(r); err != nil {
				return err
			}
			continue
		}
		if r.Status != "prepared" && r.Status != "committed" && r.Status != "restoring" {
			continue
		}
		before, after := r.Before.File(), r.After.File()
		current, err := s.Repository.Get(ctx, before.Base().ID)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("conversion %s needs recovery: media entry was removed", r.ID)
		}
		isAfter := current.Base().Path == after.Base().Path && current.Base().Fingerprints.GetString("md5") == after.Base().Fingerprints.GetString("md5")
		isBefore := current.Base().Path == before.Base().Path
		switch {
		case r.Status == "restoring":
			switch {
			case isBefore && matches(before.Base().Path, before.Base().Fingerprints.GetString("md5")):
				if err := removeMatching(after.Base().Path, after.Base().Fingerprints.GetString("md5")); err != nil {
					return err
				}
				r.Status = "restored"
			case isAfter:
				if err := removeMatching(before.Base().Path, before.Base().Fingerprints.GetString("md5")); err != nil {
					return err
				}
				r.Status = "complete"
			default:
				return fmt.Errorf("conversion %s needs manual recovery; files changed", r.ID)
			}
		case isAfter && matches(after.Base().Path, after.Base().Fingerprints.GetString("md5")):
			if err := removeMatching(before.Base().Path, before.Base().Fingerprints.GetString("md5")); err != nil {
				return err
			}
			r.Status = "complete"
		case isBefore && matches(before.Base().Path, before.Base().Fingerprints.GetString("md5")):
			if err := removeMatching(after.Base().Path, after.Base().Fingerprints.GetString("md5")); err != nil {
				return err
			}
			r.Status, r.Error = "failed", "activation interrupted; source retained"
		default:
			return fmt.Errorf("conversion %s needs manual recovery; files changed", r.ID)
		}
		if r.Status == "failed" || r.Status == "restored" {
			if err := os.Remove(s.backup(r.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			r.Cached = false
		}
		if r.Staging != "" {
			_ = os.RemoveAll(r.Staging)
		}
		if err := s.save(r); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) Trim() error {
	c, err := s.Config()
	if err != nil {
		return err
	}
	records, err := s.History()
	if err != nil {
		return err
	}
	used := Summarize(records).CacheBytes
	for _, r := range records {
		if !r.Cached || (r.Status != "complete" && r.Status != "restored" && r.Status != "failed") {
			continue
		}
		if used <= c.CacheLimitBytes && r.Status == "complete" {
			continue
		}
		if err := os.Remove(s.backup(r.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		r.Cached = false
		if err := s.save(r); err != nil {
			return err
		}
		used -= r.Before.File().Base().Size
	}
	return nil
}

func (s Store) Restore(ctx context.Context, id string) error {
	r, err := s.Record(id)
	if err != nil {
		return err
	}
	if r.Status != "complete" || !r.Cached {
		return fmt.Errorf("original is no longer in the restore cache")
	}
	before, after := r.Before.File(), r.After.File()
	current, err := s.Repository.Get(ctx, before.Base().ID)
	if err != nil {
		return err
	}
	// New pHashes may have been generated since conversion; use actual content identity.
	if current == nil || current.Base().Path != after.Base().Path || !matches(after.Base().Path, after.Base().Fingerprints.GetString("md5")) {
		return fmt.Errorf("converted file is no longer active or has changed; restore the latest conversion first")
	}
	if _, err := os.Lstat(before.Base().Path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("original destination is occupied; no files changed")
	}
	if !matches(s.backup(id), before.Base().Fingerprints.GetString("md5")) {
		return fmt.Errorf("cached original is missing or corrupted")
	}
	r.Status = "restoring"
	if err := s.save(r); err != nil {
		return err
	}
	stat, err := os.Stat(s.backup(id))
	if err != nil {
		return err
	}
	if err := copyExclusive(s.backup(id), before.Base().Path, stat.Mode().Perm()); err != nil {
		return err
	}
	_ = os.Chtimes(before.Base().Path, before.Base().ModTime, before.Base().ModTime)
	if err := s.Repository.Swap(context.WithoutCancel(ctx), current, before); err != nil {
		return err
	}
	if err := removeMatching(after.Base().Path, after.Base().Fingerprints.GetString("md5")); err != nil {
		return err
	}
	r.Status = "restored"
	if err := s.save(r); err != nil {
		return err
	}
	return s.Trim()
}
