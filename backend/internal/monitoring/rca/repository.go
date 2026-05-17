package rca

import "time"

// ReportRepository persists RCA reports.
type ReportRepository interface {
	Save(report RCAReport) error
	GetReport(id string) (*RCAReport, error)
	ReportsForIncident(incidentID string) ([]RCAReport, error)
	AllReports(limit int) ([]RCAReport, error)
	PruneBefore(cutoff time.Time) error
}
