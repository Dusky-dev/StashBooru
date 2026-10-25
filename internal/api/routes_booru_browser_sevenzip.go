package api

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
)

// 7z files use the same server-issued media keys, download limits,
// inflation semaphore and verified image/video response path as ZIP.
// Unsupported encryption and multi-volume archives are not bypassed.
const booruSevenZipHeaderMax = 2 << 20
const booruSevenZipTotalMaxMultiplier = 4

func supportedBooruArchiveFormat(format string) bool {
	switch strings.ToLower(format) {
	case "zip", "cbz", "7z":
		return true
	}
	return false
}

func validateBooruArchiveBytes(data []byte, format string, maxBytes int64) error {
	_, _, err := openBooruArchiveEntriesWithLimit(data, format, maxBytes)
	return err
}

// The returned opener remains tied to the already validated in-memory
// archive. Indices are provider archive indices, not filesystem paths.
func openBooruArchiveEntriesWithLimit(data []byte, format string, maxBytes int64) (
	booruArchiveListing, func(int) (io.ReadCloser, error), error,
) {
	if strings.EqualFold(format, "7z") {
		listing, reader, err := indexBooruSevenZipWithLimit(data, maxBytes)
		if err != nil {
			return listing, nil, err
		}
		return listing, func(index int) (io.ReadCloser, error) {
			if index < 0 || index >= len(reader.File) {
				return nil, fmt.Errorf("invalid archive media index")
			}
			return reader.File[index].Open()
		}, nil
	}
	if !strings.EqualFold(format, "zip") && !strings.EqualFold(format, "cbz") {
		return booruArchiveListing{}, nil, fmt.Errorf("unsupported archive format")
	}
	listing, reader, err := indexBooruArchiveWithLimit(data, maxBytes)
	if err != nil {
		return listing, nil, err
	}
	return listing, func(index int) (io.ReadCloser, error) {
		if index < 0 || index >= len(reader.File) {
			return nil, fmt.Errorf("invalid archive media index")
		}
		return reader.File[index].Open()
	}, nil
}

func validBooruSevenZipName(name string) bool {
	if !utf8.ValidString(name) || len(name) == 0 || len(name) > 256 ||
		strings.ContainsAny(name, "\\\r\n\x00:") ||
		strings.HasPrefix(name, "/") || path.Clean(name) != name ||
		name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return false
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".apng", ".webp", ".avif",
		".jxl", ".bmp", ".tiff", ".mp4", ".webm":
		return true
	}
	return false
}

func indexBooruSevenZipWithLimit(data []byte, maxBytes int64) (booruArchiveListing, *sevenzip.Reader, error) {
	listing := booruArchiveListing{Files: []booruArchiveFile{}}
	if maxBytes < 1 || maxBytes > booruHardMaxFileSizeMiB<<20 ||
		len(data) < 32 || int64(len(data)) > maxBytes ||
		!bytes.Equal(data[:6], []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}) {
		return listing, nil, fmt.Errorf("invalid 7z archive or archive exceeds the size limit")
	}
	// Bound the encoded header before sevenzip.NewReader parses it. Reject
	// split-volume archives and oversized next headers; do not write to disk.
	offset, size := binary.LittleEndian.Uint64(data[12:20]), binary.LittleEndian.Uint64(data[20:28])
	if size > booruSevenZipHeaderMax || offset > uint64(len(data)-32) ||
		size > uint64(len(data)-32)-offset {
		return listing, nil, fmt.Errorf("7z archive header is oversized or incomplete")
	}
	reader, err := sevenzip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return listing, nil, fmt.Errorf("provider did not return a readable, unencrypted 7z archive: %w", err)
	}
	if len(reader.File) > booruArchiveMaxFiles {
		return listing, nil, fmt.Errorf("7z archive contains too many entries")
	}
	var mediaBytes uint64
	for i, file := range reader.File {
		size := file.UncompressedSize
		if !file.FileInfo().Mode().IsRegular() || size == 0 || size > uint64(maxBytes) ||
			!validBooruSevenZipName(file.Name) || len(listing.Files) >= booruArchiveMaxMedia {
			listing.Skipped++
			continue
		}
		// A solid 7z stream has no per-file compressed size. Cap the sum of
		// selectable bytes instead of applying ZIP's per-file ratio test.
		if mediaBytes+size > uint64(maxBytes)*booruSevenZipTotalMaxMultiplier {
			listing.Skipped++
			continue
		}
		mediaBytes += size
		listing.Files = append(listing.Files, booruArchiveFile{
			Index: i, Name: file.Name, Bytes: size,
			Format: strings.TrimPrefix(strings.ToLower(path.Ext(file.Name)), "."),
		})
	}
	return listing, reader, nil
}

// 7z members share the bounded ZIP stream-reader path for preview/download.
