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
	Method          string                    `json:"method"`
	Distance        int                       `json:"distance"`
	Threshold       float64                   `json:"threshold"`
	Neighbors       int                       `json:"neighbors"`
	IndexedImages   int                       `json:"indexedImages"`
	TotalImages     int                       `json:"totalImages"`
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

func similarityClusterMemberForImage(ctx context.Context, repository models.Repository, imageID int) (*similarityClusterMember, error) {
	image, err := repository.Image.Find(ctx, imageID)
	if err != nil {
		return nil, err
	}
	if image == nil {
		return nil, nil // Image was deleted after the vector snapshot.
	}
	if err := image.LoadPrimaryFile(ctx, repository.File); err != nil {
		return nil, err
	}

	member := similarityClusterMember{ID: imageID}
	primary := image.Files.Primary()
	if primary == nil {
		return &member, nil
	}
	member.FileSize = primary.Base().Size
	if visual, ok := primary.(models.VisualFile); ok {
		member.Width = visual.GetWidth()
		member.Height = visual.GetHeight()
	}
	return &member, nil
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

// A missing method selects pHash, which is more appropriate for finding
// duplicate files than the semantically related imagery returned by EVA02.
func similarityClusterMethodOptions(r *http.Request) (string, int, error) {
	method := strings.TrimSpace(r.URL.Query().Get("method"))
	if method == "" {
		method = "phash"
	}
	if method != "phash" && method != "embedding" {
		return "", 0, fmt.Errorf("similarity method must be phash or embedding")
	}
	distance := 5
	if raw := strings.TrimSpace(r.URL.Query().Get("distance")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 8 {
			return "", 0, fmt.Errorf("pHash distance must be between 0 and 8")
		}
		distance = value
	}
	return method, distance, nil
}

func (rs imageRoutes) ImageSimilarityClusters(w http.ResponseWriter, r *http.Request) {
	method, distance, err := similarityClusterMethodOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	threshold, neighbors, includeSingletons, err := similarityClusterOptions(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repository := manager.GetInstance().Repository
	var clusterIDs [][]int
	var indexedImages int
	if method == "phash" {
		// Uses only primary-file pHashes and the existing BK-tree. No EVA02
		// worker, model or 1024-dimensional embedding is needed.
		if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
			var readErr error
			clusterIDs, indexedImages, readErr = sqlite.ImagePHashClusters(ctx, distance)
			return readErr
		}); err != nil {
			http.Error(w, fmt.Sprintf("building pHash duplicate groups: %v", err), http.StatusConflict)
			return
		}
	} else {
		// Snapshot live vectors and release the database during the CPU-bound
		// approximate candidate search.
		var vectors []sqlite.ImageEmbeddingVector
		if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
			var readErr error
			vectors, readErr = sqlite.VisualEmbeddings.ImageEmbeddingVectors(ctx)
			return readErr
		}); err != nil {
			http.Error(w, fmt.Sprintf("loading similarity vectors: %v", err), http.StatusConflict)
			return
		}
		if len(vectors) == 0 {
			http.Error(w, "visual similarity index has no Images", http.StatusConflict)
			return
		}
		edges, err := collectSimilarityClusterEdges(r.Context(), vectors, neighbors, threshold)
		if err != nil {
			http.Error(w, fmt.Sprintf("finding similarity candidates: %v", err), http.StatusConflict)
			return
		}
		assetIDs := make([]int, len(vectors))
		for i, vector := range vectors {
			assetIDs[i] = vector.ID
		}
		indexedImages = len(assetIDs)
		clusterIDs = buildSimilarityClusters(assetIDs, edges, threshold)
	}

	response := similarityClusterResponse{
		Method:          method,
		IndexedImages:   indexedImages,
		SingletonsShown: includeSingletons,
	}
	if method == "phash" {
		response.Distance = distance
	} else {
		response.Threshold = threshold
		response.Neighbors = neighbors
	}
	if err := repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var countErr error
		response.TotalImages, countErr = repository.Image.Count(ctx)
		if countErr != nil {
			return countErr
		}
		clusters := make([]similarityClusterReview, 0)
		for _, ids := range clusterIDs {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(ids) == 1 && !includeSingletons {
				continue
			}
			members := make([]similarityClusterMember, 0, len(ids))
			for _, id := range ids {
				member, err := similarityClusterMemberForImage(ctx, repository, id)
				if err != nil {
					return err
				}
				if member != nil {
					members = append(members, *member)
				}
			}
			if len(members) == 0 || (len(members) == 1 && !includeSingletons) {
				continue
			}
			preferredID := preferredSimilarityClusterMember(members)
			aliveIDs := make([]int, len(members))
			for i := range members {
				members[i].Preferred = members[i].ID == preferredID
				aliveIDs[i] = members[i].ID
			}
			clusters = append(clusters, similarityClusterReview{
				IDs: aliveIDs,
				Members: members,
				PreferredID: preferredID,
			})
		}
		response.Clusters = clusters
		return nil
	}); err != nil {
		http.Error(w, fmt.Sprintf("building similarity clusters: %v", err), http.StatusConflict)
		return
	}
	writeVisualSimilarityJSON(w, response)
}
