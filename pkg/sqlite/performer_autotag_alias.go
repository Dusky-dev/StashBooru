package sqlite

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/stashapp/stash/pkg/models"
)

// QueryAliasesForAutoTag returns performer candidates whose aliases start with
// one of the path words. Exact path matching still happens in pkg/match, so
// this query is intentionally broad and only narrows the candidate set.
func (qb *PerformerStore) QueryAliasesForAutoTag(ctx context.Context, words []string) ([]*models.Performer, error) {
	if len(words) == 0 {
		return nil, nil
	}

	table := qb.table()
	aliasTable := performersAliasesJoinTable
	q := qb.selectDataset().Distinct().InnerJoin(
		aliasTable,
		goqu.On(aliasTable.Col(performerIDColumn).Eq(table.Col(idColumn))),
	)

	var whereClauses []exp.Expression
	for _, word := range words {
		whereClauses = append(whereClauses, aliasTable.Col(performerAliasColumn).Like(word+"%"))
	}

	q = q.Where(
		goqu.Or(whereClauses...),
		table.Col("ignore_auto_tag").Eq(0),
	)

	ret, err := qb.getMany(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("getting performer aliases for autotag: %w", err)
	}

	return ret, nil
}
