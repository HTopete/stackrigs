package store

import (
	"database/sql"

	"github.com/htopete/stackrigs/internal/model"
)

type UptimeStore struct {
	db *sql.DB
}

func NewUptimeStore(db *sql.DB) *UptimeStore {
	return &UptimeStore{db: db}
}

// RecordCheck inserts a health check ping.
func (s *UptimeStore) RecordCheck(status string, responseMs int) error {
	_, err := s.db.Exec(
		`INSERT INTO uptime_checks (status, response_ms) VALUES (?, ?)`,
		status, responseMs,
	)
	return err
}

// GetDailySummary returns uptime stats per day for the last N days.
func (s *UptimeStore) GetDailySummary(days int) ([]model.UptimeDay, error) {
	rows, err := s.db.Query(`
		SELECT
			date(checked_at) AS day,
			SUM(CASE WHEN status = 'ok' THEN 1 ELSE 0 END) AS checks_ok,
			COUNT(*) AS checks_total
		FROM uptime_checks
		WHERE checked_at >= datetime('now', ? || ' days')
		GROUP BY day
		ORDER BY day ASC
	`, -days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build a map of day → data
	dayMap := make(map[string]model.UptimeDay)
	for rows.Next() {
		var d model.UptimeDay
		if err := rows.Scan(&d.Date, &d.ChecksOK, &d.ChecksTotal); err != nil {
			return nil, err
		}
		if d.ChecksTotal > 0 {
			d.UptimePct = float64(d.ChecksOK) / float64(d.ChecksTotal) * 100
		}
		switch {
		case d.UptimePct >= 99:
			d.Status = "up"
		case d.UptimePct > 0:
			d.Status = "partial"
		default:
			d.Status = "down"
		}
		dayMap[d.Date] = d
	}

	// Fill in all days (including days with no data = "unknown")
	result := make([]model.UptimeDay, days)
	for i := 0; i < days; i++ {
		// Query the date for (days-1-i) days ago
		var dateStr string
		offset := -(days - 1 - i)
		err := s.db.QueryRow(
			`SELECT date('now', ? || ' days')`, offset,
		).Scan(&dateStr)
		if err != nil {
			continue
		}

		if d, ok := dayMap[dateStr]; ok {
			result[i] = d
		} else {
			result[i] = model.UptimeDay{
				Date:   dateStr,
				Status: "unknown",
			}
		}
	}

	return result, nil
}

// Cleanup removes checks older than the given number of days.
func (s *UptimeStore) Cleanup(keepDays int) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM uptime_checks WHERE checked_at < datetime('now', ? || ' days')`,
		-keepDays,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
