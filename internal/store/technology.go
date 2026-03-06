package store

import (
	"database/sql"
	"fmt"

	"github.com/htopete/stackrigs/internal/model"
)

type TechnologyStore struct {
	db *sql.DB
}

func NewTechnologyStore(db *sql.DB) *TechnologyStore {
	return &TechnologyStore{db: db}
}

func (s *TechnologyStore) List() ([]model.Technology, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.slug, t.category, COUNT(bt.build_id) as build_count
		 FROM technologies t
		 LEFT JOIN build_technologies bt ON t.id = bt.technology_id
		 GROUP BY t.id
		 ORDER BY build_count DESC, t.name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing technologies: %w", err)
	}
	defer rows.Close()

	techs := make([]model.Technology, 0)
	for rows.Next() {
		var t model.Technology
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Category, &t.BuildCount); err != nil {
			return nil, fmt.Errorf("scanning technology: %w", err)
		}
		techs = append(techs, t)
	}
	return techs, nil
}

func (s *TechnologyStore) GetBySlug(slug string) (*model.Technology, error) {
	t := &model.Technology{}
	err := s.db.QueryRow(
		`SELECT t.id, t.name, t.slug, t.category, COUNT(bt.build_id) as build_count
		 FROM technologies t
		 LEFT JOIN build_technologies bt ON t.id = bt.technology_id
		 WHERE t.slug = ?
		 GROUP BY t.id`, slug,
	).Scan(&t.ID, &t.Name, &t.Slug, &t.Category, &t.BuildCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying technology by slug: %w", err)
	}
	return t, nil
}

func (s *TechnologyStore) GetBuildsBySlug(slug string, limit, offset int) ([]model.Build, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var total int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM builds b
		 JOIN build_technologies bt ON b.id = bt.build_id
		 JOIN technologies t ON bt.technology_id = t.id
		 WHERE t.slug = ?`, slug,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting builds for tech: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM builds b
		 JOIN build_technologies bt ON b.id = bt.build_id
		 JOIN technologies t ON bt.technology_id = t.id
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE t.slug = ?
		 ORDER BY b.updated_at DESC
		 LIMIT ? OFFSET ?`, slug, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("querying builds for tech: %w", err)
	}
	defer rows.Close()

	builds := make([]model.Build, 0)
	for rows.Next() {
		var b model.Build
		var builder model.Builder
		if err := rows.Scan(
			&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage, &b.CreatedAt, &b.UpdatedAt,
			&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning build: %w", err)
		}
		b.Builder = &builder
		builds = append(builds, b)
	}

	return builds, total, nil
}
