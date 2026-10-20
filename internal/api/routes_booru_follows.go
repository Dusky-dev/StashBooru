package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

var booruFollowID = regexp.MustCompile(`^[A-Z2-7]{26,64}$`)

type booruFollowView struct {
	models.BooruFollow
	Status         string                      `json:"status"`
	ResolvedSource string                      `json:"resolvedSource,omitempty"`
	SyncSupported  bool                        `json:"syncSupported"`
	Feed           *models.FollowingCheckpoint `json:"feed,omitempty"`
}

// Display IDs may be renamed. An enabled alias of the exact same engine/URL can
// resolve a follow; reusing an ID for another instance cannot retarget it.
func resolveBooruFollow(row models.BooruFollow, sources []booru.Source) booruFollowView {
	view := booruFollowView{BooruFollow: row, Status: "source-unavailable"}
	for _, source := range sources {
		if source.Instance() == row.Instance {
			view.Status = "available"
			view.ResolvedSource = source.ID
			if source.ID == row.SourceID {
				return view
			}
		}
	}
	if view.Status != "available" {
		for _, source := range sources {
			if source.ID == row.SourceID {
				view.Status = "source-changed"
				break
			}
		}
	}
	return view
}

func (rs booruBrowserRoutes) Follows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if rs.repository.BooruFollow == nil {
		writeBooruError(w, 503, "follows", "Follow storage is unavailable.")
		return
	}
	var rows []models.BooruFollow
	states := map[string]models.FollowingCheckpoint{}
	err := rs.repository.WithReadTxn(r.Context(), func(ctx context.Context) error {
		var err error
		rows, err = rs.repository.BooruFollow.List(ctx)
		if err == nil && rs.repository.Following != nil {
			for _, row := range rows {
				state, readErr := rs.repository.Following.Checkpoint(ctx, row.ID, row.QueryRevision)
				if readErr != nil {
					return readErr
				}
				states[row.ID] = state
			}
		}
		return err
	})
	if err != nil {
		writeBooruError(w, 500, "follows", "Cannot load follows.")
		return
	}
	sources, sourceErr := rs.sources()
	views := make([]booruFollowView, 0, len(rows))
	for _, row := range rows {
		view := resolveBooruFollow(row, sources)
		view.SyncSupported = row.Engine == "danbooru" && view.Status == "available"
		if state, ok := states[row.ID]; ok {
			view.Feed = &state
		}
		if sourceErr != nil {
			view.Status = "configuration-error"
			view.ResolvedSource = ""
			view.SyncSupported = false
		}
		views = append(views, view)
	}
	writeVisualSimilarityJSON(w, views)
}

type booruFollowMutation struct {
	Action   string `json:"action"`
	ID       string `json:"id"`
	Version  int    `json:"version"`
	Source   string `json:"source"`
	Instance string `json:"instance"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
}

func (input booruFollowMutation) valid() bool {
	definition := models.BooruFollowDefinition{Name: input.Name, Kind: input.Kind, Target: input.Target}
	if input.Action == "create" {
		return input.ID == "" && input.Version == 0 && input.Source != "" && input.Instance != "" && definition.Validate() == nil
	}
	if !booruFollowID.MatchString(input.ID) || input.Version < 1 || input.Source != "" || input.Instance != "" {
		return false
	}
	switch input.Action {
	case "update":
		return definition.Validate() == nil
	case "unfollow", "refollow":
		return input.Name == "" && input.Kind == "" && input.Target == ""
	default:
		return false
	}
}

func supportsBooruFollow(source booru.Source) bool {
	switch source.Engine {
	case "danbooru", "gelbooru", "moebooru", "e621", "philomena", "shimmie":
		return true
	default:
		return false
	}
}

func (rs booruBrowserRoutes) UpdateFollow(w http.ResponseWriter, r *http.Request) {
	var input booruFollowMutation
	if !readBooruReviewJSON(w, r, &input) {
		return
	}
	if !input.valid() {
		writeBooruError(w, 400, "follow-input", "Use a named Artist, Character, Copyright or nonempty provider query. Updates require the current ID/version and cannot change the source.")
		return
	}
	if rs.repository.BooruFollow == nil {
		writeBooruError(w, 503, "follows", "Follow storage is unavailable.")
		return
	}
	// Unfollow must remain possible when its provider is disabled or settings
	// need repairing. Other actions need a current, enabled source binding.
	var sources []booru.Source
	if input.Action != "unfollow" {
		var err error
		sources, err = rs.sources()
		if err != nil {
			writeBooruError(w, 500, "configuration", "Cannot load booru source settings.")
			return
		}
	}
	var provider models.BooruReviewProvider
	if input.Action == "create" {
		var selected *booru.Source
		for _, source := range sources {
			if source.ID == input.Source && source.Instance() == input.Instance {
				selected = &source
				break
			}
		}
		if selected == nil {
			writeBooruError(w, 409, "source-changed", "Source changed or is unavailable. Reload sources before following.")
			return
		}
		if !supportsBooruFollow(*selected) {
			writeBooruError(w, 400, "unsupported-source", "Follows currently support tag-based boorus. Creator-service follows are a later phase.")
			return
		}
		provider = models.BooruReviewProvider{Instance: selected.Instance(), SourceID: selected.ID, Engine: selected.Engine, URL: selected.URL}
	}
	var row *models.BooruFollow
	err := rs.repository.WithTxn(r.Context(), func(ctx context.Context) error {
		definition := models.BooruFollowDefinition{Name: input.Name, Kind: input.Kind, Target: input.Target}
		if input.Action == "create" {
			var err error
			row, err = rs.repository.BooruFollow.Create(ctx, provider, definition)
			return err
		}
		current, err := rs.repository.BooruFollow.Find(ctx, input.ID)
		if err != nil {
			return err
		}
		if current == nil || current.Version != input.Version {
			return models.ErrBooruFollowConflict
		}
		if input.Action != "unfollow" && resolveBooruFollow(*current, sources).Status != "available" {
			return errBooruFollowSource
		}
		if input.Action == "update" {
			row, err = rs.repository.BooruFollow.Update(ctx, input.ID, input.Version, &definition, nil)
		} else {
			active := input.Action == "refollow"
			row, err = rs.repository.BooruFollow.Update(ctx, input.ID, input.Version, nil, &active)
		}
		return err
	})
	switch {
	case errors.Is(err, models.ErrBooruFollowConflict):
		writeBooruError(w, 409, "conflict", "Follow changed. Reload follows and retry with the current version.")
	case errors.Is(err, models.ErrBooruFollowDuplicate):
		writeBooruError(w, 409, "duplicate-follow", "This source/category/target is already saved. Reload follows and refollow the existing record if needed.")
	case errors.Is(err, errBooruFollowSource):
		writeBooruError(w, 409, "source-changed", "This follow's source is unavailable or changed. Restore the original source or create a separate follow.")
	case errors.Is(err, models.ErrBooruFollowLimit):
		writeBooruError(w, 409, "follow-limit", "At most 500 follow definitions, including inactive follows, can be stored.")
	case errors.Is(err, models.ErrBooruFollowInvalid):
		writeBooruError(w, 400, "follow-input", "Invalid follow definition.")
	case err != nil:
		writeBooruError(w, 500, "follows", "Cannot save follow. Nothing was changed.")
	default:
		writeVisualSimilarityJSON(w, row)
	}
}

var errBooruFollowSource = errors.New("follow source unavailable")
