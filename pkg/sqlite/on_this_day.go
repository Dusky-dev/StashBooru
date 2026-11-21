package sqlite

import (
	"context"
	"fmt"

	"github.com/stashapp/stash/pkg/models"
)

type OnThisDayStore struct{}

// An item's day is its Date when that has day precision (0), else the day
// it was added. created_at is stored with its offset; date() reads it as UTC.
func (s *OnThisDayStore) OnThisDay(ctx context.Context, kind string, month, day, year, limit int) ([]models.OnThisDayItem, error) {
	table := map[string]string{"image": "images", "scene": "scenes"}[kind]
	if table == "" || month < 1 || month > 12 || day < 1 || day > 31 || limit < 1 || limit > 200 {
		return nil, models.ErrFollowingInvalid
	}
	rows := []models.OnThisDayItem{}
	err := dbWrapper.Select(ctx, &rows, fmt.Sprintf(`SELECT id, day FROM (
 SELECT id, CASE WHEN date IS NOT NULL AND date <> '' AND COALESCE(date_precision,0)=0 THEN date(date) ELSE date(created_at) END AS day FROM %s
) WHERE day IS NOT NULL AND strftime('%%m-%%d', day)=? AND CAST(strftime('%%Y', day) AS INTEGER)<>?
 ORDER BY day DESC, id DESC LIMIT ?`, table), fmt.Sprintf("%02d-%02d", month, day), year, limit)
	return rows, err
}
