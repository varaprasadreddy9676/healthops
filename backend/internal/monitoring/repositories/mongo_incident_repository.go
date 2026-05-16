package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"medics-health-check/backend/internal/monitoring"
)

// ErrIncidentRepoNotConfigured is returned when the incident repository is not
// initialized with a mongo client.
var ErrIncidentRepoNotConfigured = errors.New("incident repository is not configured")

// ErrIncidentNotFound is returned when an incident lookup misses.
var ErrIncidentNotFound = errors.New("incident not found")

// ErrIncidentExists is returned when CreateIncident is called for an id that
// already exists.
var ErrIncidentExists = errors.New("incident already exists")

// mongoIncidentOpTimeout bounds per-operation Mongo work. The retention/prune
// path uses a longer dedicated timeout because it can scan many documents.
const mongoIncidentOpTimeout = 5 * time.Second

// MongoIncidentRepository implements monitoring.IncidentRepository backed by
// MongoDB. It is safe for concurrent use by multiple goroutines.
//
// Collection: <prefix>_incidents
// Document shape mirrors monitoring.Incident via existing bson tags
// (Incident.ID is mapped to _id).
type MongoIncidentRepository struct {
	collection *mongo.Collection
}

// NewMongoIncidentRepository constructs a Mongo-backed incident repository.
// The provided client must already be connected and pinged by the caller; this
// constructor does NOT take ownership of disconnecting it.
func NewMongoIncidentRepository(client *mongo.Client, dbName, prefix string) (*MongoIncidentRepository, error) {
	if client == nil {
		return nil, ErrIncidentRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoIncidentRepository{
		collection: client.Database(dbName).Collection(prefix + "_incidents"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		// Index creation is best-effort; Mongo creates them in background.
		fmt.Printf("WARNING: MongoDB incident index creation deferred: %v\n", err)
	}

	return repo, nil
}

// ensureIndexes creates the indexes required for the standard access patterns
// used by the incident manager (open incident lookup, list-by-time, prune).
func (r *MongoIncidentRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "checkId", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().
				SetName("incidents_checkId_status"),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "updatedAt", Value: -1}},
			Options: options.Index().
				SetName("incidents_status_updatedAt"),
		},
		{
			Keys: bson.D{{Key: "startedAt", Value: -1}},
			Options: options.Index().
				SetName("incidents_startedAt"),
		},
	})
	if err != nil && !indexAlreadyExistsForIncidents(err) {
		return err
	}
	return nil
}

func indexAlreadyExistsForIncidents(err error) bool {
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Name == "IndexOptionsConflict" || cmdErr.Code == 85 || cmdErr.Code == 86
	}
	return false
}

// CreateIncident inserts a new incident. Returns ErrIncidentExists if an
// incident with the same id already exists.
func (r *MongoIncidentRepository) CreateIncident(incident monitoring.Incident) error {
	if r == nil || r.collection == nil {
		return ErrIncidentRepoNotConfigured
	}
	if incident.ID == "" {
		return fmt.Errorf("incident id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoIncidentOpTimeout)
	defer cancel()

	if _, err := r.collection.InsertOne(ctx, incident); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("%w: %s", ErrIncidentExists, incident.ID)
		}
		return fmt.Errorf("insert incident %s: %w", incident.ID, err)
	}
	return nil
}

// GetIncident returns the incident with the given id, or a zero-value
// Incident with nil error when not found (preserves the MemoryIncidentRepository
// contract used by the rest of the codebase).
func (r *MongoIncidentRepository) GetIncident(id string) (monitoring.Incident, error) {
	if r == nil || r.collection == nil {
		return monitoring.Incident{}, ErrIncidentRepoNotConfigured
	}
	if id == "" {
		return monitoring.Incident{}, fmt.Errorf("incident id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoIncidentOpTimeout)
	defer cancel()

	var incident monitoring.Incident
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&incident)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return monitoring.Incident{}, nil
		}
		return monitoring.Incident{}, fmt.Errorf("get incident %s: %w", id, err)
	}
	return incident, nil
}

// UpdateIncident applies the mutator to the stored incident atomically by
// performing a load-mutate-replace cycle. If the mutator returns an error, the
// change is not persisted. Returns an error wrapping ErrIncidentNotFound if
// the incident does not exist.
//
// Note: this is single-document optimistic update — the replace overwrites the
// whole document. Concurrent mutations on the same incident race; the last
// writer wins. Callers should not assume strict serializability across
// goroutines (the in-memory implementation has the same property).
func (r *MongoIncidentRepository) UpdateIncident(id string, mutator func(*monitoring.Incident) error) error {
	if r == nil || r.collection == nil {
		return ErrIncidentRepoNotConfigured
	}
	if id == "" {
		return fmt.Errorf("incident id is required")
	}
	if mutator == nil {
		return fmt.Errorf("mutator is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoIncidentOpTimeout)
	defer cancel()

	var incident monitoring.Incident
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&incident); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return fmt.Errorf("%w: %s", ErrIncidentNotFound, id)
		}
		return fmt.Errorf("load incident %s: %w", id, err)
	}

	if err := mutator(&incident); err != nil {
		return err
	}

	res, err := r.collection.ReplaceOne(ctx, bson.M{"_id": id}, incident)
	if err != nil {
		return fmt.Errorf("update incident %s: %w", id, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("%w: %s", ErrIncidentNotFound, id)
	}
	return nil
}

// ListIncidents returns all incidents, ordered by most-recently-updated first.
func (r *MongoIncidentRepository) ListIncidents() ([]monitoring.Incident, error) {
	if r == nil || r.collection == nil {
		return nil, ErrIncidentRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoIncidentOpTimeout)
	defer cancel()

	cur, err := r.collection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]monitoring.Incident, 0)
	for cur.Next(ctx) {
		var inc monitoring.Incident
		if err := cur.Decode(&inc); err != nil {
			return nil, fmt.Errorf("decode incident: %w", err)
		}
		out = append(out, inc)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}
	return out, nil
}

// FindOpenIncident returns the most recently updated non-resolved incident for
// the given check, matching the MemoryIncidentRepository semantics. Returns a
// zero-value Incident with nil error when no open incident exists.
func (r *MongoIncidentRepository) FindOpenIncident(checkID string) (monitoring.Incident, error) {
	if r == nil || r.collection == nil {
		return monitoring.Incident{}, ErrIncidentRepoNotConfigured
	}
	if checkID == "" {
		return monitoring.Incident{}, fmt.Errorf("checkID is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoIncidentOpTimeout)
	defer cancel()

	filter := bson.M{
		"checkId": checkID,
		"status":  bson.M{"$ne": "resolved"},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "updatedAt", Value: -1}})

	var inc monitoring.Incident
	if err := r.collection.FindOne(ctx, filter, opts).Decode(&inc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return monitoring.Incident{}, nil
		}
		return monitoring.Incident{}, fmt.Errorf("find open incident for %s: %w", checkID, err)
	}
	return inc, nil
}

// PruneBefore deletes resolved incidents whose updatedAt is before the cutoff.
// Open and acknowledged incidents are preserved regardless of age.
func (r *MongoIncidentRepository) PruneBefore(cutoff time.Time) error {
	if r == nil || r.collection == nil {
		return ErrIncidentRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := r.collection.DeleteMany(ctx, bson.M{
		"status":    "resolved",
		"updatedAt": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return fmt.Errorf("prune incidents before %s: %w", cutoff.Format(time.RFC3339), err)
	}
	return nil
}
