package image

import (
	"context"
	"fmt"
	"os"

	"github.com/stashapp/stash/pkg/models"
)

// AdaptiveThumbnailChoice is deliberately size-aware rather than a perceptual
// similarity guarantee. It encodes only the selected format, never a silent
// fallback to a different codec.
type AdaptiveThumbnailChoice struct {
	Resolution int
	Quality    int
	Bytes      int
}

// GetAdaptiveThumbnail uses Stash's 320px JPEG Q70 as an approximate byte
// budget. Existing native cache bytes are reused as the reference when they
// exist. Otherwise a temporary baseline JPEG is encoded without writing it
// to the native cache (which may be intentionally deleted).
//
// Up to nine bounded candidate encodes are tried; the highest-scoring candidate
// under the budget wins. If none fits, return the smallest selected-codec
// candidate and let the caller report that limitation. Nothing is written
// until a complete candidate is selected.
func (e *ThumbnailEncoder) GetAdaptiveThumbnail(
	ctx context.Context,
	f models.File,
	maxWidth, minimumQuality, budgetPercent int,
	nativePath string,
) ([]byte, AdaptiveThumbnailChoice, error) {
	if maxWidth < 320 || maxWidth > 640 || minimumQuality < 10 ||
		minimumQuality > e.ThumbnailQuality || budgetPercent < 50 || budgetPercent > 150 {
		return nil, AdaptiveThumbnailChoice{}, fmt.Errorf("invalid adaptive thumbnail settings")
	}
	if err := ctx.Err(); err != nil {
		return nil, AdaptiveThumbnailChoice{}, err
	}
	var baselineBytes int64
	if st, err := os.Stat(nativePath); err == nil && st.Mode().IsRegular() && st.Size() > 0 {
		baselineBytes = st.Size()
	}
	if baselineBytes == 0 {
		base := *e
		base.ThumbnailFormat = "jpg"
		base.ThumbnailQuality = 70
		data, err := base.GetThumbnail(f, models.DefaultGthumbWidth)
		if err != nil {
			return nil, AdaptiveThumbnailChoice{}, fmt.Errorf("adaptive JPEG reference: %w", err)
		}
		baselineBytes = int64(len(data))
	}
	limit := baselineBytes * int64(budgetPercent) / 100
	if limit < 1 {
		limit = 1
	}
	widths := uniqueThumbnailValues(maxWidth, (maxWidth+320)/2, 320)
	qualities := uniqueThumbnailValues(e.ThumbnailQuality, (e.ThumbnailQuality+minimumQuality)/2, minimumQuality)
	var best []byte
	bestScore := -1
	var bestChoice AdaptiveThumbnailChoice
	var smallest []byte
	var smallestChoice AdaptiveThumbnailChoice
	// Do not reward resolution above the source size.
	sourceWidth := maxWidth
	if vf, ok := f.(models.VisualFile); ok {
		sourceWidth = vf.GetWidth()
		if h := vf.GetHeight(); h > sourceWidth {
			sourceWidth = h
		}
	}
	for _, width := range widths {
		for _, q := range qualities {
			if err := ctx.Err(); err != nil {
				return nil, AdaptiveThumbnailChoice{}, err
			}
			candidateEncoder := *e
			candidateEncoder.ThumbnailQuality = q
			data, err := candidateEncoder.GetThumbnail(f, width)
			if err != nil {
				return nil, AdaptiveThumbnailChoice{}, fmt.Errorf("adaptive thumbnail %dpx Q%d: %w", width, q, err)
			}
			choice := AdaptiveThumbnailChoice{Resolution: width, Quality: q, Bytes: len(data)}
			if smallest == nil || len(data) < len(smallest) {
				smallest, smallestChoice = data, choice
			}
			if int64(len(data)) <= limit {
				effectiveWidth := width
				if sourceWidth > 0 && sourceWidth < effectiveWidth {
					effectiveWidth = sourceWidth
				}
				score := q + (effectiveWidth-320)/16
				if score > bestScore || (score == bestScore && (best == nil || len(data) < len(best))) {
					best, bestChoice, bestScore = data, choice, score
				}
				// Maximum resolution at maximum permitted quality cannot be
				// improved under these bounds.
				if width == maxWidth && q == e.ThumbnailQuality {
					return best, bestChoice, nil
				}
				// Lower quality at this same width cannot beat its high-quality
				// candidate's score, so move to the next resolution.
				break
			}
		}
	}
	if best != nil {
		return best, bestChoice, nil
	}
	return smallest, smallestChoice, nil
}

func uniqueThumbnailValues(values ...int) []int {
	seen := make(map[int]bool, len(values))
	ret := make([]int, 0, len(values))
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			ret = append(ret, v)
		}
	}
	return ret
}
