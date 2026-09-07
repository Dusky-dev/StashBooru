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

// Similarity state is encoded in FindFilter.sort:
//
//	perceptual_similarity
//	perceptual_similarity:5
//	perceptual_similarity:5:123
//
// The final form means:
//   - Hamming distance <= 5
//   - reference entity ID = 123
func parsePHashSimilaritySort(
	value string,
) (string, *pHashSimilarityOptions, error) {
	if value == perceptualSimilaritySort {
		return perceptualSimilaritySort, &pHashSimilarityOptions{
			Distance: defaultPHashSimilarityDistance,
		}, nil
	}

	prefix := perceptualSimilaritySort + ":"
	if !strings.HasPrefix(value, prefix) {
		return value, nil, nil
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return "", nil, fmt.Errorf(
			"invalid perceptual similarity sort %q",
			value,
		)
	}

	distance, err := strconv.Atoi(parts[1])
	if err != nil ||
		distance < 0 ||
		distance > maxPHashSimilarityDistance {
		return "", nil, fmt.Errorf(
			"invalid perceptual similarity distance %q: expected 0-%d",
			parts[1],
			maxPHashSimilarityDistance,
		)
	}

	options := &pHashSimilarityOptions{
		Distance: distance,
	}

	if len(parts) == 3 {
		referenceID, err := strconv.Atoi(parts[2])
		if err != nil || referenceID <= 0 {
			return "", nil, fmt.Errorf(
				"invalid perceptual similarity reference ID %q",
				parts[2],
			)
		}

		options.ReferenceID = &referenceID
	}

	return perceptualSimilaritySort, options, nil
}

func getPHashSimilarityOptions(
	findFilter *models.FindFilterType,
) (*pHashSimilarityOptions, error) {
	if findFilter == nil ||
		findFilter.Sort == nil ||
		*findFilter.Sort == "" {
		return nil, nil
	}

	_, options, err := parsePHashSimilaritySort(*findFilter.Sort)
	return options, err
}

// Returns a SQL expression resolving the pHash belonging to an entity's
// primary file.
func pHashEntityFingerprintSQL(
	relationTable string,
	relationIDColumn string,
	entityExpression string,
) string {
	return fmt.Sprintf(
		`(SELECT similarity_fp.fingerprint
		FROM %s AS similarity_relation
		INNER JOIN %s AS similarity_fp
		ON similarity_relation.file_id = similarity_fp.file_id
		AND similarity_fp.type = 'phash'
	WHERE similarity_relation.%s = %s
	ORDER BY similarity_relation."primary" DESC,
	similarity_relation.file_id ASC
	LIMIT 1)`,
		relationTable,
		fingerprintTable,
		relationIDColumn,
		entityExpression,
	)
}

// Applies true pHash Hamming-distance matching against a selected reference
// Image or Scene.
//
// Candidates outside the chosen distance are removed and remaining candidates
// are ordered closest-first.
func applyReferencePHashSimilarity(
	query *queryBuilder,
	relationTable string,
	relationIDColumn string,
	entityTable string,
	options *pHashSimilarityOptions,
) string {
	if options == nil || options.ReferenceID == nil {
		return ""
	}

	candidate := pHashEntityFingerprintSQL(
		relationTable,
		relationIDColumn,
		entityTable+".id",
	)

	reference := pHashEntityFingerprintSQL(
		relationTable,
		relationIDColumn,
		strconv.Itoa(*options.ReferenceID),
	)

	// Stash stores pHashes as SQLite integer fingerprints.
	// Guard the phash_distance() calls so invalid/non-pHash rows aren't
	// accidentally coerced.
	query.addWhere(
		"typeof(" + candidate + ") = 'integer'",
	)
	query.addWhere(
		"typeof(" + reference + ") = 'integer'",
	)

	query.addWhere(
		fmt.Sprintf(
			"phash_distance(%s, %s) <= ?",
			candidate,
			reference,
		),
	)
	query.addArg(options.Distance)

	// Don't include the reference object itself.
	query.addWhere(entityTable + ".id != ?")
	query.addArg(*options.ReferenceID)

	// Always nearest first.
	return fmt.Sprintf(
		" ORDER BY phash_distance(%s, %s) ASC",
		candidate,
		reference,
	)
}

type pHashSimilarityCandidate struct {
	ID    int           `db:"id"`
	PHash sql.NullInt64 `db:"phash"`
}

// BK-tree node used for whole-gallery similarity grouping.
type pHashBKNode struct {
	hash     uint64
	index    int
	children map[int]*pHashBKNode
}

func pHashDistance(a uint64, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

func (n *pHashBKNode) search(
	target uint64,
	maxDistance int,
	fn func(index int),
) {
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
			n.children[distance] = &pHashBKNode{
				hash:  hash,
				index: index,
			}
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

	return &pHashUnionFind{
		parent: parent,
		rank:   make([]uint8, size),
	}
}

func (u *pHashUnionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}

	return x
}

func (u *pHashUnionFind) union(a int, b int) {
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
		index := i * len(members) / max
		ret = append(ret, members[index])
	}

	return ret
}

