package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/htopete/stackrigs/internal/model"
)

type BuildStore struct {
	db *sql.DB
}

func NewBuildStore(db *sql.DB) *BuildStore {
	return &BuildStore{db: db}
}

func (s *BuildStore) List(params model.BuildListParams) ([]model.Build, int, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if len(params.Techs) == 1 {
		where = append(where, `b.id IN (SELECT bt.build_id FROM build_technologies bt JOIN technologies t ON bt.technology_id = t.id WHERE t.slug = ?)`)
		args = append(args, params.Techs[0])
	} else if len(params.Techs) > 1 {
		placeholders := strings.Repeat("?,", len(params.Techs))
		placeholders = placeholders[:len(placeholders)-1]
		where = append(where, `b.id IN (SELECT bt.build_id FROM build_technologies bt JOIN technologies t ON bt.technology_id = t.id WHERE t.slug IN (`+placeholders+`))`)
		for _, slug := range params.Techs {
			args = append(args, slug)
		}
	}
	if params.Status != "" {
		where = append(where, "b.status = ?")
		args = append(args, params.Status)
	}
	if params.Builder != "" {
		where = append(where, "bu.handle = ?")
		args = append(args, params.Builder)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM builds b"
	if params.Builder != "" {
		countQuery += " JOIN builders bu ON b.builder_id = bu.id"
	}
	countQuery += " WHERE " + whereClause
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting builds: %w", err)
	}

	orderBy := "b.updated_at DESC"
	switch params.Sort {
	case "created":
		orderBy = "b.created_at DESC"
	case "name":
		orderBy = "b.name ASC"
	case "updated":
		orderBy = "b.updated_at DESC"
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	query := fmt.Sprintf(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image,
		        b.what_works, b.what_broke, b.what_id_change, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM builds b
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE %s ORDER BY %s LIMIT ? OFFSET ?`, whereClause, orderBy)

	args = append(args, params.Limit, params.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing builds: %w", err)
	}
	defer rows.Close()

	builds := make([]model.Build, 0)
	buildIndex := make(map[int64]int) // id → slice index
	for rows.Next() {
		var b model.Build
		var builder model.Builder
		if err := rows.Scan(
			&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage,
			&b.WhatWorks, &b.WhatBroke, &b.WhatIdChange, &b.CreatedAt, &b.UpdatedAt,
			&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning build row: %w", err)
		}
		b.Builder = &builder
		buildIndex[b.ID] = len(builds)
		builds = append(builds, b)
	}

	// Batch-load all technologies in one query (eliminates N+1).
	if len(builds) > 0 {
		ids := make([]interface{}, len(builds))
		placeholders := make([]string, len(builds))
		for i, b := range builds {
			ids[i] = b.ID
			placeholders[i] = "?"
		}
		techQuery := fmt.Sprintf(
			`SELECT bt.build_id, t.id, t.name, t.slug, t.category
			 FROM technologies t
			 JOIN build_technologies bt ON t.id = bt.technology_id
			 WHERE bt.build_id IN (%s)`, strings.Join(placeholders, ","),
		)
		techRows, err := s.db.Query(techQuery, ids...)
		if err != nil {
			return nil, 0, fmt.Errorf("batch loading technologies: %w", err)
		}
		defer techRows.Close()
		for techRows.Next() {
			var buildID int64
			var t model.Technology
			if err := techRows.Scan(&buildID, &t.ID, &t.Name, &t.Slug, &t.Category); err != nil {
				return nil, 0, fmt.Errorf("scanning technology row: %w", err)
			}
			if idx, ok := buildIndex[buildID]; ok {
				builds[idx].Technologies = append(builds[idx].Technologies, t)
			}
		}
	}

	return builds, total, nil
}

func (s *BuildStore) GetByID(id int64) (*model.Build, error) {
	b := &model.Build{}
	var builder model.Builder
	err := s.db.QueryRow(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image,
		        b.what_works, b.what_broke, b.what_id_change, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM builds b
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE b.id = ?`, id,
	).Scan(&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage,
		&b.WhatWorks, &b.WhatBroke, &b.WhatIdChange, &b.CreatedAt, &b.UpdatedAt,
		&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying build by id: %w", err)
	}
	b.Builder = &builder

	techs, err := s.getTechnologiesForBuild(b.ID)
	if err != nil {
		return nil, err
	}
	b.Technologies = techs

	updates, err := s.getUpdatesForBuild(b.ID)
	if err != nil {
		return nil, err
	}
	b.Updates = updates

	return b, nil
}

func (s *BuildStore) Create(req model.CreateBuildRequest) (*model.Build, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO builds (builder_id, name, description, status, repo_url, live_url, cover_image, what_works, what_broke, what_id_change, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.BuilderID, req.Name, req.Description, req.Status, req.RepoURL, req.LiveURL, req.CoverImage, req.WhatWorks, req.WhatBroke, req.WhatIdChange, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting build: %w", err)
	}

	buildID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	if err := s.syncTechnologies(buildID, req.Technologies); err != nil {
		return nil, err
	}

	return s.GetByID(buildID)
}

func (s *BuildStore) Update(id int64, req model.UpdateBuildRequest) (*model.Build, error) {
	sets := []string{}
	args := []interface{}{}

	if req.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		sets = append(sets, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *req.Status)
	}
	if req.RepoURL != nil {
		sets = append(sets, "repo_url = ?")
		args = append(args, *req.RepoURL)
	}
	if req.LiveURL != nil {
		sets = append(sets, "live_url = ?")
		args = append(args, *req.LiveURL)
	}
	if req.CoverImage != nil {
		sets = append(sets, "cover_image = ?")
		args = append(args, *req.CoverImage)
	}
	if req.WhatWorks != nil {
		sets = append(sets, "what_works = ?")
		args = append(args, *req.WhatWorks)
	}
	if req.WhatBroke != nil {
		sets = append(sets, "what_broke = ?")
		args = append(args, *req.WhatBroke)
	}
	if req.WhatIdChange != nil {
		sets = append(sets, "what_id_change = ?")
		args = append(args, *req.WhatIdChange)
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = ?")
		args = append(args, time.Now().UTC())
		args = append(args, id)

		query := fmt.Sprintf("UPDATE builds SET %s WHERE id = ?", strings.Join(sets, ", "))
		if _, err := s.db.Exec(query, args...); err != nil {
			return nil, fmt.Errorf("updating build: %w", err)
		}
	}

	if req.Technologies != nil {
		if err := s.syncTechnologies(id, req.Technologies); err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// UpdateCoverImage sets the cover_image field for a build directly.
func (s *BuildStore) UpdateCoverImage(buildID int64, url string) error {
	_, err := s.db.Exec(
		"UPDATE builds SET cover_image = ?, updated_at = ? WHERE id = ?",
		url, time.Now().UTC(), buildID,
	)
	if err != nil {
		return fmt.Errorf("updating cover image: %w", err)
	}
	return nil
}

// CountByBuilder returns the total number of builds for a given builder.
func (s *BuildStore) CountByBuilder(builderID int64) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM builds WHERE builder_id = ?", builderID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting builds for builder: %w", err)
	}
	return count, nil
}

