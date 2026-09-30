//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stashapp/stash/internal/manager/config"
	"github.com/stashapp/stash/pkg/job"
	"github.com/stashapp/stash/pkg/models"
	"github.com/stretchr/testify/require"
)

func (f *p06SQLiteFixture) review(t *testing.T, proposed config.AssociationInheritanceSettings) *associationInheritanceReport {
	t.Helper()
	var report *associationInheritanceReport
	current := config.GetInstance().GetAssociationInheritanceSettings()
	require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		var err error
		report, err = scanAssociationInheritanceReview(ctx, f.repository, current, proposed, nil)
		return err
	}))
	return report
}

func TestP06ReviewedInheritanceActivationPreservesDirectLinks(t *testing.T) {
	f := newP06SQLiteFixture(t)
	cfg := config.GetInstance()
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yml"))
	cfg.SetBool(config.AssociationInheritanceTags, false)
	before := cfg.GetAssociationInheritanceSettings()
	proposed := before
	proposed.Tags = true
	report := f.review(t, proposed)
	require.Equal(t, 1, report.ImagesReviewed)
	require.Equal(t, 1, report.VideosReviewed)
	require.Equal(t, 2, report.AffectedMedia)
	require.Zero(t, report.ErrorCount)
	require.Equal(t, 2, report.Domains["tag"].Added)
	require.Len(t, report.Samples, 2)
	require.Equal(t, before, cfg.GetAssociationInheritanceSettings(), "preview must not save settings")
	require.NoError(t, applyAssociationInheritanceReview(context.Background(), f.repository, cfg, report, nil))
	require.Equal(t, proposed, cfg.GetAssociationInheritanceSettings())
	for _, video := range []bool{false, true} {
		direct := f.direct(t, video)
		require.Empty(t, direct.tags)
		require.Equal(t, []int{f.artist.ID}, direct.artists)
	}

	// An explicit Tag remains explicit when its profile source is detached.
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		if err := f.repository.Image.UpdateTags(ctx, f.image.ID, []int{f.parentTag.ID}); err != nil {
			return err
		}
		_, err := f.repository.Scene.UpdatePartial(ctx, f.video.ID, models.ScenePartial{TagIDs: &models.UpdateIDs{
			IDs: []int{f.parentTag.ID}, Mode: models.RelationshipUpdateModeSet,
		}})
		if err != nil {
			return err
		}
		if err := f.repository.ImageArtist.SetImageArtists(ctx, f.image.ID, nil); err != nil {
			return err
		}
		return f.repository.SceneArtist.SetSceneArtists(ctx, f.video.ID, nil)
	}))
	for _, video := range []bool{false, true} {
		direct := f.direct(t, video)
		require.Equal(t, []int{f.parentTag.ID}, direct.tags)
		require.Empty(t, direct.artists)
		require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
			ids, provenance, err := effectiveAssociationProvenance(ctx, f.repository, direct, proposed)
			if err != nil {
				return err
			}
			require.Equal(t, []int{f.parentTag.ID}, ids.tags)
			assertHasMediaAssociationOrigin(t, provenance, "tag", f.parentTag.ID, MediaAssociationOrigin{
				Kind: "direct", SourceType: "tag", SourceID: f.parentTag.ID, SourceTagID: f.parentTag.ID,
			})
			return nil
		}))
	}
}

func TestP06InheritanceReviewRejectsStaleProfilesAndReparenting(t *testing.T) {
	for _, change := range []string{"profile", "reparent", "direct"} {
		t.Run(change, func(t *testing.T) {
			f := newP06SQLiteFixture(t)
			cfg := config.GetInstance()
			cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yml"))
			before := cfg.GetAssociationInheritanceSettings()
			proposed := before
			proposed.Tags = false
			report := f.review(t, proposed)
			require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
				switch change {
				case "profile":
					f.artist.TagIDs = models.NewRelatedIDs([]int{})
					return f.repository.Studio.Update(ctx, &models.UpdateStudioInput{Studio: f.artist})
				case "reparent":
					parent := models.NewStudio()
					parent.Name = "New main Artist"
					if err := f.repository.Studio.Create(ctx, &models.CreateStudioInput{Studio: &parent}); err != nil {
						return err
					}
					_, err := f.repository.Studio.UpdatePartial(ctx, models.StudioPartial{ID: f.artist.ID, ParentID: models.NewOptionalInt(parent.ID)})
					return err
				default:
					return f.repository.Image.UpdateTags(ctx, f.image.ID, []int{f.parentTag.ID})
				}
			}))
			require.ErrorContains(t, applyAssociationInheritanceReview(context.Background(), f.repository, cfg, report, nil), "changed since preview")
			require.Equal(t, before, cfg.GetAssociationInheritanceSettings())
		})
	}
}

func TestP06InvalidCopyrightEditLeavesReviewedSnapshotIntact(t *testing.T) {
	f := newP06SQLiteFixture(t)
	var root, child *models.Copyright
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		var err error
		root, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Root"})
		if err != nil {
			return err
		}
		child, err = f.repository.Copyright.Create(ctx, models.CopyrightCreateInput{Name: "Child", ParentIDs: []string{strconv.Itoa(root.ID)}})
		if err != nil {
			return err
		}
		return f.repository.Copyright.SetImageCopyrights(ctx, f.image.ID, []int{child.ID})
	}))
	proposed := config.GetInstance().GetAssociationInheritanceSettings()
	report := f.review(t, proposed)
	invalidName := "Must roll back"
	err := f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		_, err := f.repository.Copyright.Update(ctx, models.CopyrightUpdateInput{
			ID: strconv.Itoa(root.ID), Name: &invalidName, ParentIDs: []string{strconv.Itoa(child.ID)},
		})
		return err
	})
	require.Error(t, err)
	after := f.review(t, proposed)
	require.Equal(t, report.Fingerprint, after.Fingerprint)
	require.NoError(t, f.repository.WithReadTxn(context.Background(), func(ctx context.Context) error {
		item, err := f.repository.Copyright.Find(ctx, root.ID)
		if err != nil {
			return err
		}
		require.Equal(t, "Root", item.Name)
		return nil
	}))
}

