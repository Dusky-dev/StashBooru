package api

import "sort"

type similarityClusterEdge struct {
	LeftID     int
	RightID    int
	Similarity float64
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
