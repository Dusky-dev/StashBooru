package models

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrIdentityInvalid  = errors.New("invalid provider identity")
	ErrIdentityConflict = errors.New("identity decision changed")
	ErrIdentityLimit    = errors.New("provider identity cache is full")
)

const MaxProviderIdentities = 100000

type IdentityObservation struct {
	Instance   string
	SourceID   string
	SourceName string
	Engine     string
	SourceURL  string
	Kind       string
	Form       string
	Target     string
	Label      string
	LabelRank  int
	Context    string
}

type ProviderIdentity struct {
	ID         int64  `json:"id,string"`
	Instance   string `json:"instance"`
	SourceID   string `json:"sourceID" db:"source_id"`
	SourceName string `json:"sourceName" db:"source_name"`
	Engine     string `json:"engine"`
	SourceURL  string `json:"sourceURL" db:"source_url"`
	Kind       string `json:"kind"`
	Form       string `json:"form"`
	Target     string `json:"target"`
	Label      string `json:"label"`
	Context    string `json:"context"`
	Version    int    `json:"version"`
	EntityID   int    `json:"entityID" db:"entity_id"`
	EntityName string `json:"entityName" db:"entity_name"`
	UpdatedAt  string `json:"updatedAt" db:"updated_at"`
}

type IdentityFilter struct {
	Kind     string
	State    string
	Query    string
	ID       int64
	After    int64
	EntityID int
	Limit    int
}

type IdentityCandidate struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation"`
	Rejected       bool   `json:"rejected"`
}

type IdentityDecision struct {
	ID       int64 `json:"id,string"`
	Version  int   `json:"version"`
	EntityID int   `json:"entityID"`
}

type ProviderIdentityReaderWriter interface {
	Observe(context.Context, []IdentityObservation) ([]ProviderIdentity, error)
	Find(context.Context, int64) (*ProviderIdentity, error)
	Lookup(context.Context, string, string, string, string) (*ProviderIdentity, error)
	List(context.Context, IdentityFilter) ([]ProviderIdentity, error)
	Decide(context.Context, string, IdentityDecision) (*ProviderIdentity, error)
	Candidates(context.Context, string, string, int64, bool) ([]IdentityCandidate, error)
	DiscardUnresolved(context.Context, string) (int64, error)
}

func IdentityKind(kind string) bool {
	return kind == "artist" || kind == "character" || kind == "copyright"
}

func IdentityText(value string, limit int) bool {
	return value != "" && strings.TrimSpace(value) != "" && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

// Name keys suggest candidates only. Provider targets retain exact spelling.
func IdentityNameKey(value, kind string) string {
	value = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(value, "_", " ")))
	if kind == "artist" {
		value = strings.TrimSpace(strings.TrimSuffix(value, "(artist)"))
	}
	return strings.Join(strings.Fields(value), " ")
}
