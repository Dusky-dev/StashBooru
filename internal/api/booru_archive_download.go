package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/stashapp/stash/pkg/booru"
)

// archiveDownloadMedia extracts exactly one entry approved by the same bounded
// ZIP index and signature checks used for in-browser previews. The caller
// stages these verified bytes through the ordinary Stash download job.
func (rs booruBrowserRoutes) archiveDownloadMedia(ctx context.Context, key string, index int, parent booru.Post) (booru.Post, booru.MediaInfo, []byte, error) {
	var info booru.MediaInfo
	if index < 0 || index >= booruArchiveMaxFiles {
		return parent, info, nil, fmt.Errorf("invalid archive file index")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/booru-browser/archive", nil)
	if err != nil {
		return parent, info, nil, err
	}
	archive, err := rs.loadBooruArchive(req, key)
	if err != nil {
		return parent, info, nil, err
	}
	listing, zr, err := indexBooruArchive(archive)
	if err != nil {
		return parent, info, nil, err
	}
	var selected *booruArchiveFile
	for i := range listing.Files {
		if listing.Files[i].Index == index {
			selected = &listing.Files[i]
			break
		}
	}
	if selected == nil {
		return parent, info, nil, fmt.Errorf("archive file is missing, unsupported or over the preview limit")
	}
	decode := func(ctx context.Context) ([]byte, string, error) {
		return readBooruArchiveEntry(ctx, zr.File[index], *selected)
	}
	var decoded []byte
	var mime string
	if selected.Format == "mp4" || selected.Format == "webm" {
		decoded, mime, err = cachedBooruArchiveVideo(ctx, key, index, decode)
	} else {
		decoded, mime, err = decode(ctx)
	}
	if err != nil {
		return parent, info, nil, err
	}
	if err = ctx.Err(); err != nil {
		return parent, info, nil, err
	}
	info = booru.MediaInfo{ContentType: mime, Extension: selected.Format, Bytes: int64(len(decoded))}
	// A synthetic, entry-specific identity keeps receipts and published files
	// distinct from the parent archive and every other media entry. Include the
	// contents so a replaced remote ZIP cannot reuse a stale download receipt.
	contentHash := sha256.Sum256(decoded)
	entryHash := sha256.Sum256([]byte(parent.ID + ":" + parent.OriginalURL + ":" + strconv.Itoa(index) + ":" + selected.Name + ":" + hex.EncodeToString(contentHash[:])))
	entry := parent
	entry.ID = "a" + hex.EncodeToString(entryHash[:16])
	entry.Filename = path.Base(selected.Name)
	entry.Position = index + 1
	entry.Width, entry.Height = 0, 0
	entry.Bytes = info.Bytes
	entry.Format = selected.Format
	entry.MD5 = ""
	entry.MediaKey = ""
	entry.Attachments = nil
	entry.PreviewURL, entry.SampleURL = "", ""
	return entry, info, decoded, nil
}
