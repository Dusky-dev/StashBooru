package api

import (
	"context"
	"crypto/md5" //nolint:gosec // Provider MD5 is content evidence, not a security primitive.
	"encoding/hex"
	"io"
	"os"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

const booruLocalCandidateLimit = 100
const booruLocalFileLimit int64 = 64 << 20
const booruLocalPageLimit int64 = 256 << 20

var booruLocalSlots = make(chan struct{}, 2)

type booruVerifiedFile struct {
	path string
	info os.FileInfo
	hash string
}

// Read only indexed primary-file candidates. Hash their current bytes rather
// than trusting filenames, source_md5, stale fingerprints or visual similarity.
func verifyBooruFile(ctx context.Context, base *models.BaseFile, remaining *int64) *booruVerifiedFile {
	if ctx.Err() != nil || base.ZipFileID != nil || base.Path == "" {
		return nil
	}
	pathInfo, pathErr := os.Stat(base.Path)
	if pathErr != nil || !pathInfo.Mode().IsRegular() || pathInfo.Size() > booruLocalFileLimit || pathInfo.Size() > *remaining {
		return nil
	}
	file, err := os.Open(base.Path)
	if err != nil {
		return nil
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > booruLocalFileLimit || before.Size() > *remaining {
		return nil
	}
	*remaining -= before.Size()
	digest := md5.New() //nolint:gosec // Compare actual bytes with the provider's MD5.
	buffer := make([]byte, 128<<10)
	var total int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			total += int64(n)
			if total > before.Size() {
				return nil
			}
			_, _ = digest.Write(buffer[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil
		}
	}
	after, err := file.Stat()
	current, pathErr := os.Stat(base.Path)
	if err != nil || pathErr != nil || total != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !os.SameFile(before, current) || current.Size() != before.Size() || !current.ModTime().Equal(before.ModTime()) {
		return nil
	}
	return &booruVerifiedFile{path: base.Path, info: current, hash: hex.EncodeToString(digest.Sum(nil))}
}

func (rs booruBrowserRoutes) matchBooruLocal(parent context.Context, posts []booruReviewPost, rows []booruReviewItem) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	hashes := []string{}
	seen := map[string]bool{}
	for i, post := range posts {
		if post.MD5 != "" {
			rows[i].MatchStatus = "incomplete"
			if !seen[post.MD5] {
				seen[post.MD5] = true
				hashes = append(hashes, post.MD5)
			}
		}
	}
	if len(hashes) == 0 {
		return
	}
	select {
	case booruLocalSlots <- struct{}{}:
		defer func() { <-booruLocalSlots }()
	case <-ctx.Done():
		return
	}
	var candidates []models.BooruLocalCandidate
	files := map[models.FileID]*models.BaseFile{}
	load := func(ctx context.Context) error {
		var err error
		candidates, err = rs.repository.BooruReview.Candidates(ctx, hashes, booruLocalCandidateLimit+1)
		if err != nil {
			return err
		}
		ids := []models.FileID{}
		unique := map[models.FileID]bool{}
		for _, candidate := range candidates {
			if !unique[candidate.FileID] {
				unique[candidate.FileID] = true
				ids = append(ids, candidate.FileID)
			}
		}
		if len(ids) == 0 {
			return nil
		}
		found, err := rs.repository.File.Find(ctx, ids...)
		if err != nil {
			return err
		}
		for _, file := range found {
			files[file.Base().ID] = file.Base()
		}
		return nil
	}
	if rs.repository.WithReadTxn(ctx, load) != nil {
		return
	}
	truncated := len(candidates) > booruLocalCandidateLimit
	if truncated {
		candidates = candidates[:booruLocalCandidateLimit]
	}
	verified := map[models.FileID]*booruVerifiedFile{}
	remaining := booruLocalPageLimit
	for _, candidate := range candidates {
		if _, ok := verified[candidate.FileID]; !ok {
			if base := files[candidate.FileID]; base != nil {
				verified[candidate.FileID] = verifyBooruFile(ctx, base, &remaining)
			}
		}
	}
	// Re-read current primary-file links after filesystem I/O; no long-lived DB
	// transaction or permanent "in library" bit can survive replacement/deletion.
	var current []models.BooruLocalCandidate
	currentFiles := map[models.FileID]*models.BaseFile{}
	if err := rs.repository.WithReadTxn(ctx, func(ctx context.Context) error {
		var err error
		current, err = rs.repository.BooruReview.Candidates(ctx, hashes, booruLocalCandidateLimit+1)
		if err != nil {
			return err
		}
		ids := []models.FileID{}
		for id := range verified {
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return nil
		}
		found, err := rs.repository.File.Find(ctx, ids...)
		if err != nil {
			return err
		}
		for _, file := range found {
			currentFiles[file.Base().ID] = file.Base()
		}
		return nil
	}); err != nil {
		return
	}
	checkedAt := reviewCheckedAt()
	for i, post := range posts {
		if post.MD5 == "" {
			continue
		}
		rows[i].CheckedAt = checkedAt
		rows[i].MatchStatus = "no-indexed-match"
		if truncated {
			rows[i].MatchStatus = "incomplete"
		}
		for _, candidate := range candidates {
			if !strings.EqualFold(candidate.MD5, post.MD5) {
				continue
			}
			rows[i].MatchStatus = "incomplete"
			file := verified[candidate.FileID]
			base := currentFiles[candidate.FileID]
			if file == nil || file.hash != post.MD5 || base == nil || base.Path != file.path {
				continue
			}
			stillCurrent := false
			for _, now := range current {
				if now.Kind == candidate.Kind && now.ID == candidate.ID && now.FileID == candidate.FileID && now.MD5 == candidate.MD5 {
					stillCurrent = true
					break
				}
			}
			info, err := os.Stat(file.path)
			if !stillCurrent || err != nil || !os.SameFile(file.info, info) || info.Size() != file.info.Size() || !info.ModTime().Equal(file.info.ModTime()) {
				continue
			}
			kind := "image"
			if candidate.Kind == models.MediaKindVideo {
				kind = "scene"
			}
			rows[i].Matches = append(rows[i].Matches, booruReviewMatch{Kind: kind, ID: candidate.ID, FileID: candidate.FileID.String(), Evidence: "current-file-md5"})
		}
		if len(rows[i].Matches) > 0 {
			rows[i].MatchStatus = "verified"
		}
	}
}
