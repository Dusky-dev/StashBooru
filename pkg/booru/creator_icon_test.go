package booru

import "testing"

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
