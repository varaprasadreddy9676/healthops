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

// ErrAIQueueRepoNotConfigured is returned when the AI queue repository is not
// initialized with a mongo client.
var ErrAIQueueRepoNotConfigured = errors.New("ai queue repository is not configured")

const mongoAIQueueOpTimeout = 5 * time.Second

// MongoAIQueue implements monitoring.AIQueueRepository backed by MongoDB.
//
// Collections:
//   - <prefix>_ai_queue   – queue items keyed by incidentId
//   - <prefix>_ai_results – analysis results (append-only)
//
// Queue items are keyed by IncidentID to make Enqueue idempotent at the
// storage layer: a duplicate Enqueue for an incident that already has a
// pending/processing item is a no-op.
type MongoAIQueue struct {
	queueCollection   *mongo.Collection
	resultsCollection *mongo.Collection
}

// aiQueueDoc pairs an IncidentID-keyed _id with the AIQueueItem payload.
type aiQueueDoc struct {
	ID   string                 `bson:"_id"`
	Item monitoring.AIQueueItem `bson:",inline"`
}

// aiResultDoc is the storage shape for AIAnalysisResult, with an auto _id.
type aiResultDoc struct {
	Result monitoring.AIAnalysisResult `bson:",inline"`
}

// NewMongoAIQueue constructs a Mongo-backed AI queue repository.
func NewMongoAIQueue(client *mongo.Client, dbName, prefix string) (*MongoAIQueue, error) {
	if client == nil {
		return nil, ErrAIQueueRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	db := client.Database(dbName)
	repo := &MongoAIQueue{
		queueCollection:   db.Collection(prefix + "_ai_queue"),
		resultsCollection: db.Collection(prefix + "_ai_results"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		fmt.Printf("WARNING: MongoDB AI queue index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoAIQueue) ensureIndexes(ctx context.Context) error {
	if _, err := r.queueCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: 1}},
			Options: options.Index().SetName("aiq_status_createdAt"),
		},
	}); err != nil && !indexAlreadyExistsForAIQueue(err) {
		return err
	}

	if _, err := r.resultsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "incidentId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("air_incidentId_createdAt"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("air_createdAt"),
		},
	}); err != nil && !indexAlreadyExistsForAIQueue(err) {
		return err
	}

	return nil
}

func indexAlreadyExistsForAIQueue(err error) bool {
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Name == "IndexOptionsConflict" || cmdErr.Code == 85 || cmdErr.Code == 86
	}
	return false
}

