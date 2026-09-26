package mediaconvert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (s Store) convertedBackup(id string) string {
	return filepath.Join(s.Root, id+".converted")
}

// ConvertedCached reports whether the inactive converted representation is
// retained and still matches the conversion journal.
func (s Store) ConvertedCached(r *Record) bool {
	if r == nil || r.After.File() == nil {
		return false
	}
	return matches(s.convertedBackup(r.ID), r.After.File().Base().Fingerprints.GetString("md5"))
}

func ensureCachedCopy(source, destination, checksum string, mode os.FileMode) error {
	if matches(destination, checksum) {
		return nil
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("cached version is occupied or corrupted: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := copyExclusive(source, destination, mode); err != nil {
		return err
	}
	if !matches(destination, checksum) {
		_ = os.Remove(destination)
		return fmt.Errorf("cached version verification failed")
	}
	return nil
}

// ToggleRestore switches between the converted result and its original bytes.
// The inactive representation is retained in the conversion cache so the same
// journal can be restored and un-restored repeatedly until cache eviction.
func (s Store) ToggleRestore(ctx context.Context, id string) error {
	r, err := s.Record(id)
	if err != nil {
		return err
	}
	if r.Status != "complete" && r.Status != "restored" {
		return fmt.Errorf("conversion is not in a restorable state")
	}
	before, after := r.Before.File(), r.After.File()
	current, err := s.Repository.Get(ctx, before.Base().ID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("media entry was removed")
	}

	beforeHash := before.Base().Fingerprints.GetString("md5")
	afterHash := after.Base().Fingerprints.GetString("md5")

	if r.Status == "complete" {
		if !r.Cached || !matches(s.backup(id), beforeHash) {
			return fmt.Errorf("original is no longer in the restore cache")
		}
		if current.Base().Path != after.Base().Path || !matches(after.Base().Path, afterHash) {
			return fmt.Errorf("converted file is no longer active or has changed; restore the latest conversion first")
		}
		if _, err := os.Lstat(before.Base().Path); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("original destination is occupied; no files changed")
		}
		stat, err := os.Stat(after.Base().Path)
		if err != nil {
			return err
		}
		if err := ensureCachedCopy(after.Base().Path, s.convertedBackup(id), afterHash, stat.Mode().Perm()); err != nil {
			return err
		}
		r.Status = "restoring"
		if err := s.save(r); err != nil {
			return err
		}
		originalStat, err := os.Stat(s.backup(id))
		if err != nil {
			return err
		}
		if err := copyExclusive(s.backup(id), before.Base().Path, originalStat.Mode().Perm()); err != nil {
			return err
		}
		_ = os.Chtimes(before.Base().Path, before.Base().ModTime, before.Base().ModTime)
		if err := s.Repository.Swap(context.WithoutCancel(ctx), current, before); err != nil {
			return err
		}
		if err := removeMatching(after.Base().Path, afterHash); err != nil {
			return err
		}
		r.Status = "restored"
		if err := s.save(r); err != nil {
			return err
		}
		return s.TrimVersions()
	}

	if current.Base().Path != before.Base().Path || !matches(before.Base().Path, beforeHash) {
		return fmt.Errorf("original file is no longer active or has changed")
	}
	if !s.ConvertedCached(r) {
		return fmt.Errorf("converted version is no longer in the restore cache")
	}
	if _, err := os.Lstat(after.Base().Path); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("converted destination is occupied; no files changed")
	}
	if !r.Cached || !matches(s.backup(id), beforeHash) {
		stat, err := os.Stat(before.Base().Path)
		if err != nil {
			return err
		}
		if err := ensureCachedCopy(before.Base().Path, s.backup(id), beforeHash, stat.Mode().Perm()); err != nil {
			return err
		}
		r.Cached = true
	}

	r.Status = "restoring"
	if err := s.save(r); err != nil {
		return err
	}
	convertedStat, err := os.Stat(s.convertedBackup(id))
	if err != nil {
		return err
	}
	if err := copyExclusive(s.convertedBackup(id), after.Base().Path, convertedStat.Mode().Perm()); err != nil {
		return err
	}
	_ = os.Chtimes(after.Base().Path, after.Base().ModTime, after.Base().ModTime)
	if err := s.Repository.Swap(context.WithoutCancel(ctx), current, after); err != nil {
		return err
	}
	if err := removeMatching(before.Base().Path, beforeHash); err != nil {
		return err
	}
	r.Status = "complete"
	if err := s.save(r); err != nil {
		return err
	}
	return s.TrimVersions()
}

func (s Store) removeOriginalCache(r *Record, used *int64) (bool, error) {
	if !r.Cached || r.Before.File() == nil {
		return false, nil
	}
	path := s.backup(r.ID)
	valid := matches(path, r.Before.File().Base().Fingerprints.GetString("md5"))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	r.Cached = false
	if valid {
		*used -= r.Before.File().Base().Size
	}
	return true, nil
}

func (s Store) removeConvertedCache(r *Record, used *int64) (bool, error) {
	if !s.ConvertedCached(r) {
		return false, nil
	}
	if err := os.Remove(s.convertedBackup(r.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	*used -= r.After.File().Base().Size
	return true, nil
}

// TrimVersions enforces the shared cache limit across both retained originals
// and converted versions. It evicts the copy of the currently active version
// first, preserving the version needed to toggle state whenever possible.
func (s Store) TrimVersions() error {
	config, err := s.Config()
	if err != nil {
		return err
	}
	records, err := s.History()
	if err != nil {
		return err
	}

	var used int64
	for _, r := range records {
		if r.Cached && r.Before.File() != nil && matches(s.backup(r.ID), r.Before.File().Base().Fingerprints.GetString("md5")) {
			used += r.Before.File().Base().Size
		}
		if s.ConvertedCached(r) {
			used += r.After.File().Base().Size
		}
	}
	if used <= config.CacheLimitBytes {
		return nil
	}

	for _, r := range records {
		changed := false
		if r.Status == "complete" {
			if used > config.CacheLimitBytes {
				removed, err := s.removeConvertedCache(r, &used)
				if err != nil {
					return err
				}
				changed = changed || removed
			}
			if used > config.CacheLimitBytes {
				removed, err := s.removeOriginalCache(r, &used)
				if err != nil {
					return err
				}
				changed = changed || removed
			}
		} else {
			if used > config.CacheLimitBytes {
				removed, err := s.removeOriginalCache(r, &used)
				if err != nil {
					return err
				}
				changed = changed || removed
			}
			if used > config.CacheLimitBytes {
				removed, err := s.removeConvertedCache(r, &used)
				if err != nil {
					return err
				}
				changed = changed || removed
			}
		}
		if changed {
			if err := s.save(r); err != nil {
				return err
			}
		}
		if used <= config.CacheLimitBytes {
			break
		}
	}
	return nil
}
