package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/stashapp/stash/internal/characterdedup"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
)

type characterDedupRoutes struct {
	repository models.Repository
	jobs       *job.Manager
}

func (rs characterDedupRoutes) Get(w http.ResponseWriter, r *http.Request) {
	result, err := characterdedup.Preview(r.Context(), rs.repository)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeVisualSimilarityJSON(w, result)
}

func (rs characterDedupRoutes) Post(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Fingerprint string                      `json:"fingerprint"`
		Action      string                      `json:"action"`
		Selected    []int                       `json:"selected"`
		Pairs       []models.CharacterDedupPair `json:"pairs"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 131072))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "Invalid Character review request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid Character review request", http.StatusBadRequest)
		return
	}
	digest, err := hex.DecodeString(input.Fingerprint)
	if err != nil || len(digest) != 32 || len(input.Selected) > 10000 || len(input.Pairs) > 10000 {
		http.Error(w, "A fresh Character merge preview is required", http.StatusBadRequest)
		return
	}

	switch input.Action {
	case "merge":
		if len(input.Selected) == 0 || len(input.Pairs) != 0 {
			http.Error(w, "Select eligible Character groups to merge", http.StatusBadRequest)
			return
		}
		// Selected IDs are checked against the fingerprint-bound plan inside
		// the worker transaction. Never trust client-supplied group members.
		id := rs.jobs.Add(r.Context(), "Merging selected duplicate Characters", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error {
			p.Indefinite()
			return characterdedup.ApplySelected(ctx, rs.repository, input.Fingerprint, input.Selected, func(done, total int) {
				p.SetTotal(total)
				p.SetProcessed(done)
			})
		}))
		writeVisualSimilarityJSON(w, map[string]int{"jobID": id})
	case "exclude", "restore":
		if input.Action == "exclude" && (len(input.Selected) == 0 || len(input.Pairs) != 0) ||
			input.Action == "restore" && (len(input.Selected) != 0 || len(input.Pairs) == 0) {
			http.Error(w, "Invalid not-duplicate selection", http.StatusBadRequest)
			return
		}
		updated, err := characterdedup.ChangeDecisions(r.Context(), rs.repository, input.Fingerprint, input.Action, input.Selected, input.Pairs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeVisualSimilarityJSON(w, updated)
	default:
		http.Error(w, "Invalid Character review action", http.StatusBadRequest)
	}
}
