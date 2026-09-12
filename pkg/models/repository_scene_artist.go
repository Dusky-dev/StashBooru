package models

import "context"

// SceneArtistReaderWriter manages the many-to-many Artist relationship for
// videos while keeping Scene.StudioID as the backwards-compatible primary
// Artist.
type SceneArtistReaderWriter interface {
	FindBySceneID(ctx context.Context, sceneID int) ([]*Studio, error)
	SetSceneArtists(ctx context.Context, sceneID int, studioIDs []int) error
	AddSceneArtists(ctx context.Context, sceneID int, studioIDs []int) error
}
