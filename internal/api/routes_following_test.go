package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"

	"github.com/stretchr/testify/require"
)

func TestFollowingDurableMediaURL(t *testing.T) {
	const stable = "https://cdn.example.org/image.jpg"
	require.Equal(t, stable, followingDurableMediaURL(stable))
	for _, raw := range []string{
		stable + "?X-Amz-Credential=private&X-Amz-Signature=secret",
		stable + "?X-Goog-Credential=private&X-Goog-Signature=secret",
		stable + "?Policy=private&Signature=secret&Key-Pair-Id=private",
		stable + "?sig=secret",
		stable + "?%58-Amz-Signature=secret",
		stable + "?width=300",
		stable + "?",
		stable + "#secret",
		"https://private:secret@cdn.example.org/image.jpg",
		"http://cdn.example.org/image.jpg",
		"//cdn.example.org/image.jpg",
		"/image.jpg",
		"https://cdn.example.org/%zz",
		"",
	} {
		require.Empty(t, followingDurableMediaURL(raw), raw)
	}
}

func TestFollowingSnapshotTrimsLargeAttachmentLists(t *testing.T) {
	post := booru.Post{ID: "a1", Title: "Large archive post"}
	for i := range 400 {
		post.Attachments = append(post.Attachments, booru.Post{ID: fmt.Sprintf("a1-%d", i), PreviewURL: "https://img.example.org/" + strings.Repeat("x", 200)})
	}
	payload, err := followingSnapshotPayload(post)
	if err != nil || len(payload) > models.MaxFollowingPayload {
		t.Fatalf("payload %d bytes, %v", len(payload), err)
	}
	var stored followingStoredPost
	if json.Unmarshal(payload, &stored) != nil || len(stored.Post.Attachments) == 0 || len(stored.Post.Attachments) >= 400 {
		t.Fatalf("attachments kept: %d", len(stored.Post.Attachments))
	}
}
