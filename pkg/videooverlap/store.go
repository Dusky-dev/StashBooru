package videooverlap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// The rebuildable sidecar belongs to StashBooru. It adds no native schema or
// relationships and contains no media payloads or remote credentials.
type Store struct{ Root string }

func (s Store) open(ctx context.Context) (*sql.DB, error) {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return nil, err
	}
	index, err := filepath.Abs(filepath.Join(s.Root, "index.sqlite"))
	if err != nil {
		return nil, err
	}
	index = filepath.ToSlash(index)
	if !strings.HasPrefix(index, "/") {
		index = "/" + index // A Windows drive needs an absolute file URI path.
	}
	u := url.URL{Scheme: "file", Path: index, RawQuery: "_foreign_keys=on&_busy_timeout=10000&_journal_mode=WAL"}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var version int
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		_ = db.Close()
		return nil, err
	}
	if version > 1 {
		_ = db.Close()
		return nil, fmt.Errorf("video index version %d is newer than supported version 1", version)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS videos (
		id INTEGER PRIMARY KEY, sha256 TEXT NOT NULL, algorithm TEXT NOT NULL, signature TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS video_digest ON videos(sha256);
		CREATE TABLE IF NOT EXISTS bands (
		scene_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
		frame INTEGER NOT NULL, band INTEGER NOT NULL, value INTEGER NOT NULL,
		PRIMARY KEY(scene_id,frame,band));
		CREATE INDEX IF NOT EXISTS band_lookup ON bands(band,value,scene_id);
		CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), value TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY, action TEXT NOT NULL, reference INTEGER NOT NULL,
		updated INTEGER NOT NULL, body TEXT NOT NULL);
		CREATE INDEX IF NOT EXISTS job_reference ON jobs(reference,updated DESC);
		PRAGMA user_version=1;`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func NewID() string {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

func (s Signature) Key() string {
	data, err := json.Marshal(struct {
		Source                  Source
		SHA, Algorithm, Decoder string
		Config                  Config
	}{s.Source, s.SHA256, s.Algorithm, s.Decoder, s.Config})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (s Store) Config(ctx context.Context) (Config, error) {
	ret := DefaultConfig()
	db, err := s.open(ctx)
	if err != nil {
		return ret, err
	}
	defer db.Close()
	var data string
	err = db.QueryRowContext(ctx, "SELECT value FROM settings WHERE id=1").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return ret, nil
	}
	if err != nil {
		return ret, err
	}
	err = json.Unmarshal([]byte(data), &ret)
	return ret, err
}

func (s Store) Configure(ctx context.Context, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	db, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "INSERT INTO settings(id,value) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET value=excluded.value", string(data))
	return err
}

func (s Store) Put(ctx context.Context, signature Signature) error {
	if signature.Source.SceneID <= 0 || signature.Source.FileID <= 0 || signature.Algorithm != Algorithm || !validDigest(signature.SHA256) || !validDigest(signature.Decoder) || len(signature.Frames) == 0 || len(signature.Frames) > MaxFrames {
		return fmt.Errorf("invalid video signature")
	}
	data, err := json.Marshal(signature)
	if err != nil {
		return err
	}
	db, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, "DELETE FROM bands WHERE scene_id=?", signature.Source.SceneID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO videos(id,sha256,algorithm,signature) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET sha256=excluded.sha256,algorithm=excluded.algorithm,signature=excluded.signature", signature.Source.SceneID, signature.SHA256, signature.Algorithm, string(data))
	if err != nil {
		return err
	}
	statement, err := tx.PrepareContext(ctx, "INSERT INTO bands(scene_id,frame,band,value) VALUES(?,?,?,?)")
	if err != nil {
		return err
	}
	defer statement.Close()
	for i, f := range signature.Frames {
		if f.Weight < .4 {
			continue
		}
		for band, value := range Bands(f.Hash) {
			if _, err = statement.ExecContext(ctx, signature.Source.SceneID, i, band, value); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s Store) Get(ctx context.Context, id int) (*Signature, error) {
	db, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data string
	err = db.QueryRowContext(ctx, "SELECT signature FROM videos WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ret Signature
	err = json.Unmarshal([]byte(data), &ret)
	return &ret, err
}

func (s Store) Count(ctx context.Context) (int, error) {
	db, err := s.open(ctx)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM videos WHERE algorithm=?", Algorithm).Scan(&n)
	return n, err
}

// Candidates uses inverted pHash bands, not a video-pair scan or duration gate.
// Posting lists >10,000 frames are omitted and explicitly reported. Exact file
// candidates use their own digest index and are never lost through this limit.
func (s Store) Candidates(ctx context.Context, reference Signature, limit int) (CandidateReport, error) {
	ret := CandidateReport{IDs: []int{}}
	if limit < 1 || limit > 1000 {
		return ret, fmt.Errorf("invalid candidate limit")
	}
	db, err := s.open(ctx)
	if err != nil {
		return ret, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ret, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, "CREATE TEMP TABLE probes (band INTEGER,value INTEGER,PRIMARY KEY(band,value)); CREATE TEMP TABLE candidate_counts (id INTEGER PRIMARY KEY,hits INTEGER,exact INTEGER)")
	if err != nil {
		return ret, err
	}
	statement, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO probes(band,value) VALUES(?,?)")
	if err != nil {
		return ret, err
	}
	defer statement.Close()
	for _, f := range reference.Frames {
		if f.Weight < .4 {
			continue
		}
		for band, value := range Bands(f.Hash) {
			if _, err = statement.ExecContext(ctx, band, value); err != nil {
				_ = statement.Close()
				return ret, err
			}
		}
	}
	_ = statement.Close()
	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE rare_probes AS
		SELECT p.band,p.value FROM probes p JOIN bands b INDEXED BY band_lookup
		ON b.band=p.band AND b.value=p.value GROUP BY p.band,p.value HAVING COUNT(*)<=10000;
		CREATE UNIQUE INDEX rare_probe_key ON rare_probes(band,value);`)
	if err != nil {
		return ret, err
	}
	err = tx.QueryRowContext(ctx, "SELECT (SELECT COUNT(*) FROM probes)-(SELECT COUNT(*) FROM rare_probes)").Scan(&ret.OmittedCommonBands)
	if err != nil {
		return ret, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO candidate_counts(id,hits,exact)
		SELECT b.scene_id,COUNT(*),0 FROM rare_probes p JOIN bands b INDEXED BY band_lookup
		ON b.band=p.band AND b.value=p.value JOIN videos v ON v.id=b.scene_id
		WHERE v.algorithm=? AND b.scene_id<>? GROUP BY b.scene_id`, Algorithm, reference.Source.SceneID)
	if err != nil {
		return ret, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO candidate_counts(id,hits,exact)
		SELECT id,0,1 FROM videos WHERE sha256=? AND id<>?
		ON CONFLICT(id) DO UPDATE SET exact=1`, reference.SHA256, reference.Source.SceneID)
	if err != nil {
		return ret, err
	}
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM candidate_counts").Scan(&ret.Total)
	if err != nil {
		return ret, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id FROM candidate_counts ORDER BY exact DESC,hits DESC,id ASC LIMIT ?", limit)
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return ret, err
		}
		ret.IDs = append(ret.IDs, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return ret, err
	}
	ret.Limited = ret.Total > len(ret.IDs) || ret.OmittedCommonBands > 0
	return ret, tx.Commit()
}

