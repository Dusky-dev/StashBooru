package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/stashapp/stash/pkg/models"
)

const (
	perceptualSimilaritySort       = "perceptual_similarity"
	defaultPHashSimilarityDistance = 5
	maxPHashSimilarityDistance     = 8
)

type pHashSimilarityOptions struct {
	Distance    int
	ReferenceID *int
}

func parsePHashSimilaritySort(value string) (string, *pHashSimilarityOptions, error) {
	if value == perceptualSimilaritySort {
		return perceptualSimilaritySort, &pHashSimilarityOptions{Distance: defaultPHashSimilarityDistance}, nil
	}

	prefix := perceptualSimilaritySort + ":"
	if !strings.HasPrefix(value, prefix) {
		return value, nil, nil
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return "", nil, fmt.Errorf("invalid perceptual similarity sort %q", value)
	}

	distance, err := strconv.Atoi(parts[1])
	if err != nil || distance < 0 || distance > maxPHashSimilarityDistance {
		return "", nil, fmt.Errorf("invalid perceptual similarity distance %q: expected 0-%d", parts[1], maxPHashSimilarityDistance)
	}

	options := &pHashSimilarityOptions{Distance: distance}
	if len(parts) == 3 {
		referenceID, err := strconv.Atoi(parts[2])
		if err != nil || referenceID <= 0 {
			return "", nil, fmt.Errorf("invalid perceptual similarity reference ID %q", parts[2])
		}
		options.ReferenceID = &referenceID
	}

	return perceptualSimilaritySort, options, nil
}

func getPHashSimilarityOptions(findFilter *models.FindFilterType) (*pHashSimilarityOptions, error) {
	if findFilter == nil || findFilter.Sort == nil || *findFilter.Sort == "" {
		return nil, nil
	}
	_, options, err := parsePHashSimilaritySort(*findFilter.Sort)
	return options, err
}

func pHashEntityFingerprintSQL(relationTable, relationIDColumn, entityExpression string) string {
	return fmt.Sprintf(
		`(SELECT similarity_fp.fingerprint
           FROM %s AS similarity_relation
           INNER JOIN %s AS similarity_fp
             ON similarity_relation.file_id = similarity_fp.file_id
            AND similarity_fp.type = 'phash'
          WHERE similarity_relation.%s = %s
          ORDER BY similarity_relation."primary" DESC, similarity_relation.file_id ASC
          LIMIT 1)`,
		relationTable,
		fingerprintTable,
		relationIDColumn,
		entityExpression,
	)
}

type pHashSimilarityCandidate struct {
	ID    int           `db:"id"`
	PHash sql.NullInt64 `db:"phash"`
}

func pHashDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

type pHashReferenceMatch struct {
	ID       int
	Distance int
}

func orderReferencePHashSimilarityCandidates(candidates []pHashSimilarityCandidate, referenceID, maxDistance int) ([]int, error) {
	var reference *pHashSimilarityCandidate
	for i := range candidates {
		if candidates[i].ID == referenceID {
			reference = &candidates[i]
			break
		}
	}

	if reference == nil {
		return nil, fmt.Errorf("perceptual similarity reference %d is not in the result set", referenceID)
	}
	if !reference.PHash.Valid {
		return nil, fmt.Errorf("perceptual similarity reference %d has no pHash", referenceID)
	}

	referenceHash := uint64(reference.PHash.Int64)
	matches := make([]pHashReferenceMatch, 0)
	for _, candidate := range candidates {
		if candidate.ID == referenceID || !candidate.PHash.Valid {
			continue
		}
		distance := pHashDistance(referenceHash, uint64(candidate.PHash.Int64))
		if distance <= maxDistance {
			matches = append(matches, pHashReferenceMatch{ID: candidate.ID, Distance: distance})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Distance != matches[j].Distance {
			return matches[i].Distance < matches[j].Distance
		}
		return matches[i].ID < matches[j].ID
	})

	ids := make([]int, len(matches))
	for i, match := range matches {
		ids[i] = match.ID
	}
	return ids, nil
}

