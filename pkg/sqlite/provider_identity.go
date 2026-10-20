package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stashapp/stash/pkg/booru"
	"github.com/stashapp/stash/pkg/models"
)

type ProviderIdentityStore struct{}

const identitySelect = `SELECT p.id,p.instance,p.source_id,p.source_name,p.engine,p.source_url,
 p.kind,p.form,p.target,p.label,p.context,p.version,p.updated_at,
 COALESCE(p.studio_id,p.performer_id,p.copyright_id,0) AS entity_id,
 COALESCE(s.name,f.name,c.name,'') AS entity_name
 FROM provider_identities p
 LEFT JOIN studios s ON s.id=p.studio_id
 LEFT JOIN performers f ON f.id=p.performer_id
 LEFT JOIN copyrights c ON c.id=p.copyright_id`

func identityNative(kind string) (table, column, aliases, aliasColumn string) {
	switch kind {
	case "artist":
		return "studios", "studio_id", "studio_aliases", "studio_id"
	case "character":
		return "performers", "performer_id", "performer_aliases", "performer_id"
	case "copyright":
		return "copyrights", "copyright_id", "copyright_aliases", "copyright_id"
	}
	return "", "", "", ""
}

func (s *ProviderIdentityStore) Find(ctx context.Context, id int64) (*models.ProviderIdentity, error) {
	var row models.ProviderIdentity
	err := dbWrapper.Get(ctx, &row, identitySelect+" WHERE p.id=?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &row, err
}

func (s *ProviderIdentityStore) Lookup(ctx context.Context, instance, kind, form, target string) (*models.ProviderIdentity, error) {
	var row models.ProviderIdentity
	err := dbWrapper.Get(ctx, &row, identitySelect+" WHERE p.instance=? AND p.kind=? AND p.form=? AND p.target=?", instance, kind, form, target)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &row, err
}

func (s *ProviderIdentityStore) Observe(ctx context.Context, input []models.IdentityObservation) ([]models.ProviderIdentity, error) {
	if len(input) > 2000 {
		return nil, models.ErrIdentityInvalid
	}
	var count int
	if err := dbWrapper.Get(ctx, &count, "SELECT COUNT(*) FROM provider_identities"); err != nil {
		return nil, err
	}
	rows := make([]models.ProviderIdentity, 0, len(input))
	for _, o := range input {
		source := booru.Source{ID: o.SourceID, Name: o.SourceName, URL: o.SourceURL, Engine: o.Engine}
		if source.Validate() != nil || source.Instance() != o.Instance || !models.IdentityKind(o.Kind) ||
			(o.Form != "tag" && o.Form != "account") || (o.Form == "account" && o.Kind != "artist") ||
			!models.IdentityText(o.Target, 1024) || !models.IdentityText(o.Label, 200) || len(o.Context) > 4096 || o.LabelRank < 0 || o.LabelRank > 2 {
			return nil, models.ErrIdentityInvalid
		}
		existing, err := s.Lookup(ctx, o.Instance, o.Kind, o.Form, o.Target)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			if count >= models.MaxProviderIdentities {
				return nil, models.ErrIdentityLimit
			}
			count++
		}
		_, err = dbWrapper.Exec(ctx, `INSERT INTO provider_identities
 (instance,source_id,source_name,engine,source_url,kind,form,target,label,label_rank,context,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(instance,kind,form,target) DO UPDATE SET
 source_id=excluded.source_id,source_name=excluded.source_name,
 label=CASE WHEN excluded.label_rank>=provider_identities.label_rank THEN excluded.label ELSE provider_identities.label END,
 label_rank=MAX(provider_identities.label_rank,excluded.label_rank),
 context=CASE WHEN excluded.context!='' THEN excluded.context ELSE provider_identities.context END,
 updated_at=excluded.updated_at`,
			o.Instance, o.SourceID, o.SourceName, o.Engine, o.SourceURL, o.Kind, o.Form, o.Target, o.Label, o.LabelRank, o.Context, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return nil, err
		}
		row, err := s.Lookup(ctx, o.Instance, o.Kind, o.Form, o.Target)
		if err != nil {
			return nil, err
		}
		rows = append(rows, *row)
	}
	return rows, nil
}

func identityLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	return "%" + strings.ReplaceAll(value, "_", "!_") + "%"
}

func (s *ProviderIdentityStore) List(ctx context.Context, filter models.IdentityFilter) ([]models.ProviderIdentity, error) {
	if filter.Limit < 1 || filter.Limit > 51 || filter.After < 0 || filter.ID < 0 || filter.EntityID < 0 ||
		(filter.Kind != "" && !models.IdentityKind(filter.Kind)) || len(filter.Query) > 200 ||
		(filter.State != "" && filter.State != "linked" && filter.State != "unresolved") {
		return nil, models.ErrIdentityInvalid
	}
	where := []string{"p.id>?"}
	args := []any{filter.After}
	if filter.ID > 0 {
		where = append(where, "p.id=?")
		args = append(args, filter.ID)
	}
	if filter.Kind != "" {
		where = append(where, "p.kind=?")
		args = append(args, filter.Kind)
	}
	if filter.EntityID > 0 {
		if !models.IdentityKind(filter.Kind) {
			return nil, models.ErrIdentityInvalid
		}
		_, column, _, _ := identityNative(filter.Kind)
		where = append(where, "p."+column+"=?")
		args = append(args, filter.EntityID)
	}
	if filter.State == "linked" {
		where = append(where, "COALESCE(p.studio_id,p.performer_id,p.copyright_id,0)>0")
	}
	if filter.State == "unresolved" {
		where = append(where, "COALESCE(p.studio_id,p.performer_id,p.copyright_id,0)=0")
	}
	if filter.Query != "" {
		where = append(where, "(p.target LIKE ? ESCAPE '!' OR p.label LIKE ? ESCAPE '!' OR COALESCE(s.name,f.name,c.name,'') LIKE ? ESCAPE '!')")
		q := identityLike(filter.Query)
		args = append(args, q, q, q)
	}
	args = append(args, filter.Limit)
	rows := []models.ProviderIdentity{}
	err := dbWrapper.Select(ctx, &rows, identitySelect+" WHERE "+strings.Join(where, " AND ")+" ORDER BY p.id LIMIT ?", args...)
	return rows, err
}

