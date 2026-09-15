package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
)

const (
	defaultSimilarityClusterThreshold = 0.92
	defaultSimilarityClusterNeighbors = 12
	maxSimilarityClusterNeighbors     = 50
)

type similarityClusterEdge struct {
	LeftID     int
	RightID    int
	Similarity float64
}

type similarityClusterMember struct {
	ID        int   `json:"id"`
	Width     int   `json:"width"`
	Height    int   `json:"height"`
	FileSize  int64 `json:"fileSize"`
	Preferred bool  `json:"preferred"`
}

type similarityClusterReview struct {
	IDs         []int                     `json:"ids"`
	Members     []similarityClusterMember `json:"members"`
	PreferredID int                       `json:"preferredID"`
}

type similarityClusterResponse struct {
	Threshold       float64                   `json:"threshold"`
	Neighbors       int                       `json:"neighbors"`
	IndexedImages   int                       `json:"indexedImages"`
	Clusters        []similarityClusterReview `json:"clusters"`
	SingletonsShown bool                      `json:"singletonsShown"`
}

// buildSimilarityClusters groups assets connected by similarity edges at or
// above the requested threshold. All requested assets are returned, including
// singletons, so triage can distinguish isolated media from actual clusters.
func buildSimilarityClusters(assetIDs []int, edges []similarityClusterEdge, minSimilarity float64) [][]int {
	parent := make(map[int]int, len(assetIDs))
	for _, id := range assetIDs {
		if id > 0 {
			parent[id] = id
		}
	}

	var find func(int) int
	find = func(id int) int {
		root, ok := parent[id]
		if !ok {
			return 0
		}
		if root != id {
			parent[id] = find(root)
		}
		return parent[id]
	}
	union := func(left, right int) {
		leftRoot := find(left)
		rightRoot := find(right)
		if leftRoot == 0 || rightRoot == 0 || leftRoot == rightRoot {
			return
		}
		if leftRoot < rightRoot {
			parent[rightRoot] = leftRoot
		} else {
			parent[leftRoot] = rightRoot
		}
	}

	for _, edge := range edges {
		if edge.Similarity < minSimilarity || edge.Similarity > 1 {
			continue
		}
		union(edge.LeftID, edge.RightID)
	}

	byRoot := make(map[int][]int)
	for id := range parent {
		root := find(id)
		byRoot[root] = append(byRoot[root], id)
	}

	clusters := make([][]int, 0, len(byRoot))
	for _, cluster := range byRoot {
		sort.Ints(cluster)
		clusters = append(clusters, cluster)
	}
	sort.Slice(clusters, func(i, j int) bool {
		if len(clusters[i]) != len(clusters[j]) {
			return len(clusters[i]) > len(clusters[j])
		}
		return clusters[i][0] < clusters[j][0]
	})
	return clusters
}

func similarityClusterScore(distance float64) float64 {
	score := 1 - distance
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func similarityClusterEdgeKey(left, right int) [2]int {
	if left > right {
		left, right = right, left
	}
	return [2]int{left, right}
}

func collectSimilarityClusterEdges(ctx context.Context, assetIDs []int, neighbors int, minSimilarity float64) ([]similarityClusterEdge, error) {
	indexed := make(map[int]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		indexed[id] = struct{}{}
	}

	byPair := make(map[[2]int]similarityClusterEdge)
	for _, referenceID := range assetIDs {
		matches, err := sqlite.VisualEmbeddings.FindSimilarImages(ctx, referenceID, neighbors)
		if err != nil {
			return nil, fmt.Errorf("finding visual neighbors for image %d: %w", referenceID, err)
		}
		for _, match := range matches {
			if _, ok := indexed[match.ID]; !ok {
				continue
			}
			similarity := similarityClusterScore(match.Distance)
			if similarity < minSimilarity {
				continue
			}
			key := similarityClusterEdgeKey(referenceID, match.ID)
			if key[0] == key[1] {
				continue
			}
			current, exists := byPair[key]
			if !exists || similarity > current.Similarity {
				byPair[key] = similarityClusterEdge{
					LeftID:     key[0],
					RightID:    key[1],
					Similarity: similarity,
				}
			}
		}
	}

	edges := make([]similarityClusterEdge, 0, len(byPair))
	for _, edge := range byPair {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].LeftID != edges[j].LeftID {
			return edges[i].LeftID < edges[j].LeftID
		}
		return edges[i].RightID < edges[j].RightID
	})
	return edges, nil
}

