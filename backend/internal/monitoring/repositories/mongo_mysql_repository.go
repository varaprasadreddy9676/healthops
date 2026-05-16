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

// ErrMySQLRepoNotConfigured is returned when the MySQL metrics repository
// is not initialized with a mongo client.
var ErrMySQLRepoNotConfigured = errors.New("mysql metrics repository is not configured")

const mongoMySQLOpTimeout = 5 * time.Second
const mongoMySQLPruneTimeout = 30 * time.Second

// MongoMySQLRepository implements monitoring.MySQLMetricsRepository backed by MongoDB.
//
// Collections:
//   - <prefix>_mysql_samples – one document per sample, keyed by sampleId
//   - <prefix>_mysql_deltas  – one document per delta, keyed by sampleId
type MongoMySQLRepository struct {
	samplesCollection *mongo.Collection
	deltasCollection  *mongo.Collection
}

// NewMongoMySQLRepository constructs a Mongo-backed MySQL metrics repository.
func NewMongoMySQLRepository(client *mongo.Client, dbName, prefix string) (*MongoMySQLRepository, error) {
	if client == nil {
		return nil, ErrMySQLRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoMySQLRepository{
		samplesCollection: client.Database(dbName).Collection(prefix + "_mysql_samples"),
		deltasCollection:  client.Database(dbName).Collection(prefix + "_mysql_deltas"),
	}

	if err := repo.ensureIndexes(); err != nil {
		return nil, fmt.Errorf("ensure mysql indexes: %w", err)
	}

	return repo, nil
}

func (r *MongoMySQLRepository) ensureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	samplesIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "checkId", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys:    bson.D{{Key: "sampleId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
	}

	deltasIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "checkId", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys:    bson.D{{Key: "sampleId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: -1}},
		},
	}

	if _, err := r.samplesCollection.Indexes().CreateMany(ctx, samplesIndexes); err != nil {
		if !indexAlreadyExists(err) {
			return fmt.Errorf("create samples indexes: %w", err)
		}
	}

	if _, err := r.deltasCollection.Indexes().CreateMany(ctx, deltasIndexes); err != nil {
		if !indexAlreadyExists(err) {
			return fmt.Errorf("create deltas indexes: %w", err)
		}
	}

	return nil
}

// AppendSample persists a MySQL sample to MongoDB. Generates a sampleID if empty.
func (r *MongoMySQLRepository) AppendSample(sample monitoring.MySQLSample) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLOpTimeout)
	defer cancel()

	if sample.SampleID == "" {
		sample.SampleID = fmt.Sprintf("%s-%d", sample.CheckID, time.Now().UnixNano())
	}

	if _, err := r.samplesCollection.InsertOne(ctx, sample); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return sample.SampleID, nil // idempotent
		}
		return "", fmt.Errorf("insert sample: %w", err)
	}
	return sample.SampleID, nil
}

// ComputeAndAppendDelta computes the delta between the given sample and the
// previous sample for the same check, then persists the delta.
func (r *MongoMySQLRepository) ComputeAndAppendDelta(sampleID string) (monitoring.MySQLDelta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLOpTimeout)
	defer cancel()

	// Find the current sample
	var current monitoring.MySQLSample
	err := r.samplesCollection.FindOne(ctx, bson.M{"sampleId": sampleID}).Decode(&current)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return monitoring.MySQLDelta{}, fmt.Errorf("sample not found: %s", sampleID)
		}
		return monitoring.MySQLDelta{}, fmt.Errorf("find sample %s: %w", sampleID, err)
	}

	// Find the previous sample for the same check (most recent before current)
	filter := bson.M{
		"checkId":   current.CheckID,
		"sampleId":  bson.M{"$ne": sampleID},
		"timestamp": bson.M{"$lt": current.Timestamp},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	var previous monitoring.MySQLSample
	err = r.samplesCollection.FindOne(ctx, filter, opts).Decode(&previous)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return monitoring.MySQLDelta{}, fmt.Errorf("no previous sample for check %s", current.CheckID)
		}
		return monitoring.MySQLDelta{}, fmt.Errorf("find previous sample: %w", err)
	}

	delta := monitoring.ComputeDelta(current, previous)

	if _, err := r.deltasCollection.InsertOne(ctx, delta); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return delta, nil // idempotent
		}
		return monitoring.MySQLDelta{}, fmt.Errorf("insert delta: %w", err)
	}
	return delta, nil
}

// LatestSample returns the most recent sample for a given check.
func (r *MongoMySQLRepository) LatestSample(checkID string) (monitoring.MySQLSample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLOpTimeout)
	defer cancel()

	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	var sample monitoring.MySQLSample
	err := r.samplesCollection.FindOne(ctx, bson.M{"checkId": checkID}, opts).Decode(&sample)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return monitoring.MySQLSample{}, fmt.Errorf("no samples found for check %s", checkID)
		}
		return monitoring.MySQLSample{}, fmt.Errorf("find latest sample: %w", err)
	}
	return sample, nil
}

// RecentSamples returns the most recent N samples for a given check, newest first.
func (r *MongoMySQLRepository) RecentSamples(checkID string, limit int) ([]monitoring.MySQLSample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLOpTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.samplesCollection.Find(ctx, bson.M{"checkId": checkID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find recent samples: %w", err)
	}
	defer cursor.Close(ctx)

	var results []monitoring.MySQLSample
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode samples: %w", err)
	}
	return results, nil
}

// RecentDeltas returns the most recent N deltas for a given check, newest first.
func (r *MongoMySQLRepository) RecentDeltas(checkID string, limit int) ([]monitoring.MySQLDelta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLOpTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.deltasCollection.Find(ctx, bson.M{"checkId": checkID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find recent deltas: %w", err)
	}
	defer cursor.Close(ctx)

	var results []monitoring.MySQLDelta
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decode deltas: %w", err)
	}
	return results, nil
}

// PruneBefore removes all samples and deltas older than the cutoff time.
func (r *MongoMySQLRepository) PruneBefore(cutoff time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoMySQLPruneTimeout)
	defer cancel()

	filter := bson.M{"timestamp": bson.M{"$lt": cutoff}}

	if _, err := r.samplesCollection.DeleteMany(ctx, filter); err != nil {
		return fmt.Errorf("prune mysql samples: %w", err)
	}
	if _, err := r.deltasCollection.DeleteMany(ctx, filter); err != nil {
		return fmt.Errorf("prune mysql deltas: %w", err)
	}
	return nil
}
