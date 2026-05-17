package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"health-ops/backend/internal/monitoring"
)

var ErrAuditRepoNotConfigured = errors.New("audit repository is not configured")

const mongoAuditOpTimeout = 5 * time.Second

// MongoAuditRepository implements monitoring.AuditRepository backed by MongoDB.
//
// Collection: <prefix>_audit
type MongoAuditRepository struct {
	collection *mongo.Collection
}

var _ monitoring.AuditRepository = (*MongoAuditRepository)(nil)

func NewMongoAuditRepository(client *mongo.Client, dbName, prefix string) (*MongoAuditRepository, error) {
	if client == nil {
		return nil, ErrAuditRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoAuditRepository{
		collection: client.Database(dbName).Collection(prefix + "_audit"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		fmt.Printf("WARNING: MongoDB audit index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoAuditRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("audit_timestamp"),
		},
		{
			Keys:    bson.D{{Key: "action", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("audit_action_ts"),
		},
		{
			Keys:    bson.D{{Key: "actor", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("audit_actor_ts"),
		},
		{
			Keys:    bson.D{{Key: "target", Value: 1}, {Key: "targetId", Value: 1}},
			Options: options.Index().SetName("audit_target"),
		},
	})
	return err
}

func (r *MongoAuditRepository) InsertEvent(event monitoring.AuditEvent) error {
	if r == nil || r.collection == nil {
		return ErrAuditRepoNotConfigured
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAuditOpTimeout)
	defer cancel()

	doc := bson.M{
		"_id":       event.ID,
		"action":    event.Action,
		"actor":     event.Actor,
		"target":    event.Target,
		"targetId":  event.TargetID,
		"details":   event.Details,
		"timestamp": event.Timestamp,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil && mongo.IsDuplicateKeyError(err) {
		return nil // idempotent
	}
	return err
}

func (r *MongoAuditRepository) ListEvents(filter monitoring.AuditFilter) ([]monitoring.AuditEvent, error) {
	if r == nil || r.collection == nil {
		return nil, ErrAuditRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAuditOpTimeout)
	defer cancel()

	mongoFilter := bson.D{}
	if filter.Action != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "action", Value: filter.Action})
	}
	if filter.Actor != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "actor", Value: filter.Actor})
	}
	if filter.Target != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "target", Value: filter.Target})
	}
	if filter.TargetID != "" {
		mongoFilter = append(mongoFilter, bson.E{Key: "targetId", Value: filter.TargetID})
	}
	if !filter.StartTime.IsZero() || !filter.EndTime.IsZero() {
		tsFilter := bson.D{}
		if !filter.StartTime.IsZero() {
			tsFilter = append(tsFilter, bson.E{Key: "$gte", Value: filter.StartTime})
		}
		if !filter.EndTime.IsZero() {
			tsFilter = append(tsFilter, bson.E{Key: "$lte", Value: filter.EndTime})
		}
		mongoFilter = append(mongoFilter, bson.E{Key: "timestamp", Value: tsFilter})
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	if filter.Offset > 0 {
		opts.SetSkip(int64(filter.Offset))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 200
	}
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer cursor.Close(ctx)

	var results []monitoring.AuditEvent
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		evt := monitoring.AuditEvent{
			ID:     strVal(doc, "_id"),
			Action: strVal(doc, "action"),
			Actor:  strVal(doc, "actor"),
			Target: strVal(doc, "target"),
		}
		if v, ok := doc["targetId"].(string); ok {
			evt.TargetID = v
		}
		if v, ok := doc["details"].(bson.M); ok {
			evt.Details = map[string]interface{}(v)
		}
		if v, ok := doc["timestamp"].(time.Time); ok {
			evt.Timestamp = v
		}
		results = append(results, evt)
	}

	if results == nil {
		results = []monitoring.AuditEvent{}
	}
	return results, nil
}

// PruneBefore deletes audit events older than the cutoff.
func (r *MongoAuditRepository) PruneBefore(cutoff time.Time) error {
	if r == nil || r.collection == nil {
		return ErrAuditRepoNotConfigured
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := r.collection.DeleteMany(ctx, bson.D{
		{Key: "timestamp", Value: bson.D{{Key: "$lt", Value: cutoff}}},
	})
	return err
}

func strVal(doc bson.M, key string) string {
	if v, ok := doc[key].(string); ok {
		return v
	}
	return ""
}
