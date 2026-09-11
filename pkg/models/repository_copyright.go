package models

import "context"

// CopyrightReaderWriter provides storage and relationship operations for the
// native Copyright category.
type CopyrightReaderWriter interface {
	Find(ctx context.Context, id int) (*Copyright, error)
	FindMany(ctx context.Context, ids []int) ([]*Copyright, error)
	FindByName(ctx context.Context, name string, nocase bool) (*Copyright, error)
	FindByAlias(ctx context.Context, alias string, nocase bool) (*Copyright, error)
	Query(ctx context.Context, filter *FindFilterType) ([]*Copyright, int, error)

	Create(ctx context.Context, input CopyrightCreateInput) (*Copyright, error)
	Update(ctx context.Context, input CopyrightUpdateInput) (*Copyright, error)
	Destroy(ctx context.Context, id int) error

	FindParents(ctx context.Context, id int) ([]*Copyright, error)
	FindChildren(ctx context.Context, id int) ([]*Copyright, error)
	ImageCount(ctx context.Context, id int) (int, error)
	SceneCount(ctx context.Context, id int) (int, error)

	FindByImageID(ctx context.Context, imageID int) ([]*Copyright, error)
	FindBySceneID(ctx context.Context, sceneID int) ([]*Copyright, error)
	SetImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error
	AddImageCopyrights(ctx context.Context, imageID int, copyrightIDs []int) error
	SetSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error
	AddSceneCopyrights(ctx context.Context, sceneID int, copyrightIDs []int) error
}
