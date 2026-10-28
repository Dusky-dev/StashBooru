package sqlite

import (
	"context"
	"fmt"
	"sort"
)

// ImagePHashClusters groups Images with Hamming distance at or below
// maxDistance using their current primary-file pHash. No model or EVA02
// embedding is required. Each Image is read once and compared using the
// existing BK-tree rather than a quadratic all-pairs scan.
func ImagePHashClusters(ctx context.Context, maxDistance int) ([][]int, int, error) {
	if maxDistance < 0 || maxDistance > maxPHashSimilarityDistance {
		return nil, 0, fmt.Errorf("pHash distance must be between 0 and %d", maxPHashSimilarityDistance)
	}

	// Exclude secondary-file fingerprints and source_phash (provenance hashes):
	// only the active primary file is evidence about the displayed Image.
	const query = `SELECT i.id AS id, MIN(fp.fingerprint) AS phash
FROM images AS i
JOIN images_files AS image_file
  ON image_file.image_id = i.id AND image_file."primary" = 1
JOIN files_fingerprints AS fp
  ON fp.file_id = image_file.file_id AND fp.type = 'phash'
WHERE fp.fingerprint IS NOT NULL
GROUP BY i.id
ORDER BY i.id`
	var candidates []pHashSimilarityCandidate
	if err := dbWrapper.Select(ctx, &candidates, query); err != nil {
		return nil, 0, fmt.Errorf("loading image pHashes: %w", err)
	}
	count := len(candidates)
	if count == 0 {
		return nil, 0, nil
	}

	union := newPHashUnionFind(count)
	var tree *pHashBKNode
	for index, candidate := range candidates {
		if index%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
		}
		hash := uint64(candidate.PHash.Int64)
		if tree == nil {
			tree = &pHashBKNode{hash: hash, index: index}
			continue
		}
		tree.search(hash, maxDistance, func(other int) {
			union.union(index, other)
		})
		tree.insert(hash, index)
	}

	byRoot := make(map[int][]int)
	for index, candidate := range candidates {
		root := union.find(index)
		byRoot[root] = append(byRoot[root], candidate.ID)
	}
	clusters := make([][]int, 0, len(byRoot))
	for _, ids := range byRoot {
		clusters = append(clusters, ids) // input is sorted by Image ID
	}
	sort.Slice(clusters, func(i, j int) bool {
		if len(clusters[i]) != len(clusters[j]) {
			return len(clusters[i]) > len(clusters[j])
		}
		return clusters[i][0] < clusters[j][0]
	})
	return clusters, count, nil
}
