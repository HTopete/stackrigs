package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/htopete/stackrigs/internal/model"
)

type SearchStore struct {
	db *sql.DB
}

func NewSearchStore(db *sql.DB) *SearchStore {
	return &SearchStore{db: db}
}

// RebuildIndex drops all rows from the FTS5 search_index and repopulates it
// from builders, builds, and technologies tables.
func (s *SearchStore) RebuildIndex() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning rebuild transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing index
	if _, err := tx.Exec("DELETE FROM search_index"); err != nil {
		return fmt.Errorf("clearing search index: %w", err)
	}

	// Index builders
	rows, err := tx.Query("SELECT id, handle, display_name, bio FROM builders")
	if err != nil {
		return fmt.Errorf("querying builders for index: %w", err)
	}
	for rows.Next() {
		var id int64
		var handle, displayName, bio string
		if err := rows.Scan(&id, &handle, &displayName, &bio); err != nil {
			rows.Close()
			return fmt.Errorf("scanning builder for index: %w", err)
		}
		if _, err := tx.Exec(
			"INSERT INTO search_index (entity_type, entity_id, title, body) VALUES (?, ?, ?, ?)",
			"builder", fmt.Sprintf("%d", id), displayName, handle+" "+bio,
		); err != nil {
			rows.Close()
			return fmt.Errorf("inserting builder into index: %w", err)
		}
	}
	rows.Close()

	// Index builds
	rows, err = tx.Query("SELECT id, name, description FROM builds")
	if err != nil {
		return fmt.Errorf("querying builds for index: %w", err)
	}
	for rows.Next() {
		var id int64
		var name, description string
		if err := rows.Scan(&id, &name, &description); err != nil {
			rows.Close()
			return fmt.Errorf("scanning build for index: %w", err)
		}
		if _, err := tx.Exec(
			"INSERT INTO search_index (entity_type, entity_id, title, body) VALUES (?, ?, ?, ?)",
			"build", fmt.Sprintf("%d", id), name, description,
		); err != nil {
			rows.Close()
			return fmt.Errorf("inserting build into index: %w", err)
		}
	}
	rows.Close()

	// Index technologies
	rows, err = tx.Query("SELECT id, name, slug, category FROM technologies")
	if err != nil {
		return fmt.Errorf("querying technologies for index: %w", err)
	}
	for rows.Next() {
		var id int64
		var name, slug, category string
		if err := rows.Scan(&id, &name, &slug, &category); err != nil {
			rows.Close()
			return fmt.Errorf("scanning technology for index: %w", err)
		}
		if _, err := tx.Exec(
			"INSERT INTO search_index (entity_type, entity_id, title, body) VALUES (?, ?, ?, ?)",
			"technology", fmt.Sprintf("%d", id), name, slug+" "+category,
		); err != nil {
			rows.Close()
			return fmt.Errorf("inserting technology into index: %w", err)
		}
	}
	rows.Close()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing rebuild transaction: %w", err)
	}

	return nil
}

// InsertIndex adds a single entity to the FTS5 search index.
func (s *SearchStore) InsertIndex(entityType string, entityID int64, title, body string) error {
	_, err := s.db.Exec(
		"INSERT INTO search_index (entity_type, entity_id, title, body) VALUES (?, ?, ?, ?)",
		entityType, fmt.Sprintf("%d", entityID), title, body,
	)
	if err != nil {
		return fmt.Errorf("inserting into search index: %w", err)
	}
	return nil
}

// DeleteIndex removes an entity from the FTS5 search index.
func (s *SearchStore) DeleteIndex(entityType string, entityID int64) error {
	_, err := s.db.Exec(
		"DELETE FROM search_index WHERE entity_type = ? AND entity_id = ?",
		entityType, fmt.Sprintf("%d", entityID),
	)
	if err != nil {
		return fmt.Errorf("deleting from search index: %w", err)
	}
	return nil
}

// UpdateIndex replaces an entity in the FTS5 search index.
func (s *SearchStore) UpdateIndex(entityType string, entityID int64, title, body string) error {
	if err := s.DeleteIndex(entityType, entityID); err != nil {
		return err
	}
	return s.InsertIndex(entityType, entityID, title, body)
}

