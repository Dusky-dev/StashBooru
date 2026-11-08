package api

import "testing"

func TestEffectiveBooruFilenameTemplateSharesValidatedLayout(t *testing.T) {
	shared := "%copyright% - %character% - %artist%_%md5%.%ext%"
	ui := map[string]interface{}{"imageFilenameLayout": shared}
	if got := effectiveBooruFilenameTemplate("", ui); got != shared {
		t.Fatalf("shared filename layout was not used: %q", got)
	}
	if got := effectiveBooruFilenameTemplate("%id%.%ext%", ui); got != "%id%.%ext%" {
		t.Fatalf("legacy download override was lost: %q", got)
	}
	for _, invalid := range []interface{}{"/unsafe/%md5%.%ext%", "%artist%/%bogus%.%ext%", 12} {
		if got := effectiveBooruFilenameTemplate("", map[string]interface{}{"imageFilenameLayout": invalid}); got != "" {
			t.Fatalf("invalid shared layout was used: %q", got)
		}
	}
	if got := effectiveBooruFilenameTemplate("", nil); got != "" {
		t.Fatalf("missing layout was used: %q", got)
	}
}
