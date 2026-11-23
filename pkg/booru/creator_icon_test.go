package booru

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestCreatorIconURL(t *testing.T) {
	kemono := Source{ID: "kemono", Name: "Kemono", Engine: "kemono", URL: "https://kemono.cr"}
	got, err := CreatorIconURL(kemono, "creator:patreon:49481727")
	if err != nil || got != "https://kemono.cr/icons/patreon/49481727" {
		t.Fatalf("got %q %v", got, err)
	}
	pawchive := Source{ID: "pawchive", Name: "Pawchive", Engine: "pawchive", URL: "https://pawchive.pw/"}
	if got, err = CreatorIconURL(pawchive, "creator:fanbox:7"); err != nil || got != "https://pawchive.pw/icons/fanbox/7" {
		t.Fatalf("got %q %v", got, err)
	}
	// Redirect targets on the img. host stay within the source's media hosts.
	if kemono.mediaURL("https://img.kemono.cr/icons/patreon/49481727") == "" {
		t.Fatal("Kemono's icon host must be an approved media host")
	}
	for _, target := range []string{"artist_name", "creator:patreon", "creator:../x:1", "creator:a:b:c"} {
		if _, err := CreatorIconURL(kemono, target); err == nil {
			t.Fatalf("%q must be refused", target)
		}
	}
	if _, err := CreatorIconURL(Source{ID: "d", Engine: "danbooru", URL: "https://danbooru.donmai.us"}, "creator:patreon:1"); err == nil {
		t.Fatal("tag sites have no creator pictures")
	}
}

func TestPixivCreatorIcon(t *testing.T) {
	pixiv := Source{ID: "pixiv", Name: "Pixiv", Engine: "pixiv", URL: "https://app-api.pixiv.net", MediaHosts: []string{"pximg.net"}, APIKey: "token"}
	avatar := "https://i.pximg.net/user-profile/img/2020/01/01/00/00/00/123_abc_170.jpg"
	var asked []string
	client := testClient(func(r *http.Request) (*http.Response, error) {
		asked = append(asked, r.URL.Path+"?"+r.URL.RawQuery+" "+r.Header.Get("Authorization"))
		switch r.URL.Query().Get("user_id") {
		case "11":
			return response(200, `{"user":{"id":11,"profile_image_urls":{"medium":"`+avatar+`"}}}`), nil
		case "12":
			return response(200, `{"user":{"id":12,"profile_image_urls":{"medium":"https://s.pximg.net/common/images/no_profile.png"}}}`), nil
		default:
			return response(200, `{"user":{"id":13,"profile_image_urls":{"medium":"https://evil.example/x.jpg"}}}`), nil
		}
	})
	ctx := context.Background()
	if got, err := client.CreatorIcon(ctx, pixiv, "user:11"); err != nil || got != avatar {
		t.Fatalf("got %q %v", got, err)
	}
	if len(asked) != 1 || !strings.HasPrefix(asked[0], "/v1/user/detail?") || !strings.Contains(asked[0], "user_id=11") || !strings.HasSuffix(asked[0], "Bearer token") {
		t.Fatalf("asked %v", asked)
	}
	if _, err := client.CreatorIcon(ctx, pixiv, "user:12"); !errors.Is(err, ErrNoCreatorPicture) {
		t.Fatalf("the default avatar is no picture: %v", err)
	}
	if _, err := client.CreatorIcon(ctx, pixiv, "user:13"); err == nil {
		t.Fatal("an avatar off Pixiv's media hosts must be refused")
	}
	for _, target := range []string{"artist_name", "user:abc", "creator:patreon:1"} {
		if _, err := client.CreatorIcon(ctx, pixiv, target); err == nil {
			t.Fatalf("%q must be refused", target)
		}
	}
	pixiv.APIKey = ""
	if _, err := client.CreatorIcon(ctx, pixiv, "user:11"); err == nil {
		t.Fatal("Pixiv needs a token")
	}
	kemono := Source{ID: "kemono", Name: "Kemono", Engine: "kemono", URL: "https://kemono.cr"}
	if got, err := client.CreatorIcon(ctx, kemono, "creator:patreon:1"); err != nil || got != "https://kemono.cr/icons/patreon/1" {
		t.Fatalf("archive creators keep the fixed address: %q %v", got, err)
	}
}
