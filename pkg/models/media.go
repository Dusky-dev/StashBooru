package models

import (
	"context"
	"fmt"
	"io"
	"strconv"
)

type MediaKind string

const (
	MediaKindImage MediaKind = "IMAGE"
	MediaKindVideo MediaKind = "VIDEO"
)

func (k MediaKind) IsValid() bool  { return k == MediaKindImage || k == MediaKindVideo }
func (k MediaKind) String() string { return string(k) }
func (k *MediaKind) UnmarshalGQL(v interface{}) error {
	s, ok := v.(string)
	if !ok || !MediaKind(s).IsValid() {
		return fmt.Errorf("invalid media kind: %v", v)
	}
	*k = MediaKind(s)
	return nil
}
func (k MediaKind) MarshalGQL(w io.Writer) { fmt.Fprint(w, strconv.Quote(string(k))) }

// MediaFilterType deliberately exposes the shared native media criteria.
// Duration is Video-only: supplying it excludes Images, including IS_NULL.
type MediaFilterType struct {
	MediaTypes        []MediaKind                      `json:"media_types"`
	Title             *StringCriterionInput            `json:"title"`
	Details           *StringCriterionInput            `json:"details"`
	Path              *StringCriterionInput            `json:"path"`
	Rating100         *IntCriterionInput               `json:"rating100"`
	Date              *DateCriterionInput              `json:"date"`
	CreatedAt         *TimestampCriterionInput         `json:"created_at"`
	UpdatedAt         *TimestampCriterionInput         `json:"updated_at"`
	Organized         *bool                            `json:"organized"`
	PerformerFavorite *bool                            `json:"performer_favorite"`
	Tags              *HierarchicalMultiCriterionInput `json:"tags"`
	Performers        *MultiCriterionInput             `json:"performers"`
	Studios           *HierarchicalMultiCriterionInput `json:"studios"`
	Copyrights        *HierarchicalMultiCriterionInput `json:"copyrights"`
	Duration          *IntCriterionInput               `json:"duration"`
}

type MediaReference struct {
	Kind MediaKind `db:"kind"`
	ID   int       `db:"id"`
}

func (m MediaReference) Key() string {
	prefix := "image"
	if m.Kind == MediaKindVideo {
		prefix = "scene"
	}
	return fmt.Sprintf("%s:%d", prefix, m.ID)
}

type MediaQueryResult struct {
	Count      int `db:"count"`
	ImageCount int `db:"image_count"`
	VideoCount int `db:"video_count"`
	Items      []MediaReference
}

type MediaReader interface {
	Query(ctx context.Context, filter *MediaFilterType, find *FindFilterType) (*MediaQueryResult, error)
}
