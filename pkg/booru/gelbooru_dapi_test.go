package booru

import (
	"errors"
	"testing"
)

// Shapes observed from the Gelbooru 0.2 DAPI used by Rule34 and Safebooru.
func TestParseGelbooruDAPIShapes(t *testing.T) {
	source := testSource("gelbooru")
	got, err := parse(source, []byte(`[{"id":7340440,"hash":"5A5745314DEF514BC3154F295DB5E476","tags":"1girl","file_url":"https://cdn.example.org/a.jpg"}]`), 1, 1)
	if err != nil || len(got.Posts) != 1 || got.Posts[0].MD5 != "5a5745314def514bc3154f295db5e476" {
		t.Fatalf("the checksum is read from hash: %+v %v", got.Posts, err)
	}

	got, err = parse(source, []byte("  \n"), 1, 1)
	if err != nil || len(got.Posts) != 0 || got.HasMore {
		t.Fatalf("an empty body means no posts: %+v %v", got, err)
	}

	got, err = parse(source, []byte(`{"@attributes":{"limit":1,"offset":0,"count":0}}`), 1, 1)
	if err != nil || len(got.Posts) != 0 {
		t.Fatalf("gelbooru.com without a post array means no posts: %+v %v", got, err)
	}

	_, err = parse(source, []byte(`"Missing authentication. Go to api.rule34.xxx for more information"`), 1, 1)
	var providerErr *Error
	if !errors.As(err, &providerErr) || providerErr.Code != "authentication" || providerErr.Message != "Provider said: Missing authentication. Go to api.rule34.xxx for more information" {
		t.Fatalf("a bare string is the provider's own error: %#v", err)
	}
	_, err = parse(source, []byte(`"Search down for maintenance"`), 1, 1)
	if !errors.As(err, &providerErr) || providerErr.Code != "provider-error" {
		t.Fatalf("got %#v", err)
	}
}
