package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"health-ops/backend/internal/monitoring"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrMaintenanceRepoNotConfigured = errors.New("maintenance repository is not configured")

// MongoMaintenanceRepository implements monitoring.MaintenanceWindowStore with MongoDB.
type MongoMaintenanceRepository struct {
	collection *mongo.Collection
}

var _ monitoring.MaintenanceWindowStore = (*MongoMaintenanceRepository)(nil)

// NewMongoMaintenanceRepository creates a MongoDB-backed maintenance window repository.
func NewMongoMaintenanceRepository(client *mongo.Client, dbName, prefix string) (*MongoMaintenanceRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("maintenance repository: mongo client is nil")
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	collection := client.Database(dbName).Collection(prefix + "_maintenance_windows")
	repo := &MongoMaintenanceRepository{collection: collection}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("create maintenance indexes: %w", err)
	}

	return repo, nil
}

func (r *MongoMaintenanceRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "startTime", Value: 1},
				{Key: "endTime", Value: 1},
			},
			Options: options.Index().SetName("maint_time_range"),
		},
		{
			Keys:    bson.D{{Key: "enabled", Value: 1}},
			Options: options.Index().SetName("maint_enabled"),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoMaintenanceRepository) Create(mw monitoring.MaintenanceWindow) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if mw.ID == "" {
		mw.ID = fmt.Sprintf("mw-%d", time.Now().UnixNano())
	}
	if mw.CreatedAt.IsZero() {
		mw.CreatedAt = time.Now().UTC()
	}

	doc := bson.M{
		"_id":            mw.ID,
		"name":           mw.Name,
		"description":    mw.Description,
		"startTime":      mw.StartTime,
		"endTime":        mw.EndTime,
		"checkIds":       mw.CheckIDs,
		"tags":           mw.Tags,
		"servers":        mw.Servers,
		"recurring":      mw.Recurring,
		"recurrenceRule": mw.RecurrenceRule,
		"createdAt":      mw.CreatedAt,
		"createdBy":      mw.CreatedBy,
		"enabled":        mw.Enabled,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("create maintenance window: %w", err)
	}
	return nil
}

func (r *MongoMaintenanceRepository) Update(id string, mutator func(*monitoring.MaintenanceWindow) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mw monitoring.MaintenanceWindow
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&mw)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("maintenance window %q not found", id)
		}
		return fmt.Errorf("find maintenance window: %w", err)
	}

	if err := mutator(&mw); err != nil {
		return err
	}

	opts := options.Replace().SetUpsert(false)
	_, err = r.collection.ReplaceOne(ctx, bson.M{"_id": id}, r.toDoc(mw), opts)
	if err != nil {
		return fmt.Errorf("update maintenance window: %w", err)
	}
	return nil
}

func (r *MongoMaintenanceRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete maintenance window: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("maintenance window %q not found", id)
	}
	return nil
}

func (r *MongoMaintenanceRepository) Get(id string) (monitoring.MaintenanceWindow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var mw monitoring.MaintenanceWindow
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&mw)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return monitoring.MaintenanceWindow{}, fmt.Errorf("maintenance window %q not found", id)
		}
		return monitoring.MaintenanceWindow{}, fmt.Errorf("get maintenance window: %w", err)
	}
	return mw, nil
}

func (r *MongoMaintenanceRepository) List() []monitoring.MaintenanceWindow {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var windows []monitoring.MaintenanceWindow
	if err := cursor.All(ctx, &windows); err != nil {
		return nil
	}
	return windows
}

func (r *MongoMaintenanceRepository) ListActive() []monitoring.MaintenanceWindow {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	filter := bson.M{
		"enabled":   true,
		"endTime":   bson.M{"$gt": now},
		"startTime": bson.M{"$lte": now},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var windows []monitoring.MaintenanceWindow
	if err := cursor.All(ctx, &windows); err != nil {
		return nil
	}

	// For recurring windows, the DB filter is an approximation.
	// Re-check with the struct's IsActive method for correctness.
	var active []monitoring.MaintenanceWindow
	for _, mw := range windows {
		if mw.IsActive(now) {
			active = append(active, mw)
		}
	}

	// Also check recurring windows that might not match the simple time filter
	// but are active due to recurrence projection.
	recurFilter := bson.M{
		"enabled":   true,
		"recurring": true,
	}
	recurCursor, err := r.collection.Find(ctx, recurFilter)
	if err == nil {
		defer recurCursor.Close(ctx)
		var recurWindows []monitoring.MaintenanceWindow
		if recurCursor.All(ctx, &recurWindows) == nil {
			seen := make(map[string]bool, len(active))
			for _, mw := range active {
				seen[mw.ID] = true
			}
			for _, mw := range recurWindows {
				if !seen[mw.ID] && mw.IsActive(now) {
					active = append(active, mw)
				}
			}
		}
	}

	return active
}

func (r *MongoMaintenanceRepository) IsCheckInMaintenance(check monitoring.CheckConfig) bool {
	activeWindows := r.ListActive()
	for _, mw := range activeWindows {
		if mw.CoversCheck(check) {
			return true
		}
	}
	return false
}

func (r *MongoMaintenanceRepository) PruneExpired(cutoff time.Time) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"recurring": bson.M{"$ne": true},
		"endTime":   bson.M{"$lt": cutoff},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("prune maintenance windows: %w", err)
	}
	return int(result.DeletedCount), nil
}

func (r *MongoMaintenanceRepository) toDoc(mw monitoring.MaintenanceWindow) bson.M {
	return bson.M{
		"_id":            mw.ID,
		"name":           mw.Name,
		"description":    mw.Description,
		"startTime":      mw.StartTime,
		"endTime":        mw.EndTime,
		"checkIds":       mw.CheckIDs,
		"tags":           mw.Tags,
		"servers":        mw.Servers,
		"recurring":      mw.Recurring,
		"recurrenceRule": mw.RecurrenceRule,
		"createdAt":      mw.CreatedAt,
		"createdBy":      mw.CreatedBy,
		"enabled":        mw.Enabled,
	}
}
