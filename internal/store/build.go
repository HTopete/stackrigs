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

	if params.Tech != "" {
		where = append(where, `b.id IN (SELECT bt.build_id FROM build_technologies bt JOIN technologies t ON bt.technology_id = t.id WHERE t.slug = ?)`)
		args = append(args, params.Tech)
	}
	if params.Status != "" {
		where = append(where, "b.status = ?")
		args = append(args, params.Status)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM builds b WHERE "+whereClause, args...).Scan(&total)
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
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image, b.created_at, b.updated_at,
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
	for rows.Next() {
		var b model.Build
		var builder model.Builder
		if err := rows.Scan(
			&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage, &b.CreatedAt, &b.UpdatedAt,
			&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning build row: %w", err)
		}
		b.Builder = &builder
		techs, err := s.getTechnologiesForBuild(b.ID)
		if err != nil {
			return nil, 0, err
		}
		b.Technologies = techs
		builds = append(builds, b)
	}

	return builds, total, nil
}

func (s *BuildStore) GetByID(id int64) (*model.Build, error) {
	b := &model.Build{}
	var builder model.Builder
	err := s.db.QueryRow(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM builds b
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE b.id = ?`, id,
	).Scan(&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage, &b.CreatedAt, &b.UpdatedAt,
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
		`INSERT INTO builds (builder_id, name, description, status, repo_url, live_url, cover_image, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.BuilderID, req.Name, req.Description, req.Status, req.RepoURL, req.LiveURL, req.CoverImage, now, now,
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
		`SELECT id, build_id, content, created_at FROM build_updates WHERE build_id = ? ORDER BY created_at DESC`, buildID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying updates for build: %w", err)
	}
	defer rows.Close()

	updates := make([]model.BuildUpdate, 0)
	for rows.Next() {
		var u model.BuildUpdate
		if err := rows.Scan(&u.ID, &u.BuildID, &u.Content, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning build update: %w", err)
		}
		updates = append(updates, u)
	}
	return updates, nil
}
