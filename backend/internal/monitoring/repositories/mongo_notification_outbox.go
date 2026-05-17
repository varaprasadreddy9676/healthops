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

// ErrNotificationRepoNotConfigured is returned when the notification outbox
// repository is not initialized with a mongo client.
var ErrNotificationRepoNotConfigured = errors.New("notification outbox repository is not configured")

const mongoNotificationOpTimeout = 5 * time.Second

// MongoNotificationOutbox implements monitoring.NotificationOutboxRepository
// backed by MongoDB.
//
// Collection: <prefix>_notification_outbox
// _id is monitoring.NotificationEvent.NotificationID.
type MongoNotificationOutbox struct {
	collection *mongo.Collection
}

// mongoNotificationDoc mirrors NotificationEvent and pins _id to NotificationID
// so duplicate enqueues are idempotent at the storage layer.
type mongoNotificationDoc struct {
	ID    string                       `bson:"_id"`
	Event monitoring.NotificationEvent `bson:",inline"`
}

// NewMongoNotificationOutbox constructs a Mongo-backed notification outbox.
func NewMongoNotificationOutbox(client *mongo.Client, dbName, prefix string) (*MongoNotificationOutbox, error) {
	if client == nil {
		return nil, ErrNotificationRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoNotificationOutbox{
		collection: client.Database(dbName).Collection(prefix + "_notification_outbox"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		fmt.Printf("WARNING: MongoDB notification outbox index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoNotificationOutbox) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: 1}},
			Options: options.Index().
				SetName("notif_status_createdAt"),
		},
		{
			Keys: bson.D{{Key: "incidentId", Value: 1}},
			Options: options.Index().
				SetName("notif_incidentId"),
		},
		{
			Keys: bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().
				SetName("notif_createdAt"),
		},
	})
	if err != nil && !indexAlreadyExistsForIncidents(err) {
		return err
	}
	return nil
}

// Enqueue stores a notification event. Idempotent on NotificationID.
func (r *MongoNotificationOutbox) Enqueue(evt monitoring.NotificationEvent) error {
	if r == nil || r.collection == nil {
		return ErrNotificationRepoNotConfigured
	}
	if evt.NotificationID == "" {
		return fmt.Errorf("notification id is required")
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}
	if evt.Status == "" {
		evt.Status = "pending"
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoNotificationOpTimeout)
	defer cancel()

	doc := mongoNotificationDoc{ID: evt.NotificationID, Event: evt}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil // idempotent
		}
		return fmt.Errorf("enqueue notification %s: %w", evt.NotificationID, err)
	}
	return nil
}

// ListPending returns up to `limit` pending notifications, oldest first.
func (r *MongoNotificationOutbox) ListPending(limit int) ([]monitoring.NotificationEvent, error) {
	if r == nil || r.collection == nil {
		return nil, ErrNotificationRepoNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoNotificationOpTimeout)
	defer cancel()

	cur, err := r.collection.Find(
		ctx,
		bson.M{"status": "pending"},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list pending notifications: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]monitoring.NotificationEvent, 0)
	for cur.Next(ctx) {
		var doc mongoNotificationDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode notification: %w", err)
		}
		out = append(out, doc.Event)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return out, nil
}

// ListAll returns up to `limit` notifications, newest first, optionally filtered by status and/or channel.
func (r *MongoNotificationOutbox) ListAll(limit int, status string, channel string) ([]monitoring.NotificationEvent, error) {
	if r == nil || r.collection == nil {
		return nil, ErrNotificationRepoNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoNotificationOpTimeout)
	defer cancel()

	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	if channel != "" {
		filter["channel"] = bson.M{"$regex": channel, "$options": "i"}
	}

	cur, err := r.collection.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]monitoring.NotificationEvent, 0)
	for cur.Next(ctx) {
		var doc mongoNotificationDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode notification: %w", err)
		}
		out = append(out, doc.Event)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return out, nil
}

// MarkSent transitions a notification to "sent" status and records sentAt.
func (r *MongoNotificationOutbox) MarkSent(id string) error {
	if r == nil || r.collection == nil {
		return ErrNotificationRepoNotConfigured
	}
	if id == "" {
		return fmt.Errorf("notification id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoNotificationOpTimeout)
	defer cancel()

	now := time.Now().UTC()
	res, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"status": "sent", "sentAt": now}},
	)
	if err != nil {
		return fmt.Errorf("mark notification sent %s: %w", id, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}
	return nil
}

// MarkFailed transitions a notification to "failed" status with a reason and
// increments retryCount.
func (r *MongoNotificationOutbox) MarkFailed(id string, reason string) error {
	if r == nil || r.collection == nil {
		return ErrNotificationRepoNotConfigured
	}
	if id == "" {
		return fmt.Errorf("notification id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoNotificationOpTimeout)
	defer cancel()

	res, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{
			"$set": bson.M{"status": "failed", "lastError": reason},
			"$inc": bson.M{"retryCount": 1},
		},
	)
	if err != nil {
		return fmt.Errorf("mark notification failed %s: %w", id, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}
	return nil
}

// PruneBefore deletes notifications older than cutoff regardless of status.
// Matches FileNotificationOutbox behaviour.
func (r *MongoNotificationOutbox) PruneBefore(cutoff time.Time) error {
	if r == nil || r.collection == nil {
		return ErrNotificationRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := r.collection.DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}})
	if err != nil {
		return fmt.Errorf("prune notifications: %w", err)
	}
	return nil
}

// AllNotifications returns every notification (no filter). The API surfaces
// this for the operator dashboard; bounded by the retention job.
func (r *MongoNotificationOutbox) AllNotifications() []monitoring.NotificationEvent {
	if r == nil || r.collection == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := r.collection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)

	out := make([]monitoring.NotificationEvent, 0)
	for cur.Next(ctx) {
		var doc mongoNotificationDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc.Event)
	}
	return out
}
