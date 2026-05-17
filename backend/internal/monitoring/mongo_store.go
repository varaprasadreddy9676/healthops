package monitoring

import (
	"context"
	"errors"
	"fmt"
	"health-ops/backend/internal/monitoring/cryptoutil"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoStore implements Store with MongoDB as the sole persistence layer.
// All reads and writes go directly to MongoDB.
type MongoStore struct {
	client        *mongo.Client
	db            *mongo.Database
	checks        *mongo.Collection
	results       *mongo.Collection
	state         *mongo.Collection
	dashboard     *mongo.Collection
	retentionDays int
	logger        *log.Logger

	// In-memory cache for fast Snapshot() reads, refreshed on every write.
	mu    sync.RWMutex
	cache State
}

var _ Store = (*MongoStore)(nil)

func NewMongoStore(client *mongo.Client, dbName, prefix string, retentionDays int, seedChecks []CheckConfig, logger *log.Logger) (*MongoStore, error) {
	if client == nil {
		return nil, fmt.Errorf("mongo client is required for MongoStore")
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	db := client.Database(dbName)
	s := &MongoStore{
		client:        client,
		db:            db,
		checks:        db.Collection(prefix + "_checks"),
		results:       db.Collection(prefix + "_results"),
		state:         db.Collection(prefix + "_state"),
		dashboard:     db.Collection(prefix + "_dashboard"),
		retentionDays: retentionDays,
		logger:        logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.ensureIndexes(ctx); err != nil {
		logger.Printf("WARNING: MongoStore index creation deferred: %v", err)
	}

	// Load existing state from Mongo
	state, err := s.readStateFromMongo(ctx)
	if err != nil {
		return nil, fmt.Errorf("read initial state from mongo: %w", err)
	}

	// Lazy-migrate: if existing checks in MongoDB have SSH/MySQL blocks
	// with no encrypted password (bson:"-" silently dropped plaintext on
	// previous seeds), recover from the seed config and re-persist.
	if len(state.Checks) > 0 && len(seedChecks) > 0 {
		seedMap := make(map[string]*CheckConfig, len(seedChecks))
		for i := range seedChecks {
			seedMap[seedChecks[i].ID] = &seedChecks[i]
		}
		migrated := 0
		for i := range state.Checks {
			c := &state.Checks[i]
			seed, ok := seedMap[c.ID]
			if !ok {
				continue
			}
			changed := false
			if c.SSH != nil && seed.SSH != nil &&
				c.SSH.Password == "" && c.SSH.PasswordEnc == "" && c.SSH.PasswordEnv == "" &&
				c.SSH.KeyPath == "" && c.SSH.KeyEnv == "" &&
				seed.SSH.Password != "" {
				enc, err := cryptoutil.Encrypt(seed.SSH.Password)
				if err == nil {
					c.SSH.PasswordEnc = enc
					changed = true
				}
			}
			if c.MySQL != nil && seed.MySQL != nil &&
				c.MySQL.Password == "" && c.MySQL.PasswordEnc == "" &&
				seed.MySQL.Password != "" {
				enc, err := cryptoutil.Encrypt(seed.MySQL.Password)
				if err == nil {
					c.MySQL.PasswordEnc = enc
					changed = true
				}
			}
			if changed {
				_ = s.UpsertCheck(state.Checks[i])
				migrated++
			}
		}
		if migrated > 0 {
			logger.Printf("Migrated %d check(s) with missing encrypted credentials from seed config", migrated)
		}
	}

	// Seed checks on first run (no checks in Mongo yet)
	if len(state.Checks) == 0 && len(seedChecks) > 0 {
		state.Checks = cloneChecks(seedChecks)
		// Encrypt any plaintext passwords in seed checks before persisting.
		// Password fields have bson:"-" and would be silently dropped without this.
		for i := range state.Checks {
			if err := prepareCheckSecrets(&state.Checks[i], nil); err != nil {
				logger.Printf("WARNING: failed to encrypt seed check %q secrets: %v", state.Checks[i].ID, err)
			}
		}
		state.UpdatedAt = time.Now().UTC()
		if err := s.writeChecksToMongo(ctx, state.Checks); err != nil {
			return nil, fmt.Errorf("seed checks to mongo: %w", err)
		}
		if err := s.writeStateMeta(ctx, state); err != nil {
			return nil, fmt.Errorf("seed state meta to mongo: %w", err)
		}
		logger.Printf("Seeded %d checks to MongoDB", len(seedChecks))
	}

	s.mu.Lock()
	s.cache = state
	s.mu.Unlock()

	return s, nil
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	_, err := s.results.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "checkId", Value: 1}, {Key: "finishedAt", Value: -1}}},
		{Keys: bson.D{{Key: "finishedAt", Value: -1}}},
	})
	if err != nil && !indexAlreadyExists(err) {
		return err
	}
	return nil
}

