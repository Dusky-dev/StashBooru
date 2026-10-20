package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/performer"
	"github.com/stashapp/stash/pkg/studio"
)

func identityForm(source booru.Source, kind, target string) string {
	if kind == "artist" && ((source.Engine == "pixiv" && strings.HasPrefix(target, "user:")) ||
		((source.Engine == "pawchive" || source.Engine == "kemono") && strings.HasPrefix(target, "creator:"))) {
		return "account"
	}
	return "tag"
}

func identityObservation(source booru.Source, kind, target, label, copyrightContext string, rank int) models.IdentityObservation {
	if label == "" {
		label = target
	}
	label = strings.ToValidUTF8(label[:min(len(label), 200)], "")
	return models.IdentityObservation{
		Instance: source.Instance(), SourceID: source.ID, SourceName: source.Name,
		Engine: source.Engine, SourceURL: source.URL, Kind: kind,
		Form: identityForm(source, kind, target), Target: target,
		Label: label, LabelRank: rank, Context: copyrightContext,
	}
}

func (rs booruBrowserRoutes) IdentityList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if rs.repository.ProviderIdentity == nil {
		writeBooruError(w, 503, "identities", "Identity storage is unavailable.")
		return
	}
	q := r.URL.Query()
	filter := models.IdentityFilter{Kind: q.Get("kind"), State: q.Get("state"), Query: q.Get("q"), Limit: 51}
	var err error
	if q.Get("after") != "" {
		filter.After, err = strconv.ParseInt(q.Get("after"), 10, 64)
	}
	if err == nil && q.Get("identity") != "" {
		filter.ID, err = strconv.ParseInt(q.Get("identity"), 10, 64)
	}
	if err == nil && q.Get("entity") != "" {
		filter.EntityID, err = strconv.Atoi(q.Get("entity"))
	}
	if err != nil {
		identityError(w, models.ErrIdentityInvalid)
		return
	}
	var rows []models.ProviderIdentity
	err = rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		rows, err = rs.repository.ProviderIdentity.List(ctx, filter)
		return err
	})
	if err != nil {
		identityError(w, err)
		return
	}
	next := ""
	if len(rows) > 50 {
		rows = rows[:50]
		next = strconv.FormatInt(rows[49].ID, 10)
	}
	writeVisualSimilarityJSON(w, map[string]any{"items": rows, "next": next})
}

func (rs booruBrowserRoutes) IdentityCandidates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q := r.URL.Query()
	id := int64(0)
	var err error
	if q.Get("identity") != "" {
		id, err = strconv.ParseInt(q.Get("identity"), 10, 64)
	}
	if rs.repository.ProviderIdentity == nil || err != nil || id < 0 || (q.Get("rejected") != "" && q.Get("rejected") != "1") {
		identityError(w, models.ErrIdentityInvalid)
		return
	}
	var rows []models.IdentityCandidate
	err = rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		if id > 0 {
			current, err := rs.repository.ProviderIdentity.Find(ctx, id)
			if err != nil {
				return err
			}
			if current == nil || current.Kind != q.Get("kind") {
				return models.ErrIdentityConflict
			}
		}
		var err error
		rows, err = rs.repository.ProviderIdentity.Candidates(ctx, q.Get("kind"), q.Get("q"), id, q.Get("rejected") == "1")
		return err
	})
	if err != nil {
		identityError(w, err)
		return
	}
	more := len(rows) > 50
	if more {
		rows = rows[:50]
	}
	writeVisualSimilarityJSON(w, map[string]any{"items": rows, "truncated": more})
}

type identityMutation struct {
	Action         string                    `json:"action"`
	Kind           string                    `json:"kind"`
	Source         string                    `json:"source,omitempty"`
	Instance       string                    `json:"instance,omitempty"`
	Target         string                    `json:"target,omitempty"`
	Label          string                    `json:"label,omitempty"`
	Name           string                    `json:"name,omitempty"`
	Disambiguation string                    `json:"disambiguation,omitempty"`
	Changes        []models.IdentityDecision `json:"changes,omitempty"`
}

