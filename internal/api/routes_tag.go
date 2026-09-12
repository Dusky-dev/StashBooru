package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/stashapp/stash/internal/manager"
	"github.com/stashapp/stash/internal/static"
	"github.com/stashapp/stash/pkg/logger"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/utils"
)

type TagFinder interface {
	models.TagGetter
	GetImage(ctx context.Context, tagID int) ([]byte, error)
}

type tagRoutes struct {
	routes
	tagFinder TagFinder
}

func (rs tagRoutes) Routes() chi.Router {
	r := chi.NewRouter()

	// Copyright is a separate metadata category, but this lightweight image
	// endpoint shares the already-mounted metadata-image router.
	r.Get("/copyright/{copyrightId}/image", rs.CopyrightImage)

	r.Route("/{tagId}", func(r chi.Router) {
		r.Use(rs.TagCtx)
		r.Get("/image", rs.Image)
	})

	return r
}

func (rs tagRoutes) CopyrightImage(w http.ResponseWriter, r *http.Request) {
	copyrightID, err := strconv.Atoi(chi.URLParam(r, "copyrightId"))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	defaultParam := r.URL.Query().Get("default")
	repository := manager.GetInstance().Repository
	var image []byte
	if defaultParam != "true" {
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			image, err = repository.Copyright.GetImage(ctx, copyrightID)
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
		disableEntityFallbackCaching(w)
		var fallbackID int
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			ids, findErr := repository.Copyright.FindImageIDs(ctx, copyrightID)
			if findErr != nil {
				return findErr
			}
			fallbackID = randomID(ids)
			return nil
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on copyright fallback image: %v", readTxnErr)
		} else if fallbackID != 0 {
			http.Redirect(w, r, imageThumbnailURL("", fallbackID), http.StatusFound)
			return
		}

		image = static.ReadAll(static.DefaultTagImage)
	}
	utils.ServeImage(w, r, image)
}

func (rs tagRoutes) Image(w http.ResponseWriter, r *http.Request) {
	tag := r.Context().Value(tagKey).(*models.Tag)
	defaultParam := r.URL.Query().Get("default")

	var image []byte
	if defaultParam != "true" {
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			image, err = rs.tagFinder.GetImage(ctx, tag.ID)
			return err
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on fetch tag image: %v", readTxnErr)
		}
	}

	if len(image) == 0 {
		disableEntityFallbackCaching(w)
		var fallback *models.Image
		readTxnErr := rs.withReadTxn(r, func(ctx context.Context) error {
			filter := &models.ImageFilterType{
				Tags: &models.HierarchicalMultiCriterionInput{
					Value:    []string{strconv.Itoa(tag.ID)},
					Modifier: models.CriterionModifierIncludes,
				},
			}
			var err error
			fallback, err = randomRelatedImage(ctx, manager.GetInstance().Repository.Image, filter)
			return err
		})
		if errors.Is(readTxnErr, context.Canceled) {
			return
		}
		if readTxnErr != nil {
			logger.Warnf("read transaction error on tag fallback image: %v", readTxnErr)
		} else if fallback != nil {
			http.Redirect(w, r, imageThumbnailURL("", fallback.ID), http.StatusFound)
			return
		}

		image = static.ReadAll(static.DefaultTagImage)
	}

	utils.ServeImage(w, r, image)
}

func (rs tagRoutes) TagCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tagID, err := strconv.Atoi(chi.URLParam(r, "tagId"))
		if err != nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		var tag *models.Tag
		_ = rs.withReadTxn(r, func(ctx context.Context) error {
			var err error
			tag, err = rs.tagFinder.Find(ctx, tagID)
			return err
		})
		if tag == nil {
			http.Error(w, http.StatusText(404), 404)
			return
		}

		ctx := context.WithValue(r.Context(), tagKey, tag)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
