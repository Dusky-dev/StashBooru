package scene

import (
	"regexp"
	"testing"
)

func TestFilenameParserRating100Pattern(t *testing.T) {
	initParserFields()
	field, ok := validFields["rating100"]
	if !ok {
		t.Fatal("missing rating100 parser field")
	}
	pattern := regexp.MustCompile("^" + field.regex + "$")
	for _, value := range []string{"1", "42", "100"} {
		if !pattern.MatchString(value) {
			t.Errorf("rating100 pattern did not match %q", value)
		}
	}
	for _, value := range []string{"", "1000", "invalid"} {
		if pattern.MatchString(value) {
			t.Errorf("rating100 pattern unexpectedly matched %q", value)
		}
	}
	if !validateRating100(100) || validateRating100(101) {
		t.Fatal("rating100 validation must accept 100 and reject 101")
	}
}
