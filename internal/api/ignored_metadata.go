package api

// Ignored Tags, Characters, Artists and Copyrights are hidden from lists and
// selectors unless a list asks for them with the Ignored filter, and are never
// added again by downloads or tagging. Media that already has one keeps it.

// listedIgnored is the Ignored filter a list query uses: an explicit choice,
// otherwise only records that are not ignored.
func listedIgnored(ignored *bool) *bool {
	if ignored != nil {
		return ignored
	}
	hidden := false
	return &hidden
}
