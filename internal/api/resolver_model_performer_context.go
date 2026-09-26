package api

import (
	"context"

	"github.com/stashapp/stash/internal/api/loaders"
	"github.com/stashapp/stash/pkg/models"
)

func (r *performerDisambiguationContextResolver) Copyright(ctx context.Context, obj *models.PerformerDisambiguationContext) (*models.Copyright, error) {
	if obj.CopyrightID == nil {
		return nil, nil
	}
	return loaders.From(ctx).CopyrightByID.Load(*obj.CopyrightID)
}

func (r *performerDisambiguationContextResolver) Artist(ctx context.Context, obj *models.PerformerDisambiguationContext) (*models.Studio, error) {
	if obj.ArtistID == nil {
		return nil, nil
	}
	return loaders.From(ctx).StudioByID.Load(*obj.ArtistID)
}
