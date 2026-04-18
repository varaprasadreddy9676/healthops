package monitoring

import (
	"context"
	"log"
	"sync"
	"time"
)

type HybridStore struct {
	local       *FileStore
	mirror      Mirror
	logger      *log.Logger
	readTimeout time.Duration
	syncTimeout time.Duration
	syncMu      sync.Mutex
}

var _ Store = (*HybridStore)(nil)

func NewHybridStore(statePath string, checks []CheckConfig, mongoURI, mongoDB, mongoPrefix string, retentionDays int, logger *log.Logger) (*HybridStore, error) {
	local, err := NewFileStore(statePath, checks)
	if err != nil {
		return nil, err
	}

	store := &HybridStore{
		local:       local,
		logger:      logger,
		readTimeout: 750 * time.Millisecond,
		syncTimeout: 5 * time.Second,
	}

	if mongoURI != "" {
		mirror, err := NewMongoMirror(mongoURI, mongoDB, mongoPrefix, retentionDays)
		if err != nil {
			if logger != nil {
				logger.Printf("Mongo mirror disabled: %v", err)
			}
		} else {
			store.mirror = mirror
			if err := store.syncBestEffort(); err != nil && logger != nil {
				logger.Printf("initial Mongo sync skipped: %v", err)
			}
		}
	}

	return store, nil
}

func (s *HybridStore) Snapshot() State {
	return s.local.Snapshot()
}

func (s *HybridStore) DashboardSnapshot() DashboardSnapshot {
	local := s.local.DashboardSnapshot()
	if s.mirror != nil {
		ctx, cancel := context.WithTimeout(context.Background(), s.readTimeout)
		snapshot, err := s.mirror.ReadDashboardSnapshot(ctx)
		cancel()
		if err == nil && !isEmptyDashboardSnapshot(snapshot) && !snapshotIsStale(snapshot, local) {
			return snapshot
		}
	}
	return local
}

func (s *HybridStore) Update(mutator func(*State) error) error {
	if err := s.local.Update(mutator); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) ReplaceChecks(checks []CheckConfig) error {
	if err := s.local.ReplaceChecks(checks); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) UpsertCheck(check CheckConfig) error {
	if err := s.local.UpsertCheck(check); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) DeleteCheck(id string) error {
	if err := s.local.DeleteCheck(id); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) AppendResults(results []CheckResult, retentionDays int) error {
	if err := s.local.AppendResults(results, retentionDays); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) SetLastRun(at time.Time) error {
	if err := s.local.SetLastRun(at); err != nil {
		return err
	}
	return s.syncBestEffort()
}

func (s *HybridStore) syncBestEffort() error {
	if s.mirror == nil {
		return nil
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), s.syncTimeout)
	defer cancel()

	if err := s.mirror.SyncState(ctx, s.local.Snapshot()); err != nil {
		if s.logger != nil {
			s.logger.Printf("Mongo sync skipped: %v", err)
		}
		return nil
	}
	return nil
}

func isEmptyState(state State) bool {
	return len(state.Checks) == 0 && len(state.Results) == 0 && state.LastRunAt.IsZero() && state.UpdatedAt.IsZero()
}

func isEmptyDashboardSnapshot(snapshot DashboardSnapshot) bool {
	return isEmptyState(snapshot.State) && snapshot.Summary.TotalChecks == 0 && snapshot.GeneratedAt.IsZero()
}

func snapshotIsStale(snapshot DashboardSnapshot, local DashboardSnapshot) bool {
	if local.State.UpdatedAt.IsZero() {
		return false
	}
	if snapshot.State.UpdatedAt.IsZero() {
		return true
	}
	return snapshot.State.UpdatedAt.Before(local.State.UpdatedAt)
}
