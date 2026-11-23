package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stashapp/stash/pkg/booru"
)

type creatorIconClient struct {
	asked []string
	err   error
}

func (c *creatorIconClient) Search(context.Context, booru.Source, string, int, int) (booru.Result, error) {
	return booru.Result{}, nil
}

func (c *creatorIconClient) FetchMedia(_ context.Context, _ booru.Source, input booru.MediaRequest, output io.Writer) (booru.MediaInfo, error) {
	c.asked = append(c.asked, input.URL)
	if c.err != nil {
		return booru.MediaInfo{}, c.err
	}
	if !input.ImagesOnly || input.Limit > 4<<20 {
		return booru.MediaInfo{}, errors.New("icon requests must be bounded images")
	}
	_, _ = output.Write([]byte("RIFF....WEBP"))
	return booru.MediaInfo{ContentType: "image/webp"}, nil
}

func (c *creatorIconClient) CreatorIcon(_ context.Context, source booru.Source, target string) (string, error) {
	if source.Engine != "pixiv" {
		return booru.CreatorIconURL(source, target)
	}
	switch target {
	case "user:11":
		return "https://i.pximg.net/user-profile/img/11_170.jpg", nil
	case "user:12":
		return "", booru.ErrNoCreatorPicture
	}
	return "", &booru.Error{Code: "authentication", Message: "Pixiv requires an OAuth access token."}
}

func TestCreatorIconRoute(t *testing.T) {
	var kemono booru.Source
	for _, source := range booru.Defaults() {
		if source.Engine == "kemono" {
			kemono = source
		}
	}
	if kemono.ID == "" {
		t.Skip("no default Kemono source")
	}
	client := &creatorIconClient{}
	rs := booruBrowserRoutes{configPath: t.TempDir(), client: client}
	get := func(instance, target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/booru-browser/creator-icon?instance="+url.QueryEscape(instance)+"&target="+url.QueryEscape(target), nil)
		rs.CreatorIcon(w, r)
		return w
	}
	w := get(kemono.Instance(), "creator:patreon:49481727")
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/webp" || len(client.asked) != 1 || client.asked[0] != "https://kemono.cr/icons/patreon/49481727" {
		t.Fatalf("got %d %q, asked %v", w.Code, w.Body.String(), client.asked)
	}
	if w := get(kemono.Instance(), "some_tag"); w.Code != 400 {
		t.Fatalf("a tag identity has no picture: %d", w.Code)
	}
	if w := get("https://unknown.example", "creator:patreon:1"); w.Code != 404 {
		t.Fatalf("unknown source: %d", w.Code)
	}
	client.err = errors.New("media provider returned HTTP 404")
	if w := get(kemono.Instance(), "creator:patreon:2"); w.Code != 404 {
		t.Fatalf("a creator without a picture is reported plainly: %d %s", w.Code, w.Body.String())
	}

	var pixiv booru.Source
	for _, source := range booru.Defaults() {
		if source.Engine == "pixiv" {
			pixiv = source
		}
	}
	client.err, client.asked = nil, nil
	if w := get(pixiv.Instance(), "user:11"); w.Code != 200 || len(client.asked) != 1 || client.asked[0] != "https://i.pximg.net/user-profile/img/11_170.jpg" {
		t.Fatalf("Pixiv avatar: %d %s %v", w.Code, w.Body.String(), client.asked)
	}
	if w := get(pixiv.Instance(), "user:12"); w.Code != 404 {
		t.Fatalf("Pixiv default avatar: %d", w.Code)
	}
	if w := get(pixiv.Instance(), "user:13"); w.Code != 502 || !strings.Contains(w.Body.String(), "OAuth") {
		t.Fatalf("Pixiv provider errors are shown: %d %s", w.Code, w.Body.String())
	}
}
