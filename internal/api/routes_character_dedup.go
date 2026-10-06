package api

import (
	"context"
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
		Fingerprint string `json:"fingerprint"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || len(input.Fingerprint) != 64 {
		http.Error(w, "A fresh Character merge preview is required", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	id := rs.jobs.Add(r.Context(), "Merging duplicate Characters", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error {
		p.Indefinite()
		return characterdedup.Apply(ctx, rs.repository, input.Fingerprint, func(done, total int) { p.SetTotal(total); p.SetProcessed(done) })
	}))
	writeVisualSimilarityJSON(w, map[string]int{"jobID": id})
}
