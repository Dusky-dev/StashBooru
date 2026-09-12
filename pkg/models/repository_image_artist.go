package models

import "context"

// ImageArtistReaderWriter manages the many-to-many Artist relationship for
// images while keeping Image.StudioID as the backwards-compatible primary
// Artist.
type ImageArtistReaderWriter interface {
	FindByImageID(ctx context.Context, imageID int) ([]*Studio, error)
	SetImageArtists(ctx context.Context, imageID int, studioIDs []int) error
	AddImageArtists(ctx context.Context, imageID int, studioIDs []int) error
}
