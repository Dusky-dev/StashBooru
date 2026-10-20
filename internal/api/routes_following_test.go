package api

import (
	"testing"

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
