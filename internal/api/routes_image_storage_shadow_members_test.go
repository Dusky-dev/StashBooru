package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuidedShadowMembersRejectUnknownAndDuplicateInputs(t *testing.T) {
	valid := "candidateID=stack-51-delta-0123456789abcdef01234567&stackID=51"
	got, err := storageShadowParseMembersRequest(httptest.NewRequest("GET", "/image/storage-shadow-members?"+valid, nil))
	if err != nil || got.StackID != 51 {
		t.Fatalf("refused valid guided selection: %+v %v", got, err)
	}
	for _, query := range []string{
		"",
		"stackID=51",
		valid + "&archive=/tmp/bad.pack",
		valid + "&candidateID=stack-51-delta-0123456789abcdef01234567",
		"candidateID=../../secret&stackID=51",
		"candidateID=stack-51-delta-0123456789abcdef01234567&stackID=-1",
	} {
		if _, err := storageShadowParseMembersRequest(httptest.NewRequest("GET", "/image/storage-shadow-members?"+query, nil)); err == nil {
			t.Fatalf("accepted unsafe guided selection: %s", query)
		}
	}
}

func TestGuidedShadowMembersRequireProofWithoutExposingPaths(t *testing.T) {
	group := estimateSourceGroup{
		ID: "stack:51",
		Members: []estimateSourceMember{
			{ID: "image:511", Path: "/sensitive/private/first.png"},
			{ID: "image:512", Path: "/sensitive/private/second.png"},
		},
	}
	proof := storageCandidateProof{
		Group: "stack:51", Fidelity: "byte-exact", Method: "delta",
		Verified: true, BackupVerified: true, SourcesRetained: true, NativePackingCandidate: true,
		SourceFiles: 2, SourceSHA256: map[string]string{
			"image:511": strings.Repeat("a", 64),
			"image:512": strings.Repeat("b", 64),
		},
	}
	got, err := storageShadowMemberOptions(group, proof)
	if err != nil || len(got) != 2 || got[0].ImageID != 511 || got[1].ImageID != 512 {
		t.Fatalf("unexpected safe member options: %+v %v", got, err)
	}
	for _, alter := range []func(*storageCandidateProof){
		func(p *storageCandidateProof) { p.OriginalEviction = true },
		func(p *storageCandidateProof) { p.NativeServing = true },
		func(p *storageCandidateProof) { p.Fidelity = "pixel-exact" },
		func(p *storageCandidateProof) { p.Group = "stack:52" },
		func(p *storageCandidateProof) { p.SourcesRetained = false },
	} {
		p := proof
		alter(&p)
		if _, err := storageShadowMemberOptions(group, p); err == nil {
			t.Fatal("accepted altered or dangerous candidate proof")
		}
	}
	changed := proof
	changed.SourceSHA256 = map[string]string{"image:511": strings.Repeat("a", 64)}
	if _, err := storageShadowMemberOptions(group, changed); err == nil {
		t.Fatal("accepted incomplete member checksum proof")
	}
	bad := group
	bad.Members = []estimateSourceMember{{ID: "image:511"}, {ID: "image:511"}}
	if _, err := storageShadowMemberOptions(bad, proof); err == nil {
		t.Fatal("accepted duplicate image identities")
	}
}