type JobItem struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Job struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	NativeID  int            `json:"jobID"`
	Session   string         `json:"session"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
	Config    Config         `json:"config"`
	Backend   string         `json:"backend"`
	Targets   []int          `json:"targets,omitempty"`
	All       bool           `json:"all"`
	Cursor    int            `json:"cursor"`
	UpperID   int            `json:"upperID"`
	FailedIDs []int          `json:"failedIDs"`
	Force     bool           `json:"force"`
	Total     int            `json:"total"`
	Processed int            `json:"processed"`
	Indexed   int            `json:"indexed"`
	Skipped   int            `json:"skipped"`
	Failed    int            `json:"failed"`
	Items     []JobItem      `json:"items"`
	Review    *Review        `json:"review,omitempty"`
	Versions  map[int]string `json:"versions,omitempty"`
	Updated   time.Time      `json:"updated"`
}

func (j *Job) AddItem(item JobItem) {
	j.Processed++
	switch item.Status {
	case "indexed":
		j.Indexed++
	case "current":
		j.Skipped++
	case "failed":
		j.Failed++
		j.FailedIDs = append(j.FailedIDs, item.ID)
	}
	if len(item.Error) > 2048 {
		item.Error = item.Error[:2048]
	}
	j.Items = append(j.Items, item)
	if len(j.Items) > 50 {
		j.Items = j.Items[len(j.Items)-50:]
	}
}

func (s Store) SaveJob(ctx context.Context, j *Job) error {
	j.Updated = time.Now()
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	db, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	reference := 0
	if j.Review != nil {
		reference = j.Review.Reference
	}
	_, err = db.ExecContext(ctx, "INSERT INTO jobs(id,action,reference,updated,body) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET updated=excluded.updated,body=excluded.body", j.ID, j.Action, reference, j.Updated.UnixNano(), string(data))
	return err
}

func (s Store) LoadJob(ctx context.Context, id string) (*Job, error) {
	db, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var data string
	err = db.QueryRowContext(ctx, "SELECT body FROM jobs WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ret Job
	err = json.Unmarshal([]byte(data), &ret)
	return &ret, err
}

func (s Store) LatestJobs(ctx context.Context, reference int) ([]Job, error) {
	db, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT body FROM jobs WHERE (?=0 OR reference=?) ORDER BY updated DESC LIMIT 8", reference, reference)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ret := []Job{}
	for rows.Next() {
		var data string
		var j Job
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &j); err != nil {
			return nil, err
		}
		ret = append(ret, j)
	}
	return ret, rows.Err()
}

func SortMatches(matches []Match) {
	rank := map[string]int{"exact-file": 0, "near-complete-visual": 1, "contained-clip": 2, "compilation-segments": 3, "partial-overlap": 4}
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if rank[a.Class] != rank[b.Class] {
			return rank[a.Class] < rank[b.Class]
		}
		if a.CoverageA != b.CoverageA {
			return a.CoverageA > b.CoverageA
		}
		if a.CoverageB != b.CoverageB {
			return a.CoverageB > b.CoverageB
		}
		return a.B < b.B
	})
}
