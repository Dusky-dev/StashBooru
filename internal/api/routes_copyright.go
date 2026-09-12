package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

type CopyrightFinder interface {
	Find(ctx context.Context, id int) (*models.Copyright, error)
	GetImage(ctx context.Context, id int) ([]byte, error)
}

type copyrightRoutes struct {
	routes
	copyrightFinder CopyrightFinder
}

func (rs copyrightRoutes) Routes() chi.Router {
	r := chi.NewRouter()

	r.Route("/{copyrightId}", func(r chi.Router) {
		r.Use(rs.CopyrightCtx)
		r.Get("/image", rs.Image)
	})

	return r
}

func (rs copyrightRoutes) Image(w http.ResponseWriter, r *http.Request) {
	copyright := r.Context().Value(copyrightKey).(*models.Copyright)
	defaultParam := r.URL.Query().Get("default")

	var image []byte
	if defaultParam != "true" {
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			image, err = rs.copyrightFinder.GetImage(ctx, copyright.ID)
			return err
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on fetch copyright image: %v", readTxnErr)
		}
	}

	if len(image) == 0 {
		image = static.ReadAll(static.DefaultTagImage)
	}

	utils.ServeImage(w, r, image)
}

func (rs copyrightRoutes) CopyrightCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		copyrightID, err := strconv.Atoi(chi.URLParam(r, "copyrightId"))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		var copyright *models.Copyright
		_ = rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			copyright, err = rs.copyrightFinder.Find(ctx, copyrightID)
			return err
		})
		if copyright == nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), copyrightKey, copyright)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
