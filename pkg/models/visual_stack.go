package models

import "context"

// Visual stacks are catalogue relationships, independent of file storage.
type VisualStack struct {
	ID             int    `db:"id"`
	Title          string `db:"title"`
	Version        int    `db:"version"`
	MemberCount    int    `db:"member_count"`
	Representative string
	Members        []*VisualStackMember
}

type VisualStackMember struct {
	ID             string
	Media          MediaReference
	Position       int    `db:"position"`
	Label          string `db:"label"`
	Representative bool   `db:"representative"`
}

type VisualStackMemberInput struct {
	Media MediaReference
	Label string
}
type VisualStackCreateInput struct {
	Title          string
	Members        []*VisualStackMemberInput
	Representative MediaReference
}
type VisualStackUpdateInput struct {
	ID             int
	Version        int
	Title          string
	Members        []*VisualStackMemberInput
	Representative MediaReference
}
type VisualStackSplitInput struct {
	ID             int
	Version        int
	Members        []*MediaReference
	Title          string
	Representative MediaReference
}
type VisualStackVersionInput struct {
	ID      int
	Version int
}
type VisualStackMergeInput struct {
	Stacks         []*VisualStackVersionInput
	Title          string
	Representative MediaReference
}
type VisualStackProposal struct {
	Members  []*MediaReference
	Evidence string
	Score    *float64
}
type VisualStackReaderWriter interface {
	Find(context.Context, int) (*VisualStack, error)
	FindByMedia(context.Context, MediaReference) (*VisualStack, error)
	Create(context.Context, VisualStackCreateInput) (*VisualStack, error)
	Update(context.Context, VisualStackUpdateInput) (*VisualStack, error)
	Split(context.Context, VisualStackSplitInput) (*VisualStack, error)
	Merge(context.Context, VisualStackMergeInput) (*VisualStack, error)
	Destroy(context.Context, VisualStackVersionInput) error
	Propose(context.Context, []*MediaReference) ([]*VisualStackProposal, error)
}