// Search performs a full-text search using FTS5 with bm25 ranking,
// highlight, and prefix matching. Falls back to LIKE queries if FTS5 returns
// no results (e.g. for very short queries).
func (s *SearchStore) Search(query string) (*model.SearchResult, error) {
	if query == "" {
		return &model.SearchResult{
			Builders:     []model.Builder{},
			Builds:       []model.Build{},
			Technologies: []model.Technology{},
		}, nil
	}

	// Prepare FTS5 query: add prefix matching with *
	ftsQuery := sanitizeFTSQuery(query)

	builders, err := s.searchBuildersFTS(ftsQuery, query)
	if err != nil {
		return nil, err
	}

	builds, err := s.searchBuildsFTS(ftsQuery, query)
	if err != nil {
		return nil, err
	}

	technologies, err := s.searchTechnologiesFTS(ftsQuery, query)
	if err != nil {
		return nil, err
	}

	return &model.SearchResult{
		Builders:     builders,
		Builds:       builds,
		Technologies: technologies,
	}, nil
}

// sanitizeFTSQuery escapes special FTS5 characters and appends * for prefix matching.
func sanitizeFTSQuery(q string) string {
	// Remove FTS5 special characters to prevent injection
	replacer := strings.NewReplacer(
		"\"", "",
		"'", "",
		"*", "",
		"(", "",
		")", "",
		":", "",
		"^", "",
		"-", " ",
		".", " ",
	)
	sanitized := strings.TrimSpace(replacer.Replace(q))
	if sanitized == "" {
		return ""
	}

	// Split into words and add prefix matching to the last word
	words := strings.Fields(sanitized)
	if len(words) == 0 {
		return ""
	}

	// Add * to each word for prefix matching
	for i := range words {
		words[i] = "\"" + words[i] + "\"*"
	}

	return strings.Join(words, " ")
}

func (s *SearchStore) searchBuildersFTS(ftsQuery, rawQuery string) ([]model.Builder, error) {
	results := make([]model.Builder, 0)

	if ftsQuery == "" {
		return results, nil
	}

	rows, err := s.db.Query(
		`SELECT b.id, b.handle, b.display_name, b.bio, b.avatar_url, b.website, b.github_url, b.twitter_url, b.created_at, b.updated_at
		 FROM search_index si
		 JOIN builders b ON CAST(si.entity_id AS INTEGER) = b.id
		 WHERE search_index MATCH ? AND si.entity_type = 'builder'
		 ORDER BY bm25(search_index)
		 LIMIT 10`,
		ftsQuery,
	)
	if err != nil {
		// Fallback to LIKE on FTS5 query error
		return s.searchBuildersLike(rawQuery)
	}
	defer rows.Close()

	for rows.Next() {
		var b model.Builder
		if err := rows.Scan(&b.ID, &b.Handle, &b.DisplayName, &b.Bio, &b.AvatarURL, &b.Website, &b.GithubURL, &b.TwitterURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning builder: %w", err)
		}
		results = append(results, b)
	}

	// Fallback to LIKE if FTS5 returned nothing
	if len(results) == 0 {
		return s.searchBuildersLike(rawQuery)
	}

	return results, nil
}