type pHashBKNode struct {
	hash     uint64
	index    int
	children map[int]*pHashBKNode
}

func (n *pHashBKNode) search(target uint64, maxDistance int, fn func(index int)) {
	if n == nil {
		return
	}
	distance := pHashDistance(n.hash, target)
	if distance <= maxDistance {
		fn(n.index)
	}
	minEdge := distance - maxDistance
	if minEdge < 0 {
		minEdge = 0
	}
	maxEdge := distance + maxDistance
	for edge, child := range n.children {
		if edge >= minEdge && edge <= maxEdge {
			child.search(target, maxDistance, fn)
		}
	}
}

func (n *pHashBKNode) insert(hash uint64, index int) {
	for {
		distance := pHashDistance(n.hash, hash)
		if distance == 0 {
			return
		}
		if n.children == nil {
			n.children = make(map[int]*pHashBKNode)
		}
		child := n.children[distance]
		if child == nil {
			n.children[distance] = &pHashBKNode{hash: hash, index: index}
			return
		}
		n = child
	}
}

type pHashUnionFind struct {
	parent []int
	rank   []uint8
}

func newPHashUnionFind(size int) *pHashUnionFind {
	parent := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &pHashUnionFind{parent: parent, rank: make([]uint8, size)}
}

func (u *pHashUnionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *pHashUnionFind) union(a, b int) {
	a = u.find(a)
	b = u.find(b)
	if a == b {
		return
	}
	if u.rank[a] < u.rank[b] {
		a, b = b, a
	}
	u.parent[b] = a
	if u.rank[a] == u.rank[b] {
		u.rank[a]++
	}
}

type orderedPHashCluster struct {
	members        []int
	representative int
	score          int
}

func samplePHashMembers(members []int, max int) []int {
	if len(members) <= max {
		return members
	}
	ret := make([]int, 0, max)
	for i := 0; i < max; i++ {
		ret = append(ret, members[i*len(members)/max])
	}
	return ret
}

func choosePHashRepresentative(candidates []pHashSimilarityCandidate, members []int) int {
	candidateMembers := samplePHashMembers(members, 32)
	scoreMembers := samplePHashMembers(members, 64)
	best := candidateMembers[0]
	bestScore := int(^uint(0) >> 1)

	for _, candidateIndex := range candidateMembers {
		candidateHash := uint64(candidates[candidateIndex].PHash.Int64)
		score := 0
		for _, otherIndex := range scoreMembers {
			score += pHashDistance(candidateHash, uint64(candidates[otherIndex].PHash.Int64))
		}
		if score < bestScore || (score == bestScore && candidates[candidateIndex].ID < candidates[best].ID) {
			best = candidateIndex
			bestScore = score
		}
	}
	return best
}

