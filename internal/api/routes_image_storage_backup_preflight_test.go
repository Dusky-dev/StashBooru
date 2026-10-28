package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStorageBackupPreflightRequestRequiresStackAndSnapshot(t *testing.T) {
	good := storageBackupPreflightRequest{
		StackID: 51, Snapshot: "20261029T102030Z-012345abcdef",
	}
	if err := validateStorageBackupPreflightRequest(good); err != nil {
		t.Fatalf("valid backup preflight was rejected: %v", err)
	}
	for _, test := range []storageBackupPreflightRequest{
		{StackID: 0, Snapshot: good.Snapshot},
		{StackID: -4, Snapshot: good.Snapshot},
		{StackID: 1, Snapshot: ""},
		{StackID: 1, Snapshot: "../other"},
		{StackID: 1, Snapshot: "20261029T102030Z-012345abcdeg"},
		{StackID: 1, Snapshot: "20261029T102030Z-012345abcdef.json"},
	} {
		if err := validateStorageBackupPreflightRequest(test); err == nil {
			t.Fatalf("accepted unsafe preflight selection: %+v", test)
		}
	}
}

func preflightExampleReport() map[string]any {
	return map[string]any{
		"experimental": true,
		"readOnly": true,
		"verified": true,
		"mediaBackupVerified": true,
		"snapshot": "20261029T102030Z-012345abcdef",
		"group": "stack:7",
		"selectedImagesCovered": 2,
		"snapshotFilesVerified": 3,
		"snapshotObjectsVerified": 3,
		"imageSHA256": map[string]string{
			"image:1": strings.Repeat("a", 64),
			"image:2": strings.Repeat("b", 64),
		},
		"sourcesRetained": true,
		"nativePackingEnabled": false,
		"nativeServingVerified": false,
		"databaseBackupVerified": false,
		"originalEvictionEnabled": false,
	}
}

func preflightTestGroup() estimateSourceGroup {
	return estimateSourceGroup{
		ID: "stack:7",
		Representative: "image:1",
		Members: []estimateSourceMember{
			{ID: "image:1", Path: "/unused/image1.png"},
			{ID: "image:2", Path: "/unused/image2.png"},
		},
	}
}

func assertRejectsStorageBackupPreflightReport(t *testing.T, report map[string]any) {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateStorageBackupPreflightReport(
		data, preflightTestGroup(), "20261029T102030Z-012345abcdef",
	); err == nil {
		t.Fatalf("accepted invalid native backup preflight report: %+v", report)
	}
}

func TestStorageBackupPreflightValidatesReportIdentityAndSafety(t *testing.T) {
	report := preflightExampleReport()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := validateStorageBackupPreflightReport(
		data, preflightTestGroup(), "20261029T102030Z-012345abcdef",
	)
	if err != nil || len(raw) != len(data) {
		t.Fatalf("verified report was rejected: bytes=%d, error=%v", len(raw), err)
	}
	for _, field := range []string{
		"experimental", "readOnly", "verified", "mediaBackupVerified", "sourcesRetained",
	} {
		t.Run("require_"+field, func(t *testing.T) {
			broken := preflightExampleReport()
			broken[field] = false
			assertRejectsStorageBackupPreflightReport(t, broken)
		})
	}
	for _, field := range []string{
		"nativePackingEnabled", "nativeServingVerified",
		"databaseBackupVerified", "originalEvictionEnabled",
	} {
		t.Run("reject_"+field, func(t *testing.T) {
			broken := preflightExampleReport()
			broken[field] = true
			assertRejectsStorageBackupPreflightReport(t, broken)
		})
	}
	for _, key := range []string{"snapshot", "group"} {
		t.Run("mismatched_"+key, func(t *testing.T) {
			broken := preflightExampleReport()
			broken[key] = "different"
			assertRejectsStorageBackupPreflightReport(t, broken)
		})
	}
	for _, field := range []string{"selectedImagesCovered", "snapshotFilesVerified", "snapshotObjectsVerified"} {
		t.Run("bad_"+field, func(t *testing.T) {
			broken := preflightExampleReport()
			broken[field] = 0
			assertRejectsStorageBackupPreflightReport(t, broken)
		})
	}
}

func TestStorageBackupPreflightRejectsMissingOrInvalidImageDigests(t *testing.T) {
	for _, digests := range []map[string]string{
		{"image:1": strings.Repeat("a", 64)},
		{"image:1": strings.Repeat("a", 64), "image:other": strings.Repeat("b", 64)},
		{"image:1": strings.Repeat("a", 64), "image:2": "../unsafe"},
		{"image:1": strings.Repeat("a", 64), "image:2": strings.Repeat("f", 63)},
	} {
		report := preflightExampleReport()
		report["imageSHA256"] = digests
		assertRejectsStorageBackupPreflightReport(t, report)
	}
	if _, err := validateStorageBackupPreflightReport(nil, preflightTestGroup(), "20261029T102030Z-012345abcdef"); err == nil {
		t.Fatal("accepted an empty preflight report")
	}
	if _, err := validateStorageBackupPreflightReport(
		[]byte(strings.Repeat("X", 128*1024+1)),
		preflightTestGroup(), "20261029T102030Z-012345abcdef",
	); err == nil {
		t.Fatal("accepted oversized preflight result")
	}
}

func TestStorageBackupPreflightRequiresExplicitSafetyFlags(t *testing.T) {
	for _, field := range []string{
		"experimental", "readOnly", "verified", "mediaBackupVerified", "sourcesRetained",
		"nativePackingEnabled", "nativeServingVerified",
		"databaseBackupVerified", "originalEvictionEnabled",
	} {
		t.Run("missing_"+field, func(t *testing.T) {
			report := preflightExampleReport()
			delete(report, field)
			assertRejectsStorageBackupPreflightReport(t, report)
		})
		t.Run("null_"+field, func(t *testing.T) {
			report := preflightExampleReport()
			report[field] = nil
			assertRejectsStorageBackupPreflightReport(t, report)
		})
	}
}