func (s *BuildStore) GetOwnerID(buildID int64) (int64, error) {
	var ownerID int64
	err := s.db.QueryRow("SELECT builder_id FROM builds WHERE id = ?", buildID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("build not found")
	}
	if err != nil {
		return 0, fmt.Errorf("querying build owner: %w", err)
	}
	return ownerID, nil
}

func (s *BuildStore) syncTechnologies(buildID int64, techSlugs []string) error {
	if _, err := s.db.Exec("DELETE FROM build_technologies WHERE build_id = ?", buildID); err != nil {
		return fmt.Errorf("clearing build technologies: %w", err)
	}

	for _, slug := range techSlugs {
		slug = strings.TrimSpace(strings.ToLower(slug))
		if slug == "" {
			continue
		}

		// Upsert technology
		_, err := s.db.Exec(
			`INSERT INTO technologies (name, slug) VALUES (?, ?) ON CONFLICT(slug) DO NOTHING`,
			slug, slug,
		)
		if err != nil {
			return fmt.Errorf("upserting technology %q: %w", slug, err)
		}

		var techID int64
		if err := s.db.QueryRow("SELECT id FROM technologies WHERE slug = ?", slug).Scan(&techID); err != nil {
			return fmt.Errorf("getting technology id for %q: %w", slug, err)
		}

		if _, err := s.db.Exec("INSERT INTO build_technologies (build_id, technology_id) VALUES (?, ?)", buildID, techID); err != nil {
			return fmt.Errorf("linking build to technology: %w", err)
		}
	}

	return nil
}

func (s *BuildStore) getTechnologiesForBuild(buildID int64) ([]model.Technology, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.slug, t.category
		 FROM technologies t
		 JOIN build_technologies bt ON t.id = bt.technology_id
		 WHERE bt.build_id = ?`, buildID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying technologies for build: %w", err)
	}
	defer rows.Close()

	techs := make([]model.Technology, 0)
	for rows.Next() {
		var t model.Technology
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Category); err != nil {
			return nil, fmt.Errorf("scanning technology: %w", err)
		}
		techs = append(techs, t)
	}
	return techs, nil
}

func (s *BuildStore) getUpdatesForBuild(buildID int64) ([]model.BuildUpdate, error) {
	rows, err := s.db.Query(
		`SELECT id, build_id, type, title, content, created_at FROM build_updates WHERE build_id = ? ORDER BY created_at DESC`, buildID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying updates for build: %w", err)
	}
	defer rows.Close()

	updates := make([]model.BuildUpdate, 0)
	for rows.Next() {
		var u model.BuildUpdate
		if err := rows.Scan(&u.ID, &u.BuildID, &u.Type, &u.Title, &u.Content, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning build update: %w", err)
		}
		updates = append(updates, u)
	}
	return updates, nil
}

func (s *BuildStore) CreateUpdate(buildID int64, updateType, title, content string) (*model.BuildUpdate, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO build_updates (build_id, type, title, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		buildID, updateType, title, content, now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting build update: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	// Also touch the build's updated_at
	_, _ = s.db.Exec(`UPDATE builds SET updated_at = ? WHERE id = ?`, now, buildID)

	return &model.BuildUpdate{
		ID:        id,
		BuildID:   buildID,
		Type:      updateType,
		Title:     title,
		Content:   content,
		CreatedAt: now,
	}, nil
}

func (s *BuildStore) DeleteUpdate(updateID, buildID int64) error {
	result, err := s.db.Exec(
		`DELETE FROM build_updates WHERE id = ? AND build_id = ?`,
		updateID, buildID,
	)
	if err != nil {
		return fmt.Errorf("deleting build update: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("update not found")
	}
	return nil
}

func (s *BuildStore) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec(`DELETE FROM build_technologies WHERE build_id = ?`, id)
	_, _ = tx.Exec(`DELETE FROM build_updates WHERE build_id = ?`, id)

	result, err := tx.Exec(`DELETE FROM builds WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting build: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("build not found")
	}

	return tx.Commit()
}