// Enqueue adds a new AI analysis task for an incident. Idempotent: if there
// is already a pending or processing item for the incident, the call is a
// no-op (matches FileAIQueue semantics).
func (r *MongoAIQueue) Enqueue(incidentID string, promptVersion string) error {
	if r == nil || r.queueCollection == nil {
		return ErrAIQueueRepoNotConfigured
	}
	if incidentID == "" {
		return fmt.Errorf("incident id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	// Check for existing pending/processing item
	var existing aiQueueDoc
	err := r.queueCollection.FindOne(ctx, bson.M{
		"_id":    incidentID,
		"status": bson.M{"$in": []string{"pending", "processing"}},
	}).Decode(&existing)
	if err == nil {
		return nil // idempotent
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return fmt.Errorf("check existing ai queue item for %s: %w", incidentID, err)
	}

	// No active item exists. Replace any terminal item with a fresh pending one.
	item := monitoring.AIQueueItem{
		IncidentID:    incidentID,
		PromptVersion: promptVersion,
		Status:        "pending",
		CreatedAt:     time.Now().UTC(),
	}
	doc := aiQueueDoc{ID: incidentID, Item: item}

	opts := options.Replace().SetUpsert(true)
	if _, err := r.queueCollection.ReplaceOne(ctx, bson.M{"_id": incidentID}, doc, opts); err != nil {
		return fmt.Errorf("enqueue ai queue item for %s: %w", incidentID, err)
	}
	return nil
}

// ClaimPending atomically transitions up to `limit` pending items to
// processing and returns them. Each item is claimed via FindOneAndUpdate so
// concurrent workers do not double-claim.
func (r *MongoAIQueue) ClaimPending(limit int) ([]monitoring.AIQueueItem, error) {
	if r == nil || r.queueCollection == nil {
		return nil, ErrAIQueueRepoNotConfigured
	}
	if limit <= 0 {
		limit = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	claimed := make([]monitoring.AIQueueItem, 0, limit)
	for i := 0; i < limit; i++ {
		now := time.Now().UTC()
		var doc aiQueueDoc
		err := r.queueCollection.FindOneAndUpdate(
			ctx,
			bson.M{"status": "pending"},
			bson.M{"$set": bson.M{"status": "processing", "claimedAt": now}},
			options.FindOneAndUpdate().
				SetSort(bson.D{{Key: "createdAt", Value: 1}}).
				SetReturnDocument(options.After),
		).Decode(&doc)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				break // queue drained
			}
			return claimed, fmt.Errorf("claim pending ai queue item: %w", err)
		}
		claimed = append(claimed, doc.Item)
	}

	return claimed, nil
}

// Complete marks the active queue item as completed and persists the result.
func (r *MongoAIQueue) Complete(incidentID string, result monitoring.AIAnalysisResult) error {
	if r == nil || r.queueCollection == nil || r.resultsCollection == nil {
		return ErrAIQueueRepoNotConfigured
	}
	if incidentID == "" {
		return fmt.Errorf("incident id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	now := time.Now().UTC()
	res, err := r.queueCollection.UpdateOne(
		ctx,
		bson.M{
			"_id":    incidentID,
			"status": bson.M{"$in": []string{"pending", "processing"}},
		},
		bson.M{"$set": bson.M{"status": "completed", "completedAt": now}},
	)
	if err != nil {
		return fmt.Errorf("mark ai queue item completed %s: %w", incidentID, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("no pending/processing AI queue item for incident %s", incidentID)
	}

	result.IncidentID = incidentID
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now().UTC()
	}

	if _, err := r.resultsCollection.InsertOne(ctx, aiResultDoc{Result: result}); err != nil {
		return fmt.Errorf("persist ai analysis result for %s: %w", incidentID, err)
	}
	return nil
}

// Fail marks the active queue item as failed.
func (r *MongoAIQueue) Fail(incidentID string, reason string) error {
	if r == nil || r.queueCollection == nil {
		return ErrAIQueueRepoNotConfigured
	}
	if incidentID == "" {
		return fmt.Errorf("incident id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	res, err := r.queueCollection.UpdateOne(
		ctx,
		bson.M{
			"_id":    incidentID,
			"status": bson.M{"$in": []string{"pending", "processing"}},
		},
		bson.M{"$set": bson.M{"status": "failed", "lastError": reason}},
	)
	if err != nil {
		return fmt.Errorf("mark ai queue item failed %s: %w", incidentID, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("no pending/processing AI queue item for incident %s", incidentID)
	}
	return nil
}

// PruneBefore deletes queue items and results older than cutoff.
func (r *MongoAIQueue) PruneBefore(cutoff time.Time) error {
	if r == nil || r.queueCollection == nil || r.resultsCollection == nil {
		return ErrAIQueueRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := r.queueCollection.DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}}); err != nil {
		return fmt.Errorf("prune ai queue: %w", err)
	}
	if _, err := r.resultsCollection.DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}}); err != nil {
		return fmt.Errorf("prune ai results: %w", err)
	}
	return nil
}

// ListPendingItems returns pending queue items (read-only view).
func (r *MongoAIQueue) ListPendingItems(limit int) ([]monitoring.AIQueueItem, error) {
	if r == nil || r.queueCollection == nil {
		return nil, ErrAIQueueRepoNotConfigured
	}
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	cur, err := r.queueCollection.Find(
		ctx,
		bson.M{"status": "pending"},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list pending ai queue items: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]monitoring.AIQueueItem, 0)
	for cur.Next(ctx) {
		var doc aiQueueDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode ai queue item: %w", err)
		}
		out = append(out, doc.Item)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai queue: %w", err)
	}
	return out, nil
}

// AllItems returns every queue item (any status). Used by the operator UI.
func (r *MongoAIQueue) AllItems() []monitoring.AIQueueItem {
	if r == nil || r.queueCollection == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := r.queueCollection.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)

	out := make([]monitoring.AIQueueItem, 0)
	for cur.Next(ctx) {
		var doc aiQueueDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc.Item)
	}
	return out
}

// GetResults returns all AI analysis results for an incident, newest first.
func (r *MongoAIQueue) GetResults(incidentID string) []monitoring.AIAnalysisResult {
	if r == nil || r.resultsCollection == nil || incidentID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	cur, err := r.resultsCollection.Find(
		ctx,
		bson.M{"incidentId": incidentID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}),
	)
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)

	out := make([]monitoring.AIAnalysisResult, 0)
	for cur.Next(ctx) {
		var doc aiResultDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc.Result)
	}
	return out
}

// AllResults returns the most recent results across all incidents.
func (r *MongoAIQueue) AllResults(limit int) []monitoring.AIAnalysisResult {
	if r == nil || r.resultsCollection == nil {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoAIQueueOpTimeout)
	defer cancel()

	cur, err := r.resultsCollection.Find(
		ctx,
		bson.D{},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil
	}
	defer cur.Close(ctx)

	out := make([]monitoring.AIAnalysisResult, 0)
	for cur.Next(ctx) {
		var doc aiResultDoc
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		out = append(out, doc.Result)
	}
	return out
}
