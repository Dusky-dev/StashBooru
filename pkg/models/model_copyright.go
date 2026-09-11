package models

import "time"

// Copyright is a first-class series/franchise metadata entity. It deliberately
// does not share Tag storage or require a synthetic root Tag.
type Copyright struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	SortName    string    `json:"sort_name"`
	Description string    `json:"description"`
	Favorite    bool      `json:"favorite"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Aliases     []string  `json:"aliases"`
}

type CopyrightCreateInput struct {
	Name        string   `json:"name"`
	SortName    *string  `json:"sort_name"`
	Description *string  `json:"description"`
	Favorite    *bool    `json:"favorite"`
	Aliases     []string `json:"aliases"`
	ParentIDs   []string `json:"parent_ids"`
	ChildIDs    []string `json:"child_ids"`
}

type CopyrightUpdateInput struct {
	ID          string    `json:"id"`
	Name        *string   `json:"name"`
	SortName    *string   `json:"sort_name"`
	Description *string   `json:"description"`
	Favorite    *bool     `json:"favorite"`
	Aliases     *[]string `json:"aliases"`
	ParentIDs   *[]string `json:"parent_ids"`
	ChildIDs    *[]string `json:"child_ids"`
}

type CopyrightDestroyInput struct {
	ID string `json:"id"`
}

type FindCopyrightsResultType struct {
	Count      int          `json:"count"`
	Copyrights []*Copyright `json:"copyrights"`
}
