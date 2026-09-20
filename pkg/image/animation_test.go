package image

import (
	"context"
	"testing"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/models/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAnimatedTagOnlyAddsRelationship(t *testing.T) {
	db := mocks.NewDatabase()
	db.Tag.On("FindByName", mock.Anything, "animated", true).Return(&models.Tag{ID: 42, Name: "Animated"}, nil).Once()
	db.Image.On("UpdatePartial", mock.Anything, 7, mock.MatchedBy(func(p models.ImagePartial) bool {
		return p.TagIDs != nil && p.TagIDs.Mode == models.RelationshipUpdateModeAdd && len(p.TagIDs.IDs) == 1 && p.TagIDs.IDs[0] == 42 && p.PrimaryFileID == nil
	})).Return(&models.Image{ID: 7}, nil).Once()
	require.NoError(t, AddAnimatedTag(context.Background(), db.Tag, db.Image, []int{7}))
	db.Tag.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	db.Image.AssertExpectations(t)
}
