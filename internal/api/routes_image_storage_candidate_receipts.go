package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const storageCandidateReceiptVersion = 1
const storageCandidateReceiptLimit = 128 * 1024
const storageCandidateArchiveLimit = 384 * 1024 * 1024
const storageCandidateListLimit = 48

var storageCandidateIDPattern = regexp.MustCompile(`^stack-[1-9][0-9]*-(delta|shared-tiles)-[0-9a-f]{24}$`)

type storageCandidateSavedReceipt struct {
	Version       int             `json:"version"`
	ArchiveSHA256 string          `json:"archiveSHA256"`
	Report        json.RawMessage `json:"report"`
}

type storageCandidateSummary struct {
	ID           string `json:"id"`
	Group        string `json:"group"`
	Method       string `json:"method"`
	Snapshot     string `json:"snapshot"`
	ArchiveBytes int64  `json:"archiveBytes"`
}

// The externally supplied selection is an opaque, server-generated basename,
// never a filesystem path. Every receipt is outside all configured media roots.
func storageCandidateReceiptPath(directory, id string) (string, error) {
	if !storageCandidateIDPattern.MatchString(id) || id == "." || id == ".." {
		return "", fmt.Errorf("invalid saved candidate identifier")
	}
	return filepath.Join(directory, id+".receipt.json"), nil
}

func storageCandidateArchiveDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > storageCandidateArchiveLimit {
		return "", fmt.Errorf("candidate must be an existing bounded regular archive")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openInfo) {
		return "", fmt.Errorf("candidate changed while opening")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, storageCandidateArchiveLimit+1))
	if err != nil || n != info.Size() {
		return "", fmt.Errorf("candidate changed during fingerprinting")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, after) || after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return "", fmt.Errorf("candidate changed during fingerprinting")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func storageCandidateReceiptReport(data json.RawMessage) (storageCandidateSummary, string, error) {
	var decoded struct {
		storageCandidateProof
		ArchivePath string `json:"archivePath"`
	}
	if len(data) == 0 || len(data) > storageCandidateReceiptLimit {
		return storageCandidateSummary{}, "", fmt.Errorf("invalid saved candidate report size")
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return storageCandidateSummary{}, "", err
	}
	var flags map[string]json.RawMessage
	if err := json.Unmarshal(data, &flags); err != nil {
		return storageCandidateSummary{}, "", err
	}
	for _, key := range []string{"verified", "backupVerified", "sourcesRetained", "nativePackingCandidateOnly"} {
		if string(flags[key]) != "true" {
			return storageCandidateSummary{}, "", fmt.Errorf("candidate missing proof flag %s", key)
		}
	}
	for _, key := range []string{"nativeServing", "nativeServingVerified", "databaseBackupVerified", "originalEvictionEnabled", "archiveActivated"} {
		if string(flags[key]) != "false" {
			return storageCandidateSummary{}, "", fmt.Errorf("candidate has unsafe proof flag %s", key)
		}
	}
	if decoded.Fidelity != "byte-exact" || decoded.ArchiveBytes <= 0 ||
		decoded.SourceFiles < 2 || decoded.SourceFiles > 16 ||
		len(decoded.SourceSHA256) != decoded.SourceFiles ||
		!mediaBackupSnapshotID.MatchString(decoded.MediaBackupSnapshot) ||
		(decoded.Method != "delta" && decoded.Method != "shared-tiles") ||
		!strings.HasPrefix(decoded.Group, "stack:") || decoded.ArchivePath == "" {
		return storageCandidateSummary{}, "", fmt.Errorf("saved candidate proof is incomplete")
	}
	for _, digest := range decoded.SourceSHA256 {
		if !storageBackupSHA256.MatchString(digest) {
			return storageCandidateSummary{}, "", fmt.Errorf("saved candidate contains invalid original digest")
		}
	}
	return storageCandidateSummary{
		Group: decoded.Group, Method: decoded.Method,
		Snapshot: decoded.MediaBackupSnapshot, ArchiveBytes: decoded.ArchiveBytes,
	}, decoded.ArchivePath, nil
}

