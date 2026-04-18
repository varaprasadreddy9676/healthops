package monitoring

import (
	"fmt"
	"sync"
)

// MemoryIncidentRepository provides an in-memory implementation of IncidentRepository
type MemoryIncidentRepository struct {
	mu        sync.RWMutex
	incidents map[string]Incident
}

// NewMemoryIncidentRepository creates a new in-memory incident repository
func NewMemoryIncidentRepository() *MemoryIncidentRepository {
	return &MemoryIncidentRepository{
		incidents: make(map[string]Incident),
	}
}

// CreateIncident creates a new incident
func (r *MemoryIncidentRepository) CreateIncident(incident Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.incidents[incident.ID]; exists {
		return fmt.Errorf("incident already exists: %s", incident.ID)
	}

	r.incidents[incident.ID] = incident
	return nil
}

// UpdateIncident updates an incident using a mutator function
func (r *MemoryIncidentRepository) UpdateIncident(id string, mutator func(*Incident) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	incident, exists := r.incidents[id]
	if !exists {
		return fmt.Errorf("incident not found: %s", id)
	}

	// Create a copy to mutate
	incidentCopy := incident
	if err := mutator(&incidentCopy); err != nil {
		return err
	}

	r.incidents[id] = incidentCopy
	return nil
}

// GetIncident retrieves an incident by ID
func (r *MemoryIncidentRepository) GetIncident(id string) (Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	incident, exists := r.incidents[id]
	if !exists {
		return Incident{}, nil
	}

	// Return a copy
	incidentCopy := incident
	if incident.Metadata != nil {
		incidentCopy.Metadata = copyMap(incident.Metadata)
	}

	return incidentCopy, nil
}

// ListIncidents returns all incidents
func (r *MemoryIncidentRepository) ListIncidents() ([]Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	incidents := make([]Incident, 0, len(r.incidents))
	for _, incident := range r.incidents {
		incidentCopy := incident
		if incident.Metadata != nil {
			incidentCopy.Metadata = copyMap(incident.Metadata)
		}
		incidents = append(incidents, incidentCopy)
	}

	return incidents, nil
}

// FindOpenIncident finds the first open incident for a given check ID
func (r *MemoryIncidentRepository) FindOpenIncident(checkID string) (Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, incident := range r.incidents {
		if incident.CheckID == checkID && incident.Status != "resolved" {
			// Return a copy
			incidentCopy := incident
			if incident.Metadata != nil {
				incidentCopy.Metadata = copyMap(incident.Metadata)
			}
			return incidentCopy, nil
		}
	}

	return Incident{}, nil
}

// copyMap creates a shallow copy of a map
func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}

	copy := make(map[string]string, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}
