package model

import "time"

type Builder struct {
	ID             int64     `json:"id"`
	Handle         string    `json:"handle"`
	DisplayName    string    `json:"display_name"`
	Bio            string    `json:"bio,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	Website        string    `json:"website,omitempty"`
	GithubURL      string    `json:"github_url,omitempty"`
	TwitterURL     string    `json:"twitter_url,omitempty"`
	InvitationCode string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateBuilderRequest struct {
	Handle         string `json:"handle"`
	DisplayName    string `json:"display_name"`
	Bio            string `json:"bio"`
	AvatarURL      string `json:"avatar_url"`
	Website        string `json:"website"`
	GithubURL      string `json:"github_url"`
	TwitterURL     string `json:"twitter_url"`
	InvitationCode string `json:"invitation_code"`
}

type Build struct {
	ID           int64        `json:"id"`
	BuilderID    int64        `json:"builder_id"`
	Builder      *Builder     `json:"builder,omitempty"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Status       string       `json:"status"`
	RepoURL      string       `json:"repo_url,omitempty"`
	LiveURL      string       `json:"live_url,omitempty"`
	CoverImage   string       `json:"cover_image,omitempty"`
	WhatWorks    string       `json:"what_works,omitempty"`
	WhatBroke    string       `json:"what_broke,omitempty"`
	WhatIdChange string       `json:"what_id_change,omitempty"`
	Technologies []Technology `json:"technologies,omitempty"`
	Updates      []BuildUpdate `json:"updates,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type CreateBuildRequest struct {
	BuilderID    int64    `json:"builder_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	RepoURL      string   `json:"repo_url"`
	LiveURL      string   `json:"live_url"`
	CoverImage   string   `json:"cover_image"`
	WhatWorks    string   `json:"what_works"`
	WhatBroke    string   `json:"what_broke"`
	WhatIdChange string   `json:"what_id_change"`
	Technologies []string `json:"technologies"`
}

type UpdateBuildRequest struct {
	Name         *string  `json:"name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Status       *string  `json:"status,omitempty"`
	RepoURL      *string  `json:"repo_url,omitempty"`
	LiveURL      *string  `json:"live_url,omitempty"`
	CoverImage   *string  `json:"cover_image,omitempty"`
	WhatWorks    *string  `json:"what_works,omitempty"`
	WhatBroke    *string  `json:"what_broke,omitempty"`
	WhatIdChange *string  `json:"what_id_change,omitempty"`
	Technologies []string `json:"technologies,omitempty"`
}

type BuildUpdate struct {
	ID        int64     `json:"id"`
	BuildID   int64     `json:"build_id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Technology struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Category   string `json:"category,omitempty"`
	BuildCount int    `json:"build_count,omitempty"`
}

type BuildListParams struct {
	Tech    string
	Status  string
	Sort    string
	Builder string
	Limit   int
	Offset  int
}

type SearchResult struct {
	Builders     []Builder    `json:"builders"`
	Builds       []Build      `json:"builds"`
	Technologies []Technology `json:"technologies"`
}

type InfraMetrics struct {
	Uptime       string  `json:"uptime"`
	MemTotal     string  `json:"mem_total"`
	MemAvailable string  `json:"mem_available"`
	MemUsedPct   float64 `json:"mem_used_pct"`
	LoadAvg      string  `json:"load_avg"`
	RequestsMin  int64   `json:"requests_min"`
	Timestamp    string  `json:"timestamp"`
}

type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Version  string `json:"version"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	HasMore    bool        `json:"has_more"`
}