func TestP06InheritanceReviewPagesAndCancellation(t *testing.T) {
	f := newP06SQLiteFixture(t)
	require.NoError(t, f.repository.WithTxn(context.Background(), func(ctx context.Context) error {
		for index := 0; index < 105; index++ {
			image := models.NewImage()
			image.StudioID = &f.artist.ID
			if err := f.repository.Image.Create(ctx, &models.CreateImageInput{Image: &image}); err != nil {
				return err
			}
			video := models.NewScene()
			video.StudioID = &f.artist.ID
			if err := f.repository.Scene.Create(ctx, &video, nil); err != nil {
				return err
			}
		}
		return nil
	}))
	proposed := config.GetInstance().GetAssociationInheritanceSettings()
	report := f.review(t, proposed)
	require.Equal(t, 106, report.ImagesReviewed)
	require.Equal(t, 106, report.VideosReviewed)
	require.Len(t, report.Samples, associationInheritanceSampleLimit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := f.repository.WithReadTxn(ctx, func(ctx context.Context) error {
		_, err := scanAssociationInheritanceReview(ctx, f.repository, proposed, proposed, func(processed, _ int) {
			if processed == 1 {
				cancel()
			}
		})
		return err
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, proposed, config.GetInstance().GetAssociationInheritanceSettings())
}

type p06FailingImageTags struct {
	models.ImageReaderWriter
}

func (p06FailingImageTags) GetTagIDs(context.Context, int) ([]int, error) {
	return nil, errors.New("injected Image Tag read failure")
}

func TestP06InheritanceReviewReportsPerItemErrorsAndBlocksApply(t *testing.T) {
	f := newP06SQLiteFixture(t)
	f.repository.Image = p06FailingImageTags{ImageReaderWriter: f.repository.Image}
	proposed := config.GetInstance().GetAssociationInheritanceSettings()
	report := f.review(t, proposed)
	require.Equal(t, 1, report.ErrorCount)
	require.Len(t, report.ErrorSamples, 1)
	require.Contains(t, report.ErrorSamples[0], "injected Image Tag read failure")
	require.ErrorContains(t, applyAssociationInheritanceReview(context.Background(), f.repository, config.GetInstance(), report, nil), "successful inheritance preview")
}

func p06ReviewPost(t *testing.T, routes *associationInheritanceRoutes, payload any) associationInheritanceResponse {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	routes.Post(response, httptest.NewRequest(http.MethodPost, "/association-inheritance", bytes.NewReader(body)))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var result associationInheritanceResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	return result
}

func TestP06InheritanceReviewHTTPJobsApplyAndQueuedCancellation(t *testing.T) {
	f := newP06SQLiteFixture(t)
	cfg := config.GetInstance()
	cfg.SetConfigFile(filepath.Join(t.TempDir(), "config.yml"))
	jobs := job.NewManager()
	t.Cleanup(jobs.Stop)
	routes := newAssociationInheritanceRoutes(f.repository, cfg, jobs)
	proposed := cfg.GetAssociationInheritanceSettings()
	proposed.Tags = false
	queued := p06ReviewPost(t, routes, map[string]any{"action": "preview", "proposed": proposed})
	require.Positive(t, queued.JobID)
	state := routes.find(queued.ReviewID)
	require.NotNil(t, state)
	require.Eventually(t, func() bool { return routes.snapshot(state).Status == "ready" }, 5*time.Second, 5*time.Millisecond)
	p06ReviewPost(t, routes, map[string]any{"action": "apply", "reviewID": queued.ReviewID})
	require.Eventually(t, func() bool { return routes.snapshot(state).Status == "applied" }, 5*time.Second, 5*time.Millisecond)
	require.Equal(t, proposed, cfg.GetAssociationInheritanceSettings())
	repeated := p06ReviewPost(t, routes, map[string]any{"action": "apply", "reviewID": queued.ReviewID})
	require.Equal(t, "applied", repeated.Status)
	require.Empty(t, f.direct(t, false).tags)
	require.Empty(t, f.direct(t, true).tags)

	blocked := make(chan struct{})
	started := make(chan struct{})
	jobs.Add(context.Background(), "block test queue", job.MakeJobExec(func(ctx context.Context, _ *job.Progress) error {
		close(started)
		select {
		case <-blocked:
		case <-ctx.Done():
		}
		return nil
	}))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("native test job did not start")
	}
	t.Cleanup(func() { close(blocked) })
	cancelled := p06ReviewPost(t, routes, map[string]any{"action": "preview", "proposed": proposed})
	jobs.CancelJob(cancelled.JobID)
	response := httptest.NewRecorder()
	routes.Get(response, httptest.NewRequest(http.MethodGet, "/association-inheritance?reviewID="+cancelled.ReviewID, nil))
	require.Equal(t, http.StatusOK, response.Code)
	var status associationInheritanceResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &status))
	require.Equal(t, "cancelled", status.Status, "cancellation from the native Jobs UI must update the review")
}
