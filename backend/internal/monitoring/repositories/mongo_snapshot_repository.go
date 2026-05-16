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

// ErrSnapshotRepoNotConfigured is returned when the snapshot repository
// is not initialized with a mongo client.
var ErrSnapshotRepoNotConfigured = errors.New("snapshot repository is not configured")

const mongoSnapshotOpTimeout = 5 * time.Second
const mongoSnapshotPruneTimeout = 30 * time.Second

// MongoSnapshotRepository implements monitoring.IncidentSnapshotRepository
// backed by MongoDB.
//
// Collection: <prefix>_incident_snapshots
type MongoSnapshotRepository struct {
	collection *mongo.Collection
}

// NewMongoSnapshotRepository constructs a Mongo-backed incident snapshot repository.
func NewMongoSnapshotRepository(client *mongo.Client, dbName, prefix string) (*MongoSnapshotRepository, error) {
	if client == nil {
		return nil, ErrSnapshotRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoSnapshotRepository{
		collection: client.Database(dbName).Collection(prefix + "_incident_snapshots"),
	}

	if err := repo.ensureIndexes(); err != nil {
		return nil, fmt.Errorf("ensure snapshot indexes: %w", err)
	}

	return repo, nil
}

func (r *MongoSnapshotRepository) ensureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "incidentId", Value: 1},
				{Key: "timestamp", Value: 1},
			},
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		if !indexAlreadyExists(err) {
			return fmt.Errorf("create snapshot indexes: %w", err)
		}
	}

	return nil
}

// SaveSnapshots persists a batch of incident evidence snapshots.
func (r *MongoSnapshotRepository) SaveSnapshots(incidentID string, snaps []monitoring.IncidentSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoSnapshotOpTimeout)
	defer cancel()

	docs := make([]interface{}, len(snaps))
	for i, s := range snaps {
		s.IncidentID = incidentID
		docs[i] = s
	}

	if _, err := r.collection.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("save snapshots: %w", err)
	}
	return nil
}

// GetSnapshots returns all snapshots for a given incident, ordered by timestamp.
func (r *MongoSnapshotRepository) GetSnapshots(incidentID string) ([]monitoring.IncidentSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoSnapshotOpTimeout)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := r.collection.Find(ctx, bson.M{"incidentId": incidentID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find snapshots: %w", err)
	}
	defer cursor.Close(ctx)

	var results []monitoring.IncidentSnapshot
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode snapshots: %w", err)
	}
	return results, nil
}

// PruneBefore removes all snapshots older than the cutoff time.
func (r *MongoSnapshotRepository) PruneBefore(cutoff time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoSnapshotPruneTimeout)
	defer cancel()

	filter := bson.M{"timestamp": bson.M{"$lt": cutoff}}
	if _, err := r.collection.DeleteMany(ctx, filter); err != nil {
		return fmt.Errorf("prune snapshots: %w", err)
	}
	return nil
}