func (s *ProviderIdentityStore) Decide(ctx context.Context, action string, input models.IdentityDecision) (*models.ProviderIdentity, error) {
	if input.ID < 1 || input.Version < 1 || (action != "link" && action != "unlink" && action != "reject" && action != "restore") {
		return nil, models.ErrIdentityInvalid
	}
	row, err := s.Find(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.Version != input.Version {
		return nil, models.ErrIdentityConflict
	}
	table, column, _, _ := identityNative(row.Kind)
	if action != "unlink" {
		if input.EntityID < 1 {
			return nil, models.ErrIdentityInvalid
		}
		var found int
		if err := dbWrapper.Get(ctx, &found, "SELECT COUNT(*) FROM "+table+" WHERE id=?", input.EntityID); err != nil {
			return nil, err
		}
		if found != 1 {
			return nil, models.ErrIdentityConflict
		}
	}

	assignment := "version=version+1"
	args := []any{}
	if action == "link" {
		assignment += "," + column + "=?"
		args = append(args, input.EntityID)
	}
	if action == "unlink" {
		assignment += "," + column + "=NULL"
	}
	args = append(args, input.ID, input.Version)
	result, err := dbWrapper.Exec(ctx, "UPDATE provider_identities SET "+assignment+" WHERE id=? AND version=?", args...)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, models.ErrIdentityConflict
	}
	switch action {
	case "reject":
		_, err = dbWrapper.Exec(ctx, "INSERT OR IGNORE INTO provider_identity_rejections(identity_id,entity_id) VALUES(?,?)", input.ID, input.EntityID)
	case "restore", "link":
		_, err = dbWrapper.Exec(ctx, "DELETE FROM provider_identity_rejections WHERE identity_id=? AND entity_id=?", input.ID, input.EntityID)
	}
	if err != nil {
		return nil, err
	}
	return s.Find(ctx, input.ID)
}

func (s *ProviderIdentityStore) Candidates(ctx context.Context, kind, query string, identityID int64, includeRejected bool) ([]models.IdentityCandidate, error) {
	if !models.IdentityKind(kind) || len(query) > 200 || identityID < 0 {
		return nil, models.ErrIdentityInvalid
	}
	table, column, aliases, aliasColumn := identityNative(kind)
	disambiguation := "''"
	if kind == "character" {
		disambiguation = "COALESCE(n.disambiguation,'')"
	}
	// Candidate names are evidence for review, never automatic identity proof.
	sqlQuery := fmt.Sprintf(`SELECT n.id,COALESCE(n.name,'') AS name,%s AS disambiguation,
 EXISTS(SELECT 1 FROM provider_identity_rejections r WHERE r.identity_id=? AND r.entity_id=n.id) AS rejected
 FROM %s n WHERE (replace(COALESCE(n.name,''),'_',' ') LIKE ? ESCAPE '!'
 OR EXISTS(SELECT 1 FROM %s a WHERE a.%s=n.id AND replace(a.alias,'_',' ') LIKE ? ESCAPE '!')
 OR EXISTS(SELECT 1 FROM provider_identities p WHERE p.%s=n.id AND (p.target LIKE ? ESCAPE '!' OR p.label LIKE ? ESCAPE '!')))`,
		disambiguation, table, aliases, aliasColumn, column)
	q := identityLike(query)
	args := []any{identityID, q, q, q, q}
	if !includeRejected {
		sqlQuery += " AND NOT EXISTS(SELECT 1 FROM provider_identity_rejections r WHERE r.identity_id=? AND r.entity_id=n.id)"
		args = append(args, identityID)
	}
	sqlQuery += " ORDER BY n.name COLLATE NOCASE,n.id LIMIT 51"
	rows := []models.IdentityCandidate{}
	err := dbWrapper.Select(ctx, &rows, sqlQuery, args...)
	return rows, err
}

// Explicit unlink/reject decisions and bound identifiers are never discarded.
func (s *ProviderIdentityStore) DiscardUnresolved(ctx context.Context, kind string) (int64, error) {
	if !models.IdentityKind(kind) {
		return 0, models.ErrIdentityInvalid
	}
	result, err := dbWrapper.Exec(ctx, `DELETE FROM provider_identities WHERE kind=? AND version=1
      AND studio_id IS NULL AND performer_id IS NULL AND copyright_id IS NULL
      AND NOT EXISTS(SELECT 1 FROM provider_identity_rejections r WHERE r.identity_id=provider_identities.id)`, kind)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