func (s *SearchStore) searchBuildersLike(query string) ([]model.Builder, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT id, handle, display_name, bio, avatar_url, website, github_url, twitter_url, created_at, updated_at
		 FROM builders
		 WHERE handle LIKE ? OR display_name LIKE ? OR bio LIKE ?
		 LIMIT 10`,
		pattern, pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching builders (like): %w", err)
	}
	defer rows.Close()

	results := make([]model.Builder, 0)
	for rows.Next() {
		var b model.Builder
		if err := rows.Scan(&b.ID, &b.Handle, &b.DisplayName, &b.Bio, &b.AvatarURL, &b.Website, &b.GithubURL, &b.TwitterURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning builder: %w", err)
		}
		results = append(results, b)
	}
	return results, nil
}

func (s *SearchStore) searchBuildsFTS(ftsQuery, rawQuery string) ([]model.Build, error) {
	results := make([]model.Build, 0)

	if ftsQuery == "" {
		return results, nil
	}

	rows, err := s.db.Query(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM search_index si
		 JOIN builds b ON CAST(si.entity_id AS INTEGER) = b.id
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE search_index MATCH ? AND si.entity_type = 'build'
		 ORDER BY bm25(search_index)
		 LIMIT 20`,
		ftsQuery,
	)
	if err != nil {
		return s.searchBuildsLike(rawQuery)
	}
	defer rows.Close()

	for rows.Next() {
		var b model.Build
		var builder model.Builder
		if err := rows.Scan(
			&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage, &b.CreatedAt, &b.UpdatedAt,
			&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scanning build: %w", err)
		}
		b.Builder = &builder
		results = append(results, b)
	}

	if len(results) == 0 {
		return s.searchBuildsLike(rawQuery)
	}

	return results, nil
}

func (s *SearchStore) searchBuildsLike(query string) ([]model.Build, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT b.id, b.builder_id, b.name, b.description, b.status, b.repo_url, b.live_url, b.cover_image, b.created_at, b.updated_at,
		        bu.id, bu.handle, bu.display_name, bu.avatar_url
		 FROM builds b
		 JOIN builders bu ON b.builder_id = bu.id
		 WHERE b.name LIKE ? OR b.description LIKE ?
		 ORDER BY b.updated_at DESC
		 LIMIT 20`,
		pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching builds (like): %w", err)
	}
	defer rows.Close()

	results := make([]model.Build, 0)
	for rows.Next() {
		var b model.Build
		var builder model.Builder
		if err := rows.Scan(
			&b.ID, &b.BuilderID, &b.Name, &b.Description, &b.Status, &b.RepoURL, &b.LiveURL, &b.CoverImage, &b.CreatedAt, &b.UpdatedAt,
			&builder.ID, &builder.Handle, &builder.DisplayName, &builder.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scanning build: %w", err)
		}
		b.Builder = &builder
		results = append(results, b)
	}
	return results, nil
}

func (s *SearchStore) searchTechnologiesFTS(ftsQuery, rawQuery string) ([]model.Technology, error) {
	results := make([]model.Technology, 0)

	if ftsQuery == "" {
		return results, nil
	}

	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.slug, t.category, COUNT(bt.build_id) as build_count
		 FROM search_index si
		 JOIN technologies t ON CAST(si.entity_id AS INTEGER) = t.id
		 LEFT JOIN build_technologies bt ON t.id = bt.technology_id
		 WHERE search_index MATCH ? AND si.entity_type = 'technology'
		 GROUP BY t.id
		 ORDER BY bm25(search_index)
		 LIMIT 10`,
		ftsQuery,
	)
	if err != nil {
		return s.searchTechnologiesLike(rawQuery)
	}
	defer rows.Close()

	for rows.Next() {
		var t model.Technology
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Category, &t.BuildCount); err != nil {
			return nil, fmt.Errorf("scanning technology: %w", err)
		}
		results = append(results, t)
	}

	if len(results) == 0 {
		return s.searchTechnologiesLike(rawQuery)
	}

	return results, nil
}

func (s *SearchStore) searchTechnologiesLike(query string) ([]model.Technology, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.slug, t.category, COUNT(bt.build_id) as build_count
		 FROM technologies t
		 LEFT JOIN build_technologies bt ON t.id = bt.technology_id
		 WHERE t.name LIKE ? OR t.slug LIKE ?
		 GROUP BY t.id
		 ORDER BY build_count DESC
		 LIMIT 10`,
		pattern, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("searching technologies (like): %w", err)
	}
	defer rows.Close()

	results := make([]model.Technology, 0)
	for rows.Next() {
		var t model.Technology
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.Category, &t.BuildCount); err != nil {
			return nil, fmt.Errorf("scanning technology: %w", err)
		}
		results = append(results, t)
	}
	return results, nil
}
