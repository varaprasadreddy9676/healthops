package rca

import (
	"context"
	"time"

	"health-ops/backend/internal/monitoring"
)

// AIServiceBridge adapts the AI service's CallProvider method to the RCA AIProvider interface.
type AIServiceBridge struct {
	callProvider func(ctx context.Context, systemMsg, userMsg string) (string, error)
}

// NewAIServiceBridge creates an AIProvider bridge from a function.
func NewAIServiceBridge(callFn func(ctx context.Context, systemMsg, userMsg string) (string, error)) *AIServiceBridge {
	return &AIServiceBridge{callProvider: callFn}
}

func (b *AIServiceBridge) Analyze(ctx context.Context, systemMsg, userMsg string) (string, error) {
	return b.callProvider(ctx, systemMsg, userMsg)
}

// StoreSignalSource adapts a monitoring.Store to the SignalSource interface.
type StoreSignalSource struct {
	store monitoring.Store
}

// NewStoreSignalSource creates a SignalSource backed by the monitoring store.
func NewStoreSignalSource(store monitoring.Store) *StoreSignalSource {
	return &StoreSignalSource{store: store}
}

func (s *StoreSignalSource) RecentResults(checkID string, limit int) []CheckResultRef {
	state := s.store.Snapshot()
	var results []CheckResultRef
	// Results are stored as a flat slice, iterate in reverse for most recent first
	for i := len(state.Results) - 1; i >= 0 && len(results) < limit; i-- {
		r := state.Results[i]
		if r.CheckID != checkID {
			continue
		}
		results = append(results, CheckResultRef{
			CheckID:    r.CheckID,
			Name:       r.Name,
			Type:       r.Type,
			Status:     r.Status,
			DurationMs: r.DurationMs,
			Timestamp:  r.StartedAt,
			Metrics:    r.Metrics,
			Server:     r.Server,
		})
	}
	return results
}

func (s *StoreSignalSource) AllRecentResults(since time.Time, limit int) []CheckResultRef {
	state := s.store.Snapshot()
	var results []CheckResultRef
	for i := len(state.Results) - 1; i >= 0 && len(results) < limit; i-- {
		r := state.Results[i]
		if r.StartedAt.Before(since) {
			break
		}
		results = append(results, CheckResultRef{
			CheckID:    r.CheckID,
			Name:       r.Name,
			Type:       r.Type,
			Status:     r.Status,
			DurationMs: r.DurationMs,
			Timestamp:  r.StartedAt,
			Metrics:    r.Metrics,
			Server:     r.Server,
		})
	}
	return results
}

// IncidentLookup creates a lookup function from an IncidentRepository.
func IncidentLookup(repo monitoring.IncidentRepository) func(id string) *IncidentRef {
	return func(id string) *IncidentRef {
		incident, err := repo.GetIncident(id)
		if err != nil {
			return nil
		}
		return &IncidentRef{
			ID:        incident.ID,
			CheckID:   incident.CheckID,
			CheckName: incident.CheckName,
			Severity:  incident.Severity,
			Status:    incident.Status,
			StartedAt: incident.StartedAt,
			Message:   incident.Message,
		}
	}
}