func storageCandidateSaveReceipt(directory string, proof json.RawMessage) (string, error) {
	summary, archive, err := storageCandidateReceiptReport(proof)
	if err != nil {
		return "", err
	}
	if err := validateStorageRecoveryArchive(archive, directory); err != nil {
		return "", err
	}
	id := strings.TrimSuffix(filepath.Base(archive), ".pack")
	path, err := storageCandidateReceiptPath(directory, id)
	if err != nil {
		return "", err
	}
	digest, err := storageCandidateArchiveDigest(archive)
	if err != nil {
		return "", err
	}
	if summary.ArchiveBytes != mustStorageCandidateArchiveSize(archive) {
		return "", fmt.Errorf("candidate size changed before recording")
	}
	encoded, err := json.Marshal(storageCandidateSavedReceipt{
		Version: storageCandidateReceiptVersion, ArchiveSHA256: digest, Report: proof,
	})
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(directory, ".candidate-proof-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return "", err
	}
	if _, err := temp.Write(encoded); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	// Link is no-overwrite and within the same filesystem. A crash cannot
	// expose a partial final receipt, nor can it replace an existing proof.
	if err := os.Link(temp.Name(), path); err != nil {
		return "", fmt.Errorf("cannot publish immutable candidate receipt: %w", err)
	}
	return id, nil
}

func mustStorageCandidateArchiveSize(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return -1
	}
	return info.Size()
}

func storageCandidateReadReceipt(directory, id string, rehash bool) (json.RawMessage, storageCandidateSummary, error) {
	path, err := storageCandidateReceiptPath(directory, id)
	if err != nil {
		return nil, storageCandidateSummary{}, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > storageCandidateReceiptLimit {
		return nil, storageCandidateSummary{}, fmt.Errorf("saved receipt missing or not a bounded regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, storageCandidateSummary{}, err
	}
	var record storageCandidateSavedReceipt
	if err := json.Unmarshal(data, &record); err != nil ||
		record.Version != storageCandidateReceiptVersion ||
		!storageBackupSHA256.MatchString(record.ArchiveSHA256) {
		return nil, storageCandidateSummary{}, fmt.Errorf("invalid persistent candidate receipt")
	}
	summary, archive, err := storageCandidateReceiptReport(record.Report)
	if err != nil {
		return nil, storageCandidateSummary{}, err
	}
	if archive != filepath.Join(directory, id+".pack") {
		return nil, storageCandidateSummary{}, fmt.Errorf("saved candidate archive path mismatch")
	}
	if err := validateStorageRecoveryArchive(archive, directory); err != nil ||
		mustStorageCandidateArchiveSize(archive) != summary.ArchiveBytes {
		return nil, storageCandidateSummary{}, fmt.Errorf("candidate missing or size mismatch")
	}
	if rehash {
		actual, err := storageCandidateArchiveDigest(archive)
		if err != nil || actual != record.ArchiveSHA256 {
			return nil, storageCandidateSummary{}, fmt.Errorf("candidate archive fingerprint does not match saved evidence")
		}
	}
	summary.ID = id
	return append(json.RawMessage(nil), record.Report...), summary, nil
}

func storageCandidateSavedList(directory string) []storageCandidateSummary {
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) > 4096 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		a, ea := entries[i].Info()
		b, eb := entries[j].Info()
		if ea != nil || eb != nil {
			return entries[i].Name() > entries[j].Name()
		}
		return a.ModTime().After(b.ModTime())
	})
	found := make([]storageCandidateSummary, 0, storageCandidateListLimit)
	for _, entry := range entries {
		if len(found) >= storageCandidateListLimit {
			break
		}
		if entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".receipt.json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".receipt.json")
		_, summary, err := storageCandidateReadReceipt(directory, id, false)
		if err == nil {
			found = append(found, summary)
		}
	}
	return found
}
