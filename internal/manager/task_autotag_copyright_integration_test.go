//go:build integration

package manager

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sqlite"
	_ "github.com/stashapp/stash/pkg/sqlite/migrations"
	"github.com/stretchr/testify/require"
)

func TestCopyrightTaskAutoTag(t *testing.T) {
	for _, selection := range []string{"wildcard", "specific", "omitted", "invalid", "missing"} {
		t.Run(selection, func(t *testing.T) {
			config.InitializeEmpty()
			db := sqlite.NewDatabase()
			require.NoError(t, db.Open(filepath.Join(t.TempDir(), "autotag.sqlite")))
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			r := db.Repository()
			ctx := context.Background()
			var copyrightID int
			root := t.TempDir()
			require.NoError(t, r.WithTxn(ctx, func(ctx context.Context) error {
				c, err := r.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Main Series", Aliases: []string{"Alias Series"}})
				if err != nil {
					return err
				}
				copyrightID = c.ID
				if _, err := r.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Other Series"}); err != nil {
					return err
				}
				for _, dir := range []string{"selected", "outside"} {
					now := time.Now()
					folder := &models.Folder{Path: filepath.Join(root, dir), CreatedAt: now, UpdatedAt: now}
					if err := r.Folder.Create(ctx, folder); err != nil {
						return err
					}
					image := models.NewImage()
					if err := r.Image.Create(ctx, &models.CreateImageInput{Image: &image}); err != nil {
						return err
					}
					scene := models.NewScene()
					if err := r.Scene.Create(ctx, &scene, nil); err != nil {
						return err
					}
					imageFile := &models.ImageFile{BaseFile: &models.BaseFile{Basename: "Alias Series.png", Path: filepath.Join(folder.Path, "Alias Series.png"), ParentFolderID: folder.ID, CreatedAt: now, UpdatedAt: now}, Format: "png", Width: 10, Height: 10}
					videoFile := &models.VideoFile{BaseFile: &models.BaseFile{Basename: "Main Series.mp4", Path: filepath.Join(folder.Path, "Main Series.mp4"), ParentFolderID: folder.ID, CreatedAt: now, UpdatedAt: now}, Format: "mp4", Width: 10, Height: 10, Duration: 1}
					if err := r.File.Create(ctx, imageFile); err != nil {
						return err
					}
					if err := r.File.Create(ctx, videoFile); err != nil {
						return err
					}
					if err := r.Image.AddFileID(ctx, image.ID, imageFile.ID); err != nil {
						return err
					}
					if err := r.Scene.AddFileID(ctx, scene.ID, videoFile.ID); err != nil {
						return err
					}
					if _, err := r.Image.UpdatePartial(ctx, image.ID, models.ImagePartial{PrimaryFileID: &imageFile.ID}); err != nil {
						return err
					}
					if _, err := r.Scene.UpdatePartial(ctx, scene.ID, models.ScenePartial{PrimaryFileID: &videoFile.ID}); err != nil {
						return err
					}
				}
				return nil
			}))
			input := AutoTagMetadataInput{Paths: []string{filepath.Join(root, "selected")}}
			switch selection {
			case "wildcard":
				input.Copyrights = []string{"*"}
			case "specific":
				input.Copyrights = []string{strconv.Itoa(copyrightID), strconv.Itoa(copyrightID)}
			case "invalid":
				input.Copyrights = []string{"no"}
			case "missing":
				input.Copyrights = []string{"99999"}
			}
			m := job.NewManager()
			defer m.Stop()
			result := make(chan error, 1)
			m.Add(ctx, "Copyright test", job.MakeJobExec(func(ctx context.Context, p *job.Progress) error {
				err := (&autoTagJob{repository: r, input: input}).Execute(ctx, p)
				result <- err
				return err
			}))
			select {
			case err := <-result:
				if selection == "invalid" || selection == "missing" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("job did not finish")
			}
			require.NoError(t, r.WithReadTxn(ctx, func(ctx context.Context) error {
				for _, id := range []int{1, 2} {
					cs, err := r.Copyright.FindByImageID(ctx, id)
					require.NoError(t, err)
					videos, err := r.Copyright.FindBySceneID(ctx, id)
					require.NoError(t, err)
					if id == 1 && (selection == "wildcard" || selection == "specific") {
						require.Len(t, cs, 1)
						require.Equal(t, copyrightID, cs[0].ID)
						require.Len(t, videos, 1)
					} else {
						require.Empty(t, cs)
						require.Empty(t, videos)
					}
				}
				return nil
			}))
		})
	}
}
