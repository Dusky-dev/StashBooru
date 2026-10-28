package api

import (
	"context"
	"math/rand"
	"sort"

	"github.com/stashapp/stash/pkg/sqlite"
)

const (
	// Multi-table sparse random-projection locality-sensitive hashing
	// substantially reduces cosine comparisons for large Image libraries.
	clusterHashBands       = 24
	clusterHashBits        = 12
	clusterProjectionTerms = 16
)

func clusterCosine(a, b []float32) float64 {
	var dot float32
	for i, v := range a {
		dot += v * b[i]
	}
	if dot > 1 {
		return 1
	}
	if dot < -1 {
		return -1
	}
	return float64(dot)
}

func keepClusterNeighbor(edges []similarityClusterEdge, candidate similarityClusterEdge, limit int) []similarityClusterEdge {
	if len(edges) < limit {
		return append(edges, candidate)
	}
	worst := 0
	for i := 1; i < len(edges); i++ {
		if edges[i].Similarity < edges[worst].Similarity {
			worst = i
		}
	}
	if candidate.Similarity > edges[worst].Similarity {
		edges[worst] = candidate
	}
	return edges
}

// collectSimilarityClusterEdges discovers pairs using approximate hashing,
// scores candidate pairs with all embedding dimensions, then retains the
// requested number of nearest neighbors. Borderline pairs can be missed;
// this is not an exhaustive pairwise cosine scan.
func collectSimilarityClusterEdges(ctx context.Context, vectors []sqlite.ImageEmbeddingVector, neighbors int, minSimilarity float64) ([]similarityClusterEdge, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if neighbors <= 0 || len(vectors) < 2 {
		return nil, nil
	}
	random := rand.New(rand.NewSource(1793143562))
	seen := make(map[uint64]struct{})
	nearest := make([][]similarityClusterEdge, len(vectors))
	var examined uint64

	for band := 0; band < clusterHashBands; band++ {
		var coordinates [clusterHashBits][clusterProjectionTerms]int
		var signs [clusterHashBits][clusterProjectionTerms]float32
		for bit := range coordinates {
			for term := range coordinates[bit] {
				coordinates[bit][term] = random.Intn(sqlite.VisualEmbeddingDimensions)
				signs[bit][term] = 1
				if random.Intn(2) == 0 {
					signs[bit][term] = -1
				}
			}
		}

		buckets := make(map[uint16][]int, len(vectors)/4)
		for i, vector := range vectors {
			var signature uint16
			for bit := 0; bit < clusterHashBits; bit++ {
				var projection float32
				for term := 0; term < clusterProjectionTerms; term++ {
					projection += vector.Values[coordinates[bit][term]] * signs[bit][term]
				}
				if projection >= 0 {
					signature |= 1 << bit
				}
			}
			buckets[signature] = append(buckets[signature], i)
		}

		for _, bucket := range buckets {
			for a := 0; a < len(bucket); a++ {
				for b := a + 1; b < len(bucket); b++ {
					left, right := bucket[a], bucket[b]
					key := uint64(uint32(left))<<32 | uint64(uint32(right))
					if _, found := seen[key]; found {
						continue
					}
					seen[key] = struct{}{}
					examined++
					if examined%8192 == 0 {
						if err := ctx.Err(); err != nil {
							return nil, err
						}
					}
					score := clusterCosine(vectors[left].Values, vectors[right].Values)
					if score < minSimilarity {
						continue
					}
					edge := similarityClusterEdge{
						LeftID: vectors[left].ID, RightID: vectors[right].ID, Similarity: score,
					}
					nearest[left] = keepClusterNeighbor(nearest[left], edge, neighbors)
					nearest[right] = keepClusterNeighbor(nearest[right], edge, neighbors)
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	byPair := make(map[[2]int]similarityClusterEdge)
	for _, edges := range nearest {
		for _, edge := range edges {
			key := similarityClusterEdgeKey(edge.LeftID, edge.RightID)
			byPair[key] = edge
		}
	}
	result := make([]similarityClusterEdge, 0, len(byPair))
	for _, edge := range byPair {
		result = append(result, edge)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LeftID != result[j].LeftID {
			return result[i].LeftID < result[j].LeftID
		}
		return result[i].RightID < result[j].RightID
	})
	return result, nil
}