func (s *MongoStore) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneState(s.cache)
}

func (s *MongoStore) DashboardSnapshot() DashboardSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildDashboardSnapshot(s.cache)
}

func (s *MongoStore) Update(mutator func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := cloneState(s.cache)
	if err := mutator(&next); err != nil {
		return err
	}
	next.UpdatedAt = time.Now().UTC()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.writeChecksToMongo(ctx, next.Checks); err != nil {
		return fmt.Errorf("mongo update checks: %w", err)
	}
	if err := s.writeStateMeta(ctx, next); err != nil {
		return fmt.Errorf("mongo update state meta: %w", err)
	}

	s.cache = next
	return nil
}

func (s *MongoStore) ReplaceChecks(checks []CheckConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete all existing checks and insert new ones
	if _, err := s.checks.DeleteMany(ctx, bson.D{}); err != nil {
		return fmt.Errorf("mongo delete checks: %w", err)
	}
	if err := s.writeChecksToMongo(ctx, checks); err != nil {
		return fmt.Errorf("mongo write checks: %w", err)
	}

	s.cache.Checks = cloneChecks(checks)
	s.cache.UpdatedAt = time.Now().UTC()
	if err := s.writeStateMeta(ctx, s.cache); err != nil {
		return fmt.Errorf("mongo update state meta: %w", err)
	}
	return nil
}

func (s *MongoStore) UpsertCheck(check CheckConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.checks.ReplaceOne(ctx, bson.M{"_id": check.ID}, check, options.Replace().SetUpsert(true)); err != nil {
		return fmt.Errorf("mongo upsert check: %w", err)
	}

	// Update cache
	found := false
	for i := range s.cache.Checks {
		if s.cache.Checks[i].ID == check.ID {
			s.cache.Checks[i] = check
			found = true
			break
		}
	}
	if !found {
		s.cache.Checks = append(s.cache.Checks, check)
	}
	s.cache.UpdatedAt = time.Now().UTC()
	_ = s.writeStateMeta(ctx, s.cache)
	return nil
}

func (s *MongoStore) DeleteCheck(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := s.checks.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("mongo delete check: %w", err)
	}

	out := s.cache.Checks[:0]
	for _, c := range s.cache.Checks {
		if c.ID != id {
			out = append(out, c)
		}
	}
	s.cache.Checks = out
	s.cache.UpdatedAt = time.Now().UTC()
	_ = s.writeStateMeta(ctx, s.cache)
	return nil
}

func (s *MongoStore) AppendResults(results []CheckResult, retentionDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Write results to Mongo
	if len(results) > 0 {
		docs := make([]interface{}, len(results))
		for i := range results {
			docs[i] = results[i]
		}
		if _, err := s.results.InsertMany(ctx, docs); err != nil {
			// If some are duplicates, ignore; otherwise fail
			if !mongo.IsDuplicateKeyError(err) {
				return fmt.Errorf("mongo insert results: %w", err)
			}
		}
	}

	// Prune old results from Mongo
	if retentionDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		_, _ = s.results.DeleteMany(ctx, bson.M{"finishedAt": bson.M{"$lt": cutoff}})
	}

	// Update cache
	s.cache.Results = append(s.cache.Results, results...)
	pruneResults(&s.cache.Results, retentionDays)
	s.cache.UpdatedAt = time.Now().UTC()

	// Update dashboard snapshot
	_ = s.writeDashboardSnapshot(ctx, s.cache)
	return nil
}

