package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ApplyAssociationInheritanceSettings saves a reviewed set of defaults. The
// comparison, in-memory update and atomic file replacement share one lock.
func (i *Config) ApplyAssociationInheritanceSettings(expected, proposed AssociationInheritanceSettings) error {
	i.Lock()
	defer i.Unlock()

	if i.associationInheritanceSettings() != expected {
		return errors.New("inheritance defaults changed since preview; run a new preview")
	}
	keys := []string{
		AssociationInheritanceCharacters,
		AssociationInheritanceArtists,
		AssociationInheritanceCopyrights,
		AssociationInheritanceTags,
	}
	values := []bool{proposed.Characters, proposed.Artists, proposed.Copyrights, proposed.Tags}
	for index, key := range keys {
		if i.overrides.Exists(key) && i.overrides.Bool(key) != values[index] {
			return fmt.Errorf("inheritance default %s is overridden and cannot be changed here", key)
		}
	}

	previous := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if i.main.Exists(key) {
			previous[key] = i.main.Get(key)
		}
	}
	applied := false
	defer func() {
		if applied {
			return
		}
		for _, key := range keys {
			if value, exists := previous[key]; exists {
				_ = i.main.Set(key, value)
			} else {
				i.main.Delete(key)
			}
		}
	}()
	for index, key := range keys {
		if err := i.main.Set(key, values[index]); err != nil {
			return err
		}
	}
	data, err := i.marshal()
	if err != nil {
		return err
	}

	target := i.filePath
	mode := os.FileMode(0640)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
		if resolved, err := filepath.EvalSymlinks(target); err == nil {
			target = resolved
		} else {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(target), ".association-inheritance-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), target); err != nil {
		return err
	}
	applied = true
	return nil
}
