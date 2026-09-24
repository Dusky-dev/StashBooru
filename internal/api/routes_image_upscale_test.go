package api

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stashapp/stash/pkg/models"
)

type upscaleImageRepository struct {
	models.ImageReaderWriter
	source  *models.Image
	created *models.CreateImageInput
}

func (r *upscaleImageRepository) Find(context.Context, int) (*models.Image, error) {
	return r.source, nil
}

func (r *upscaleImageRepository) GetCustomFields(context.Context, int) (map[string]interface{}, error) {
	return map[string]interface{}{"origin": "fixture"}, nil
}

func (r *upscaleImageRepository) Create(_ context.Context, input *models.CreateImageInput) error {
	input.ID = 200
	r.created = input
	return nil
}

type upscaleFileRepository struct{ models.FileReaderWriter }

func (r *upscaleFileRepository) Create(_ context.Context, file models.File) error {
	file.Base().ID = 100
	return nil
}

type upscaleCopyrightRepository struct {
	models.CopyrightReaderWriter
	imageID  int
	ids      []int
	readErr  error
	writeErr error
}

func (r *upscaleCopyrightRepository) FindByImageID(context.Context, int) ([]*models.Copyright, error) {
	return []*models.Copyright{{ID: 7}, {ID: 8}}, r.readErr
}

func (r *upscaleCopyrightRepository) SetImageCopyrights(_ context.Context, id int, ids []int) error {
	r.imageID, r.ids = id, ids
	return r.writeErr
}

func TestRegisterUpscaledImagePreservesMetadata(t *testing.T) {
	fileID := models.FileID(10)
	artistID, rating := 12, 80
	source := &models.Image{
		ID: 20, PrimaryFileID: &fileID, Title: "Original", Details: "Source details",
		StudioID: &artistID, Rating: &rating, Organized: true,
		URLs:         models.NewRelatedStrings([]string{"https://example.com/source"}),
		PerformerIDs: models.NewRelatedIDs([]int{3}),
		TagIDs:       models.NewRelatedIDs([]int{4}), GalleryIDs: models.NewRelatedIDs([]int{5}),
	}
	for _, failure := range []string{"", "read", "write"} {
		t.Run("copyright_"+failure, func(t *testing.T) {
			images := &upscaleImageRepository{source: source}
			copyrights := &upscaleCopyrightRepository{}
			injected := errors.New("copyright storage failed")
			if failure == "read" {
				copyrights.readErr = injected
			}
			if failure == "write" {
				copyrights.writeErr = injected
			}
			repository := models.Repository{Image: images, File: &upscaleFileRepository{}, Copyright: copyrights}
			derived := &models.ImageFile{BaseFile: &models.BaseFile{CreatedAt: time.Now(), UpdatedAt: time.Now()}}
			err := registerUpscaledImageDerivative(context.Background(), repository, source.ID, fileID, derived)
			if failure != "" {
				if !errors.Is(err, injected) {
					t.Fatalf("expected transaction failure, got %v", err)
				}
				if failure == "read" && images.created != nil {
					t.Fatal("created derivative before loading Copyrights")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			created := images.created
			if created == nil || created.ID == source.ID || !reflect.DeepEqual(created.FileIDs, []models.FileID{100}) {
				t.Fatalf("invalid derivative: %+v", created)
			}
			if created.Title != source.Title || created.Details != source.Details || *created.StudioID != artistID || *created.Rating != rating || !created.Organized ||
				!reflect.DeepEqual(created.URLs.List(), source.URLs.List()) || !reflect.DeepEqual(created.PerformerIDs.List(), source.PerformerIDs.List()) ||
				!reflect.DeepEqual(created.TagIDs.List(), source.TagIDs.List()) || !reflect.DeepEqual(created.GalleryIDs.List(), source.GalleryIDs.List()) || created.CustomFields["origin"] != "fixture" {
				t.Fatalf("derivative lost metadata: %+v", created)
			}
			if copyrights.imageID != created.ID || !reflect.DeepEqual(copyrights.ids, []int{7, 8}) {
				t.Fatalf("Copyrights not attached to derivative: %+v", copyrights)
			}
			if source.ID != 20 || *source.PrimaryFileID != fileID {
				t.Fatal("source identity changed")
			}
		})
	}
}

func TestUpscaledImageFingerprintsPreserveOriginalIdentity(t *testing.T) {
	for _, converted := range []bool{false, true} {
		source := models.Fingerprints{
			{Type: "md5", Fingerprint: "current-md5"},
			{Type: "phash", Fingerprint: int64(-9223372036854775701)},
		}
		wantMD5, wantPhash := "current-md5", "800000000000006b"
		if converted {
			wantMD5, wantPhash = "original-md5", "original-phash"
			source = append(source, models.Fingerprint{Type: "source_md5", Fingerprint: wantMD5}, models.Fingerprint{Type: "source_phash", Fingerprint: wantPhash})
		}
		got := upscaledImageFingerprints(source, "current-md5", "derivative-md5")
		if got.GetString("md5") != "derivative-md5" || got.GetString("source_md5") != wantMD5 || got.GetString("source_phash") != wantPhash || got.For("phash") != nil {
			t.Fatalf("converted=%v: wrong identities: %+v", converted, got)
		}
		if source.GetString("md5") != "current-md5" {
			t.Fatal("source fingerprints were mutated")
		}
	}
	if got := upscaledImageFingerprints(nil, "calculated-md5", "output"); got.GetString("source_md5") != "calculated-md5" {
		t.Fatal("calculated source MD5 was lost")
	}
}
