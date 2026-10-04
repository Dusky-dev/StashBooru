package imagerestore

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record is a preview journal, then permanent provenance for a registered derivative.
// Credentials and worker-owned model paths are never persisted.
type Record struct {
	ID                  string       `json:"id"`
	CreatedAt           time.Time    `json:"createdAt"`
	ExpiresAt           time.Time    `json:"expiresAt"`
	Status              string       `json:"status"`
	Error               string       `json:"error,omitempty"`
	Notice              string       `json:"notice,omitempty"`
	JobID               int          `json:"jobID"`
	ServerSession       string       `json:"serverSession"`
	SourceImageID       int          `json:"sourceImageID"`
	SourceFileID        int          `json:"sourceFileID"`
	SourcePath          string       `json:"sourcePath"`
	SourceSize          int64        `json:"sourceSize"`
	SourceSHA256        string       `json:"sourceSHA256"`
	SourceMD5           string       `json:"sourceMD5"`
	CanonicalSHA256     string       `json:"canonicalSHA256"`
	MaskSHA256          string       `json:"maskSHA256,omitempty"`
	EffectiveMaskSHA256 string       `json:"effectiveMaskSHA256,omitempty"`
	ReferenceSHA256     string       `json:"referenceSHA256,omitempty"`
	Backend             string       `json:"backend"`
	Capabilities        Capabilities `json:"capabilities"`
	Options             Options      `json:"options"`
	Feather             int          `json:"feather"`
	Receipt             Receipt      `json:"receipt"`
	Destination         string       `json:"destination,omitempty"`
	DerivedImageID      int          `json:"derivedImageID,omitempty"`
}

type Store struct{ Root string }

func (s Store) Directory(id string) (string, error) {
	b, err := hex.DecodeString(id)
	if err != nil || len(b) != 16 || id != hex.EncodeToString(b) {
		return "", fmt.Errorf("invalid restoration session ID")
	}
	return filepath.Join(s.Root, id), nil
}
func (s Store) Load(id string) (*Record, error) {
	dir, err := s.Directory(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "record.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var record Record
	if err := json.NewDecoder(f).Decode(&record); err != nil {
		return nil, err
	}
	if record.ID != id {
		return nil, fmt.Errorf("restoration journal identity mismatch")
	}
	return &record, nil
}
func (s Store) Save(record *Record) error {
	dir, err := s.Directory(record.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".journal-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := json.NewEncoder(f).Encode(record); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "record.json"))
}
func (s Store) CleanTemporary(id string) error {
	dir, err := s.Directory(id)
	if err != nil {
		return err
	}
	for _, name := range []string{"input.zip", "result.zip", "input-source", "source.png", "output.png", "mask.png", "effective-mask.png", "reference"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Prune never removes saved provenance. Running/queued records are handled by the owner.
func (s Store) Prune(now time.Time) (int64, int, error) {
	entries, err := os.ReadDir(s.Root)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	var bytes int64
	active := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := s.Load(entry.Name())
		if err != nil {
			return 0, 0, err
		}
		dir, _ := s.Directory(record.ID)
		if record.Status != "saved" && record.Status != "saving" && record.Status != "queued" && record.Status != "running" && !now.Before(record.ExpiresAt) {
			if err := os.RemoveAll(dir); err != nil {
				return 0, 0, err
			}
			continue
		}
		if record.Status == "saved" {
			continue
		}
		active++
		files, err := os.ReadDir(dir)
		if err != nil {
			return 0, 0, err
		}
		for _, file := range files {
			info, err := file.Info()
			if err != nil {
				return 0, 0, err
			}
			if !info.IsDir() {
				bytes += info.Size()
			}
		}
	}
	return bytes, active, nil
}
