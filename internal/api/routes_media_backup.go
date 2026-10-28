package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/job"
)

const mediaBackupConfigName = "media-library-backup.json"

var mediaBackupLock sync.Mutex
var mediaBackupBusy bool
var mediaBackupJobID int
var mediaBackupSnapshotID = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$`)

// mediaBackupRunningLocked also releases the lock for jobs cancelled before execution.
func mediaBackupRunningLocked() bool {
	if mediaBackupBusy && mediaBackupJobID != 0 {
		j := manager.GetInstance().JobManager.GetJob(mediaBackupJobID)
		if j == nil || j.Status == job.StatusFinished || j.Status == job.StatusCancelled || j.Status == job.StatusFailed {
			mediaBackupBusy = false
			mediaBackupJobID = 0
		}
	}
	return mediaBackupBusy
}

type mediaBackupConfig struct {
	Destination string `json:"destination"`
}

type mediaBackupSnapshot struct {
	Snapshot string `json:"snapshot"`
	Files    int    `json:"files"`
	Bytes    int64  `json:"logicalBytes"`
}

type mediaBackupStatus struct {
	Destination string                `json:"destination"`
	Roots       []string              `json:"roots"`
	Snapshots   []mediaBackupSnapshot `json:"snapshots"`
	Busy        bool                  `json:"busy"`
	Available   bool                  `json:"available"`
}

type mediaBackupRequest struct {
	Action      string `json:"action"`
	Destination string `json:"destination,omitempty"`
	Snapshot    string `json:"snapshot,omitempty"`
	Output      string `json:"output,omitempty"`
	Member      string `json:"member,omitempty"`
}

func mediaBackupConfigFile() string {
	return filepath.Join(manager.GetInstance().Config.GetConfigPath(), mediaBackupConfigName)
}

func readMediaBackupConfig() (mediaBackupConfig, error) {
	bytes, err := os.ReadFile(mediaBackupConfigFile())
	if os.IsNotExist(err) {
		return mediaBackupConfig{}, nil
	}
	if err != nil {
		return mediaBackupConfig{}, err
	}
	var cfg mediaBackupConfig
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func configuredBackupRoots() []string {
	roots := make([]string, 0)
	seen := make(map[string]struct{})
	for _, item := range manager.GetInstance().Config.GetStashPaths() {
		if item == nil || strings.TrimSpace(item.Path) == "" {
			continue
		}
		absolute, err := filepath.Abs(item.Path)
		if err != nil {
			continue
		}
		if _, exists := seen[absolute]; !exists {
			seen[absolute] = struct{}{}
			roots = append(roots, absolute)
		}
	}
	return roots
}

func mediaBackupContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func validateBackupDestination(destination string, roots []string) (string, error) {
	if !filepath.IsAbs(destination) {
		return "", fmt.Errorf("media backup destination must be an absolute path")
	}
	absolute, err := filepath.EvalSymlinks(filepath.Clean(destination))
	if err != nil {
		return "", fmt.Errorf("media backup destination must already exist: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("media backup destination must be a directory")
	}
	for _, root := range roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", fmt.Errorf("cannot resolve configured media root: %w", err)
		}
		if mediaBackupContains(resolved, absolute) || mediaBackupContains(absolute, resolved) {
			return "", fmt.Errorf("backup destination overlaps a configured media root")
		}
	}
	return absolute, nil
}

// validateBackupRestoreOutput guards current media roots even when a snapshot
// was made before the library paths were changed. Parent must already exist.
func validateBackupRestoreOutput(output, store string, roots []string) (string, error) {
	if !filepath.IsAbs(output) {
		return "", fmt.Errorf("restore output must be an absolute path")
	}
	clean := filepath.Clean(output)
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return "", fmt.Errorf("restore output parent must exist: %w", err)
	}
	resolved := filepath.Join(parent, filepath.Base(clean))
	if info, err := os.Lstat(clean); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("restore output must be a regular directory or a new directory")
		}
		resolved, err = filepath.EvalSymlinks(clean)
		if err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	locations := append(append([]string{}, roots...), store)
	for _, location := range locations {
		root, err := filepath.EvalSymlinks(location)
		if err != nil {
			return "", fmt.Errorf("cannot resolve protected location: %w", err)
		}
		if mediaBackupContains(root, resolved) || mediaBackupContains(resolved, root) {
			return "", fmt.Errorf("restore output overlaps media files or the backup store")
		}
	}
	return clean, nil
}

func saveMediaBackupConfig(cfg mediaBackupConfig) error {
	configFile := mediaBackupConfigFile()
	dir := filepath.Dir(configFile)
	file, err := os.CreateTemp(dir, ".media-backup-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(cfg); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), configFile)
}

func mediaBackupScriptPath() string {
	if path := strings.TrimSpace(os.Getenv("STASH_MEDIA_BACKUP_SCRIPT")); path != "" {
		return path
	}
	return filepath.FromSlash("scripts/media_library_backup.py")
}

func mediaBackupPython() string {
	if path := strings.TrimSpace(manager.GetInstance().Config.GetPythonPath()); path != "" {
		return path
	}
	return "python3"
}

func mediaBackupCommand(ctx context.Context, args ...string) ([]byte, error) {
	path := mediaBackupScriptPath()
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("media backup script is not installed: %s", path)
	}
	command := exec.CommandContext(ctx, mediaBackupPython(), append([]string{path}, args...)...)
	result, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("media backup operation failed: %w: %s", err, strings.TrimSpace(string(result)))
	}
	return result, nil
}

func recentMediaBackupSnapshots(store string) []mediaBackupSnapshot {
	result := make([]mediaBackupSnapshot, 0)
	files, err := os.ReadDir(filepath.Join(store, "snapshots"))
	if err != nil {
		return result
	}
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".json")
		if !mediaBackupSnapshotID.MatchString(id) {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(store, "snapshots", file.Name()))
		if err != nil {
			continue
		}
		var value struct {
			Snapshot string `json:"snapshot"`
			Entries []struct {
				Size int64 `json:"size"`
			} `json:"entries"`
		}
		if json.Unmarshal(bytes, &value) != nil || value.Snapshot != id {
			continue
		}
		snapshot := mediaBackupSnapshot{Snapshot: id, Files: len(value.Entries)}
		for _, entry := range value.Entries {
			if entry.Size > 0 {
				snapshot.Bytes += entry.Size
			}
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Snapshot > result[j].Snapshot
	})
	if len(result) > 25 {
		return result[:25]
	}
	return result
}

func (rs imageRoutes) MediaBackupStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	mediaBackupLock.Lock()
	busy := mediaBackupRunningLocked()
	mediaBackupLock.Unlock()
	roots := configuredBackupRoots()
	status := mediaBackupStatus{Destination: cfg.Destination, Roots: roots, Busy: busy, Snapshots: []mediaBackupSnapshot{}}
	if _, err := os.Stat(mediaBackupScriptPath()); err == nil {
		status.Available = true
	}
	if cfg.Destination != "" {
		status.Snapshots = recentMediaBackupSnapshots(cfg.Destination)
	}
	writeVisualSimilarityJSON(w, status)
}

func (rs imageRoutes) MediaBackupAction(w http.ResponseWriter, r *http.Request) {
	var req mediaBackupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		http.Error(w, "invalid media backup request: "+err.Error(), http.StatusBadRequest)
		return
	}
	mediaBackupLock.Lock()
	defer mediaBackupLock.Unlock()
	if mediaBackupRunningLocked() {
		http.Error(w, "media backup is already running", http.StatusConflict)
		return
	}
	roots := configuredBackupRoots()
	if len(roots) == 0 {
		http.Error(w, "no configured media roots", http.StatusBadRequest)
		return
	}
	if req.Action == "config" {
		destination, err := validateBackupDestination(strings.TrimSpace(req.Destination), roots)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := saveMediaBackupConfig(mediaBackupConfig{Destination: destination}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeVisualSimilarityJSON(w, mediaBackupConfig{Destination: destination})
		return
	}
	if req.Action != "backup" && req.Action != "verify" && req.Action != "restore" {
		http.Error(w, "unsupported media backup action", http.StatusBadRequest)
		return
	}
	cfg, err := readMediaBackupConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	store, err := validateBackupDestination(cfg.Destination, roots)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Action != "backup" && !mediaBackupSnapshotID.MatchString(req.Snapshot) {
		http.Error(w, "invalid snapshot identifier", http.StatusBadRequest)
		return
	}
	args := []string{req.Action, "--store", store}
	switch req.Action {
	case "backup":
		for i, root := range roots {
			args = append(args, "--root", fmt.Sprintf("media%d=%s", i+1, root))
		}
	case "verify":
		args = append(args, "--snapshot", req.Snapshot)
	case "restore":
		output, err := validateBackupRestoreOutput(req.Output, store, roots)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		args = append(args, "--snapshot", req.Snapshot, "--output", output)
		if req.Member != "" {
			args = append(args, "--member", req.Member)
		}
	}
	if _, err := os.Stat(mediaBackupScriptPath()); err != nil {
		http.Error(w, "media backup script is not installed", http.StatusServiceUnavailable)
		return
	}
	mediaBackupBusy = true
	mgr := manager.GetInstance()
	jobID := mgr.JobManager.Add(r.Context(), "Media files: "+req.Action, job.MakeJobExec(func(ctx context.Context, progress *job.Progress) error {
		defer func() {
			mediaBackupLock.Lock()
			mediaBackupBusy = false
			mediaBackupJobID = 0
			mediaBackupLock.Unlock()
		}()
		progress.SetTotal(1)
		if _, err := mediaBackupCommand(ctx, args...); err != nil {
			return err
		}
		progress.SetProcessed(1)
		return nil
	}))
	mediaBackupJobID = jobID
	writeVisualSimilarityJSON(w, visualSimilarityJobResponse{JobID: jobID})
}