// Picks a representative pHash for a cluster.
//
// For small clusters this effectively finds the medoid. Large clusters are
// sampled to prevent excessively expensive pair comparisons.
func choosePHashRepresentative(
	candidates []pHashSimilarityCandidate,
	members []int,
) int {
	candidateMembers := samplePHashMembers(members, 32)
	scoreMembers := samplePHashMembers(members, 64)

	best := candidateMembers[0]
	bestScore := int(^uint(0) >> 1)

	for _, candidateIndex := range candidateMembers {
		candidateHash := uint64(
			candidates[candidateIndex].PHash.Int64,
		)

		score := 0

		for _, otherIndex := range scoreMembers {
			score += pHashDistance(
				candidateHash,
				uint64(candidates[otherIndex].PHash.Int64),
			)
		}

		if score < bestScore ||
			(score == bestScore &&
				candidates[candidateIndex].ID < candidates[best].ID) {
			best = candidateIndex
			bestScore = score
		}
	}

	return best
}

// Orders the complete result set into perceptual-similarity clusters when
// there is no reference item.
func orderPHashSimilarityCandidates(
	candidates []pHashSimilarityCandidate,
	maxDistance int,
) []int {
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
			tree = &pHashBKNode{
				hash:  hash,
				index: index,
			}
			continue
		}

		tree.search(
			hash,
			maxDistance,
			func(other int) {
				union.union(index, other)
			},
		)

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

	clusters := make(
		[]orderedPHashCluster,
		0,
		len(groups),
	)

	for _, members := range groups {
		representative := choosePHashRepresentative(
			candidates,
			members,
		)

		representativeHash := uint64(
			candidates[representative].PHash.Int64,
		)

		// Nearest items to the cluster representative appear first.
		sort.SliceStable(
			members,
			func(i int, j int) bool {
				left := members[i]
				right := members[j]

				leftDistance := pHashDistance(
					representativeHash,
					uint64(candidates[left].PHash.Int64),
				)

				rightDistance := pHashDistance(
					representativeHash,
					uint64(candidates[right].PHash.Int64),
				)

				if leftDistance != rightDistance {
					return leftDistance < rightDistance
				}

				return candidates[left].ID <
					candidates[right].ID
			},
		)

		score := 0

		for _, member := range members {
			score += pHashDistance(
				representativeHash,
				uint64(candidates[member].PHash.Int64),
			)
		}

		clusters = append(
			clusters,
			orderedPHashCluster{
				members:        members,
				representative: representative,
				score:          score,
			},
		)
	}

	// Groups containing actual matches come first.
	// Within those, prefer tighter and then larger clusters.
	sort.SliceStable(
		clusters,
		func(i int, j int) bool {
			left := clusters[i]
			right := clusters[j]

			leftIsGroup := len(left.members) > 1
			rightIsGroup := len(right.members) > 1

			if leftIsGroup != rightIsGroup {
				return leftIsGroup
			}

			leftAverage :=
				float64(left.score) /
					float64(len(left.members))

			rightAverage :=
				float64(right.score) /
					float64(len(right.members))

			if leftAverage != rightAverage {
				return leftAverage < rightAverage
			}

			if len(left.members) != len(right.members) {
				return len(left.members) > len(right.members)
			}

			return candidates[left.representative].ID <
				candidates[right.representative].ID
		},
	)

	ret := make([]int, 0, len(candidates))

	for _, cluster := range clusters {
		for _, index := range cluster.members {
			ret = append(
				ret,
				candidates[index].ID,
			)
		}
	}

	// Entries with no pHash stay visible but cannot participate in
	// perceptual clustering.
	sort.SliceStable(
		withoutPHash,
		func(i int, j int) bool {
			return candidates[withoutPHash[i]].ID <
				candidates[withoutPHash[j]].ID
		},
	)

	for _, index := range withoutPHash {
		ret = append(
			ret,
			candidates[index].ID,
		)
	}

	return ret
}

func paginatePHashSimilarityIDs(
	ids []int,
	findFilter *models.FindFilterType,
) []int {
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

	// Stash uses negative per-page values for unbounded queries.
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

// Whole-gallery mode:
//
//  1. Run the current filtered query without pagination.
//  2. Resolve each entity's pHash.
//  3. Cluster them according to the selected Hamming threshold.
//  4. Apply pagination only after similarity ordering.
func findPHashClusteredIDs(
	ctx context.Context,
	query queryBuilder,
	relationTable string,
	relationIDColumn string,
	findFilter *models.FindFilterType,
	maxDistance int,
) ([]int, error) {
	const includeSortPagination = false

	baseSQL := query.toSQL(includeSortPagination)

	phashSQL := pHashEntityFingerprintSQL(
		relationTable,
		relationIDColumn,
		"similarity_candidates.id",
	)

	sqlQuery := fmt.Sprintf(
		"SELECT similarity_candidates.id AS id, %s AS phash "+
			"FROM (%s) AS similarity_candidates",
		phashSQL,
		baseSQL,
	)

	var candidates []pHashSimilarityCandidate

	if err := dbWrapper.Select(
		ctx,
		&candidates,
		sqlQuery,
		query.allArgs()...,
	); err != nil {
		return nil, fmt.Errorf(
			"querying pHash similarity candidates: %w",
			err,
		)
	}

	ids := orderPHashSimilarityCandidates(
		candidates,
		maxDistance,
	)

	return paginatePHashSimilarityIDs(
		ids,
		findFilter,
	), nil
}