func (s *MongoStore) SetLastRun(at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.LastRunAt = at.UTC()
	s.cache.UpdatedAt = time.Now().UTC()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.writeStateMeta(ctx, s.cache); err != nil {
		return err
	}
	_ = s.writeDashboardSnapshot(ctx, s.cache)
	return nil
}

// Client returns the underlying MongoDB client.
func (s *MongoStore) Client() *mongo.Client {
	return s.client
}

// --- internal Mongo helpers ---

func (s *MongoStore) readStateFromMongo(ctx context.Context) (State, error) {
	checks, err := s.readChecks(ctx)
	if err != nil {
		return State{}, fmt.Errorf("read checks: %w", err)
	}
	results, err := s.readResults(ctx)
	if err != nil {
		return State{}, fmt.Errorf("read results: %w", err)
	}
	meta, err := s.readStateMeta(ctx)
	if err != nil {
		return State{}, fmt.Errorf("read state meta: %w", err)
	}
	return State{
		Checks:    checks,
		Results:   results,
		LastRunAt: meta.LastRunAt,
		UpdatedAt: meta.UpdatedAt,
	}, nil
}

func (s *MongoStore) readChecks(ctx context.Context) ([]CheckConfig, error) {
	cur, err := s.checks.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var checks []CheckConfig
	for cur.Next(ctx) {
		var check CheckConfig
		if err := cur.Decode(&check); err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, cur.Err()
}

func (s *MongoStore) readResults(ctx context.Context) ([]CheckResult, error) {
	filter := bson.D{}
	if s.retentionDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(s.retentionDays) * 24 * time.Hour)
		filter = bson.D{{Key: "finishedAt", Value: bson.M{"$gte": cutoff}}}
	}
	findOpts := options.Find().SetSort(bson.D{{Key: "finishedAt", Value: 1}})
	cur, err := s.results.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results []CheckResult
	for cur.Next(ctx) {
		var result CheckResult
		if err := cur.Decode(&result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, cur.Err()
}

func (s *MongoStore) readStateMeta(ctx context.Context) (State, error) {
	var doc State
	err := s.state.FindOne(ctx, bson.M{"_id": "state"}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return State{}, nil
		}
		return State{}, err
	}
	return doc, nil
}

func (s *MongoStore) writeChecksToMongo(ctx context.Context, checks []CheckConfig) error {
	for _, check := range checks {
		if check.ID == "" {
			continue
		}
		if _, err := s.checks.ReplaceOne(ctx, bson.M{"_id": check.ID}, check, options.Replace().SetUpsert(true)); err != nil {
			return err
		}
	}
	return nil
}

func (s *MongoStore) writeStateMeta(ctx context.Context, state State) error {
	doc := bson.M{
		"_id":       "state",
		"lastRunAt": state.LastRunAt,
		"updatedAt": state.UpdatedAt,
	}
	_, err := s.state.ReplaceOne(ctx, bson.M{"_id": "state"}, doc, options.Replace().SetUpsert(true))
	return err
}

func (s *MongoStore) writeDashboardSnapshot(ctx context.Context, state State) error {
	snapshot := buildDashboardSnapshot(state)
	doc := bson.M{
		"_id":         "dashboard",
		"state":       snapshot.State,
		"summary":     snapshot.Summary,
		"generatedAt": snapshot.GeneratedAt,
	}
	_, err := s.dashboard.ReplaceOne(ctx, bson.M{"_id": "dashboard"}, doc, options.Replace().SetUpsert(true))
	return err
}
