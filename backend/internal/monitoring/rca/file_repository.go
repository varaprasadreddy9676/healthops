package rca

import (
	"fmt"
	"sync"
	"time"

	"medics-health-check/backend/internal/util/jsonl"
)

// FileReportRepository is a legacy file-backed ReportRepository. Retained for tests; production uses MongoDB.
type FileReportRepository struct {
	mu      sync.RWMutex
	reports []RCAReport
	path    string
}

// NewFileReportRepository creates a legacy file-backed report repository (test-only).
func NewFileReportRepository(path string) (*FileReportRepository, error) {
	reports, err := jsonl.Load[RCAReport](path)
	if err != nil {
		return nil, fmt.Errorf("load rca reports: %w", err)
	}
	return &FileReportRepository{
		reports: reports,
		path:    path,
	}, nil
}

func (r *FileReportRepository) Save(report RCAReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	for i, existing := range r.reports {
		if existing.ID == report.ID {
			r.reports[i] = report
			found = true
			break
		}
	}
	if !found {
		r.reports = append(r.reports, report)
	}

	return jsonl.Rewrite(r.path, r.reports)
}

func (r *FileReportRepository) GetReport(id string) (*RCAReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.reports {
		if r.reports[i].ID == id {
			copy := r.reports[i]
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *FileReportRepository) ReportsForIncident(incidentID string) ([]RCAReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []RCAReport
	for _, rpt := range r.reports {
		if rpt.IncidentID == incidentID {
			result = append(result, rpt)
		}
	}
	return result, nil
}

func (r *FileReportRepository) AllReports(limit int) ([]RCAReport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.reports) {
		limit = len(r.reports)
	}

	// Return most recent first
	result := make([]RCAReport, limit)
	for i := 0; i < limit; i++ {
		result[i] = r.reports[len(r.reports)-1-i]
	}
	return result, nil
}

func (r *FileReportRepository) PruneBefore(cutoff time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var kept []RCAReport
	for _, rpt := range r.reports {
		if !rpt.CreatedAt.Before(cutoff) {
			kept = append(kept, rpt)
		}
	}
	r.reports = kept
	return jsonl.Rewrite(r.path, r.reports)
}