func orderPHashSimilarityCandidates(candidates []pHashSimilarityCandidate, maxDistance int) []int {
	if len(candidates) == 0 {
		return nil
	}

	union := newPHashUnionFind(len(candidates))
	var tree *pHashBKNode
	var withoutPHash []int

	for index, candidate := range candidates {
		if !candidate.PHash.Valid {
			withoutPHash = append(withoutPHash, index)
			continue
		}
		hash := uint64(candidate.PHash.Int64)
		if tree == nil {
			tree = &pHashBKNode{hash: hash, index: index}
			continue
		}
		tree.search(hash, maxDistance, func(other int) { union.union(index, other) })
		tree.insert(hash, index)
	}

	groups := make(map[int][]int)
	for index, candidate := range candidates {
		if !candidate.PHash.Valid {
			continue
		}
		root := union.find(index)
		groups[root] = append(groups[root], index)
	}

	clusters := make([]orderedPHashCluster, 0, len(groups))
	for _, members := range groups {
		representative := choosePHashRepresentative(candidates, members)
		representativeHash := uint64(candidates[representative].PHash.Int64)

		sort.SliceStable(members, func(i, j int) bool {
			left, right := members[i], members[j]
			ld := pHashDistance(representativeHash, uint64(candidates[left].PHash.Int64))
			rd := pHashDistance(representativeHash, uint64(candidates[right].PHash.Int64))
			if ld != rd {
				return ld < rd
			}
			return candidates[left].ID < candidates[right].ID
		})

		score := 0
		for _, member := range members {
			score += pHashDistance(representativeHash, uint64(candidates[member].PHash.Int64))
		}
		clusters = append(clusters, orderedPHashCluster{members: members, representative: representative, score: score})
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		left, right := clusters[i], clusters[j]
		leftIsGroup, rightIsGroup := len(left.members) > 1, len(right.members) > 1
		if leftIsGroup != rightIsGroup {
			return leftIsGroup
		}
		la := float64(left.score) / float64(len(left.members))
		ra := float64(right.score) / float64(len(right.members))
		if la != ra {
			return la < ra
		}
		if len(left.members) != len(right.members) {
			return len(left.members) > len(right.members)
		}
		return candidates[left.representative].ID < candidates[right.representative].ID
	})

	ret := make([]int, 0, len(candidates))
	for _, cluster := range clusters {
		for _, index := range cluster.members {
			ret = append(ret, candidates[index].ID)
		}
	}

	sort.SliceStable(withoutPHash, func(i, j int) bool {
		return candidates[withoutPHash[i]].ID < candidates[withoutPHash[j]].ID
	})
	for _, index := range withoutPHash {
		ret = append(ret, candidates[index].ID)
	}
	return ret
}

func paginatePHashSimilarityIDs(ids []int, findFilter *models.FindFilterType) []int {
	page := 1
	perPage := 25
	if findFilter != nil {
		if findFilter.Page != nil && *findFilter.Page > 0 {
			page = *findFilter.Page
		}
		if findFilter.PerPage != nil {
			perPage = *findFilter.PerPage
		}
	}
	if perPage < 0 {
		return ids
	}
	if perPage == 0 {
		return []int{}
	}
	start := (page - 1) * perPage
	if start >= len(ids) {
		return []int{}
	}
	end := start + perPage
	if end > len(ids) {
		end = len(ids)
	}
	return ids[start:end]
}

func findPHashSimilarityCandidates(ctx context.Context, query queryBuilder, relationTable, relationIDColumn string) ([]pHashSimilarityCandidate, error) {
	const includeSortPagination = false
	baseSQL := query.toSQL(includeSortPagination)
	phashSQL := pHashEntityFingerprintSQL(relationTable, relationIDColumn, "similarity_candidates.id")
	sqlQuery := fmt.Sprintf("SELECT similarity_candidates.id AS id, %s AS phash FROM (%s) AS similarity_candidates", phashSQL, baseSQL)

	var candidates []pHashSimilarityCandidate
	if err := dbWrapper.Select(ctx, &candidates, sqlQuery, query.allArgs()...); err != nil {
		return nil, fmt.Errorf("querying pHash similarity candidates: %w", err)
	}
	return candidates, nil
}

func findPHashSimilarityIDs(ctx context.Context, query queryBuilder, relationTable, relationIDColumn string, findFilter *models.FindFilterType, options *pHashSimilarityOptions) ([]int, int, error) {
	if options.ReferenceID != nil && relationTable == imagesFilesTable {
		return findImageEmbeddingSimilarityIDs(ctx, query, findFilter, *options.ReferenceID)
	}

	candidates, err := findPHashSimilarityCandidates(ctx, query, relationTable, relationIDColumn)
	if err != nil {
		return nil, 0, err
	}

	var ids []int
	if options.ReferenceID != nil {
		ids, err = orderReferencePHashSimilarityCandidates(candidates, *options.ReferenceID, options.Distance)
		if err != nil {
			return nil, 0, err
		}
	} else {
		ids = orderPHashSimilarityCandidates(candidates, options.Distance)
	}

	total := len(ids)
	return paginatePHashSimilarityIDs(ids, findFilter), total, nil
}
