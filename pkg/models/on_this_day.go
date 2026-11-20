package models

import "context"

// OnThisDayItem is an Image or Video from an earlier year on the same
// calendar day, by its Date (day precision) or else the date it was added.
type OnThisDayItem struct {
	ID   int    `json:"id,string" db:"id"`
	Date string `json:"date" db:"day"`
}

type OnThisDayReader interface {
	// OnThisDay lists items of kind "image" or "scene" whose date falls on
	// month/day of a year other than year, newest first.
	OnThisDay(ctx context.Context, kind string, month, day, year, limit int) ([]OnThisDayItem, error)
}