func similarityClusterMemberForImage(ctx context.Context, repository models.Repository, imageID int) (similarityClusterMember, error) {
	image, err := repository.Image.Find(ctx, imageID)
	if err != nil {
		return similarityClusterMember{}, err
	}
	if image == nil {
		return similarityClusterMember{}, fmt.Errorf("image %d not found", imageID)
	}
	if err := image.LoadPrimaryFile(ctx, repository.File); err != nil {
		return similarityClusterMember{}, err
	}

	member := similarityClusterMember{ID: imageID}
	primary := image.Files.Primary()
	if primary == nil {
		return member, nil
	}
	member.FileSize = primary.Base().Size
	if visual, ok := primary.(models.VisualFile); ok {
		member.Width = visual.GetWidth()
		member.Height = visual.GetHeight()
	}
	return member, nil
}

func preferredSimilarityClusterMember(members []similarityClusterMember) int {
	if len(members) == 0 {
		return 0
	}
	preferred := members[0]
	for _, candidate := range members[1:] {
		preferredPixels := int64(preferred.Width) * int64(preferred.Height)
		candidatePixels := int64(candidate.Width) * int64(candidate.Height)
		if candidatePixels > preferredPixels ||
			(candidatePixels == preferredPixels && candidate.FileSize > preferred.FileSize) ||
			(candidatePixels == preferredPixels && candidate.FileSize == preferred.FileSize && candidate.ID < preferred.ID) {
			preferred = candidate
		}
	}
	return preferred.ID
}

func similarityClusterOptions(r *http.Request) (float64, int, bool, error) {
	threshold := defaultSimilarityClusterThreshold
	neighbors := defaultSimilarityClusterNeighbors
	includeSingletons := false

	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value <= 0 || value > 1 {
			return 0, 0, false, fmt.Errorf("cluster threshold must be greater than 0 and at most 1")
		}
		threshold = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("neighbors")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxSimilarityClusterNeighbors {
			return 0, 0, false, fmt.Errorf("cluster neighbors must be between 1 and %d", maxSimilarityClusterNeighbors)
		}
		neighbors = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("includeSingletons")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return 0, 0, false, fmt.Errorf("includeSingletons must be true or false")
		}
		includeSingletons = value
	}
	return threshold, neighbors, includeSingletons, nil
}

func (rs imageRoutes) ImageSimilarityClusters(w http.ResponseWriter, r *http.Request) {
	threshold, neighbors, includeSingletons, err := similarityClusterOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repository := manager.GetInstance().Repository
	var response similarityClusterResponse
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		assetIDs, err := sqlite.VisualEmbeddings.ImageEmbeddingIDs(ctx)
		if err != nil {
			return err
		}
		if len(assetIDs) == 0 {
			return fmt.Errorf("visual similarity index has no Images")
		}
		edges, err := collectSimilarityClusterEdges(ctx, assetIDs, neighbors, threshold)
		if err != nil {
			return err
		}
		clusterIDs := buildSimilarityClusters(assetIDs, edges, threshold)
		clusters := make([]similarityClusterReview, 0, len(clusterIDs))
		for _, ids := range clusterIDs {
			if len(ids) == 1 && !includeSingletons {
				continue
			}
			members := make([]similarityClusterMember, 0, len(ids))
			for _, id := range ids {
				member, err := similarityClusterMemberForImage(ctx, repository, id)
				if err != nil {
					return err
				}
				members = append(members, member)
			}
			preferredID := preferredSimilarityClusterMember(members)
			for index := range members {
				members[index].Preferred = members[index].ID == preferredID
			}
			clusters = append(clusters, similarityClusterReview{
				IDs:         append([]int(nil), ids...),
				Members:     members,
				PreferredID: preferredID,
			})
		}
		response = similarityClusterResponse{
			Threshold:       threshold,
			Neighbors:       neighbors,
			IndexedImages:   len(assetIDs),
			Clusters:        clusters,
			SingletonsShown: includeSingletons,
		}
		return nil
	}); err != nil {
		http.Error(w, fmt.Sprintf("building similarity clusters: %v", err), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
