package sqlite

import "context"

func (s *CopyrightStore) TagStructuralRole(ctx context.Context, id int) (string, error) {
	return TagStructuralRole(ctx, id)
}

func (s *CopyrightStore) SetTagStructuralRole(ctx context.Context, id int, role string) error {
	return SetTagStructuralRole(ctx, id, role)
}