func (rs booruBrowserRoutes) IdentityMutation(w http.ResponseWriter, r *http.Request) {
	var input identityMutation
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if rs.repository.ProviderIdentity == nil || !models.IdentityKind(input.Kind) {
		identityError(w, models.ErrIdentityInvalid)
		return
	}
	if input.Action == "discard" {
		var count int64
		err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
			var err error
			count, err = rs.repository.ProviderIdentity.DiscardUnresolved(ctx, input.Kind)
			return err
		})
		if err != nil {
			identityError(w, err)
			return
		}
		writeVisualSimilarityJSON(w, map[string]any{"discarded": count})
		return
	}
	if input.Action == "observe" {
		source := rs.reviewSource(w, input.Source, input.Instance)
		if source == nil {
			return
		}
		var rows []models.ProviderIdentity
		err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
			var err error
			rows, err = rs.repository.ProviderIdentity.Observe(ctx, []models.IdentityObservation{identityObservation(*source, input.Kind, input.Target, input.Label, "", 0)})
			return err
		})
		if err != nil {
			identityError(w, err)
			return
		}
		writeVisualSimilarityJSON(w, rows[0])
		return
	}
	if len(input.Changes) < 1 || len(input.Changes) > 100 {
		identityError(w, models.ErrIdentityInvalid)
		return
	}
	result := []models.ProviderIdentity{}
	err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		seen := map[int64]bool{}
		for _, change := range input.Changes {
			if seen[change.ID] {
				return models.ErrIdentityInvalid
			}
			seen[change.ID] = true
			row, err := rs.repository.ProviderIdentity.Find(ctx, change.ID)
			if err != nil {
				return err
			}
			if row == nil || row.Kind != input.Kind || row.Version != change.Version {
				return models.ErrIdentityConflict
			}
		}
		action := input.Action
		entityID := 0
		if action == "create" {
			if !models.IdentityText(input.Name, 200) || (input.Disambiguation != "" && (!models.IdentityText(input.Disambiguation, 200) || input.Kind != "character")) {
				return models.ErrIdentityInvalid
			}
			var err error
			entityID, err = rs.createIdentityEntity(ctx, input.Kind, strings.TrimSpace(input.Name), strings.TrimSpace(input.Disambiguation))
			if err != nil {
				return err
			}
			action = "link"
		}
		for _, change := range input.Changes {
			if entityID > 0 {
				change.EntityID = entityID
			}
			row, err := rs.repository.ProviderIdentity.Decide(ctx, action, change)
			if err != nil {
				return err
			}
			result = append(result, *row)
		}
		return nil
	})
	if err != nil {
		identityError(w, err)
		return
	}
	writeVisualSimilarityJSON(w, result)
}

func (rs booruBrowserRoutes) createIdentityEntity(ctx context.Context, kind, name, disambiguation string) (int, error) {
	switch kind {
	case "artist":
		input := models.NewCreateStudioInput()
		input.Name = name
		if err := studio.ValidateCreate(ctx, input, rs.repository.Studio); err != nil {
			return 0, models.ErrIdentityConflict
		}
		if err := rs.repository.Studio.Create(ctx, &input); err != nil {
			return 0, err
		}
		return input.ID, nil
	case "character":
		p := models.NewPerformer()
		p.Name, p.Disambiguation = name, disambiguation
		if err := performer.ValidateCreate(ctx, p, rs.repository.Performer); err != nil {
			return 0, models.ErrIdentityConflict
		}
		if err := rs.repository.Performer.Create(ctx, &models.CreatePerformerInput{Performer: &p}); err != nil {
			return 0, err
		}
		return p.ID, nil
	case "copyright":
		existing, err := rs.repository.Copyright.FindByName(ctx, name, true)
		if err != nil {
			return 0, err
		}
		if existing != nil {
			return 0, models.ErrIdentityConflict
		}
		existing, err = rs.repository.Copyright.FindByAlias(ctx, name, true)
		if err != nil {
			return 0, err
		}
		if existing != nil {
			return 0, models.ErrIdentityConflict
		}
		created, err := rs.repository.Copyright.Create(ctx, models.CopyrightCreateInput{Name: name})
		if err != nil {
			return 0, err
		}
		return created.ID, nil
	}
	return 0, models.ErrIdentityInvalid
}

func identityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, models.ErrIdentityConflict):
		writeBooruError(w, 409, "identity-conflict", "An identity or target changed. Reload and review again; no part of this request was saved.")
	case errors.Is(err, models.ErrIdentityInvalid):
		writeBooruError(w, 400, "identities", "Select valid identities, categories and current versions.")
	case errors.Is(err, models.ErrIdentityLimit):
		writeBooruError(w, 409, "identity-limit", "The 100,000-identity cache is full. Existing connections remain available. Discard unreviewed metadata to make room.")
	default:
		writeBooruError(w, 500, "identities", "Cannot save identity metadata. Retry after reloading.")
	}
}

// Browsing caches provider assertions, never creates catalogue entities or
// assigns media relationships. Existing user decisions survive every refresh.
func (rs booruBrowserRoutes) captureIdentityPosts(parent context.Context, source booru.Source, posts []booru.Post) {
	if rs.repository.ProviderIdentity == nil {
		return
	}
	input, positions := identityPostObservations(source, posts)
	if len(input) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	var rows []models.ProviderIdentity
	err := rs.repository.WithTxn(ctx, func(ctx context.Context) error {
		var err error
		rows, err = rs.repository.ProviderIdentity.Observe(ctx, input)
		return err
	})

	if err != nil {
		// Capacity or a failed refresh must not hide existing connections.
		recovery, stop := context.WithTimeout(parent, 2*time.Second)
		defer stop()
		_ = rs.repository.WithReadTxn(recovery, func(ctx context.Context) error {
			for i, o := range input {
				row, lookupErr := rs.repository.ProviderIdentity.Lookup(ctx, o.Instance, o.Kind, o.Form, o.Target)
				if lookupErr != nil {
					return lookupErr
				}
				if row != nil {
					p := positions[i]
					posts[p.post].Tags[p.tag].CanonicalName = row.EntityName
					posts[p.post].Tags[p.tag].IdentityID = strconv.FormatInt(row.ID, 10)
				}
			}
			return nil
		})
		return
	}
	for i, row := range rows {
		p := positions[i]
		posts[p.post].Tags[p.tag].CanonicalName = row.EntityName
		posts[p.post].Tags[p.tag].IdentityID = strconv.FormatInt(row.ID, 10)
	}
}

func identityPostObservations(source booru.Source, posts []booru.Post) ([]models.IdentityObservation, []struct{ post, tag int }) {
	input := []models.IdentityObservation{}
	positions := []struct{ post, tag int }{}
	for i, post := range posts {
		contextTags := []string{}
		for _, tag := range post.Tags {
			if tag.Category == "copyright" && len(contextTags) < 10 && len(tag.Name) <= 200 {
				contextTags = append(contextTags, tag.Name)
			}
		}
		for j, tag := range post.Tags {
			if !models.IdentityKind(tag.Category) || !models.IdentityText(tag.Name, 1024) || len(input) >= 2000 {
				continue
			}
			label, rank := tag.Label, 1
			if label == "" || source.Engine == "pawchive" || source.Engine == "kemono" {
				label, rank = tag.Name, 0
			}
			input = append(input, identityObservation(source, tag.Category, tag.Name, label, strings.Join(contextTags, ", "), rank))
			positions = append(positions, struct{ post, tag int }{i, j})
		}
	}
	return input, positions
}
