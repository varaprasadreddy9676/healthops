// Command migrate-to-mongo reads JSONL-backed repositories and bulk-inserts
// their contents into the corresponding MongoDB collections. It is idempotent:
// duplicates are skipped via upsert or insert-ignore semantics.
//
// Usage:
//
//	MONGODB_URI=mongodb://localhost:27017 go run ./cmd/migrate-to-mongo \
//	  --data-dir ./data \
//	  --db healthops \
//	  --prefix healthops
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"medics-health-check/backend/internal/monitoring"
	"medics-health-check/backend/internal/util/jsonl"
)

func main() {
	var (
		dataDir  string
		dbName   string
		prefix   string
		mongoURI string
		dryRun   bool
	)

	flag.StringVar(&dataDir, "data-dir", "data", "Path to the data directory containing JSONL files")
	flag.StringVar(&dbName, "db", "healthops", "MongoDB database name")
	flag.StringVar(&prefix, "prefix", "healthops", "Collection name prefix")
	flag.StringVar(&mongoURI, "uri", "", "MongoDB URI (overrides MONGODB_URI env var)")
	flag.BoolVar(&dryRun, "dry-run", false, "Count records without inserting")
	flag.Parse()

	if mongoURI == "" {
		mongoURI = os.Getenv("MONGODB_URI")
	}
	if mongoURI == "" {
		log.Fatal("MongoDB URI required: use --uri flag or MONGODB_URI env var")
	}

	// Resolve data directory
	if !filepath.IsAbs(dataDir) {
		wd, _ := os.Getwd()
		dataDir = filepath.Join(wd, dataDir)
	}

	log.Printf("Data directory: %s", dataDir)
	log.Printf("MongoDB: %s / db=%s / prefix=%s", mongoURI, dbName, prefix)
	if dryRun {
		log.Printf("DRY RUN — no data will be written")
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connect: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping: %v", err)
	}
	log.Printf("Connected to MongoDB")

	db := client.Database(dbName)
	var totalMigrated int

	// 1. MySQL Samples
	samplesFile := filepath.Join(dataDir, "mysql_samples.jsonl")
	count, err := migrateMySQLSamples(db, prefix, samplesFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating mysql_samples: %v", err)
	} else {
		log.Printf("mysql_samples: %d records migrated", count)
		totalMigrated += count
	}

	// 2. MySQL Deltas
	deltasFile := filepath.Join(dataDir, "mysql_deltas.jsonl")
	count, err = migrateMySQLDeltas(db, prefix, deltasFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating mysql_deltas: %v", err)
	} else {
		log.Printf("mysql_deltas: %d records migrated", count)
		totalMigrated += count
	}

	// 3. Incident Snapshots
	snapshotsFile := filepath.Join(dataDir, "incident_snapshots.jsonl")
	count, err = migrateSnapshots(db, prefix, snapshotsFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating incident_snapshots: %v", err)
	} else {
		log.Printf("incident_snapshots: %d records migrated", count)
		totalMigrated += count
	}

	// 4. Notification Outbox
	outboxFile := filepath.Join(dataDir, "notification_outbox.jsonl")
	count, err = migrateNotifications(db, prefix, outboxFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating notification_outbox: %v", err)
	} else {
		log.Printf("notification_outbox: %d records migrated", count)
		totalMigrated += count
	}

	// 5. AI Queue
	queueFile := filepath.Join(dataDir, "ai_queue.jsonl")
	count, err = migrateAIQueue(db, prefix, queueFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating ai_queue: %v", err)
	} else {
		log.Printf("ai_queue: %d records migrated", count)
		totalMigrated += count
	}

	// 6. AI Results
	resultsFile := filepath.Join(dataDir, "ai_results.jsonl")
	count, err = migrateAIResults(db, prefix, resultsFile, dryRun)
	if err != nil {
		log.Printf("ERROR migrating ai_results: %v", err)
	} else {
		log.Printf("ai_results: %d records migrated", count)
		totalMigrated += count
	}

	log.Printf("=== Migration complete: %d total records ===", totalMigrated)
}

// migrateMySQLSamples migrates mysql_samples.jsonl into MongoDB.
func migrateMySQLSamples(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := jsonl.Load[monitoring.MySQLSample](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_mysql_samples")
	return bulkUpsert(coll, items, func(item monitoring.MySQLSample) (bson.M, interface{}) {
		filter := bson.M{"sampleId": item.SampleID}
		return filter, item
	})
}

// migrateMySQLDeltas migrates mysql_deltas.jsonl into MongoDB.
func migrateMySQLDeltas(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := jsonl.Load[monitoring.MySQLDelta](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_mysql_deltas")
	return bulkUpsert(coll, items, func(item monitoring.MySQLDelta) (bson.M, interface{}) {
		filter := bson.M{"sampleId": item.SampleID, "checkId": item.CheckID, "timestamp": item.Timestamp}
		return filter, item
	})
}

// migrateSnapshots migrates incident_snapshots.jsonl into MongoDB.
func migrateSnapshots(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := jsonl.Load[monitoring.IncidentSnapshot](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_incident_snapshots")
	return bulkUpsert(coll, items, func(item monitoring.IncidentSnapshot) (bson.M, interface{}) {
		filter := bson.M{
			"incidentId":   item.IncidentID,
			"snapshotType": item.SnapshotType,
			"timestamp":    item.Timestamp,
		}
		return filter, item
	})
}

// migrateNotifications migrates notification_outbox.jsonl into MongoDB.
func migrateNotifications(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := jsonl.Load[monitoring.NotificationEvent](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_notification_outbox")
	return bulkUpsert(coll, items, func(item monitoring.NotificationEvent) (bson.M, interface{}) {
		filter := bson.M{"notificationId": item.NotificationID}
		return filter, item
	})
}

// migrateAIQueue migrates ai_queue.jsonl into MongoDB.
func migrateAIQueue(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := loadGeneric[aiQueueItem](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_ai_queue")
	return bulkUpsert(coll, items, func(item aiQueueItem) (bson.M, interface{}) {
		filter := bson.M{"incidentId": item.IncidentID, "promptVersion": item.PromptVersion}
		return filter, item
	})
}

// migrateAIResults migrates ai_results.jsonl into MongoDB.
func migrateAIResults(db *mongo.Database, prefix, path string, dryRun bool) (int, error) {
	items, err := loadGeneric[aiResultItem](path)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(items), nil
	}

	coll := db.Collection(prefix + "_ai_results")
	return bulkUpsert(coll, items, func(item aiResultItem) (bson.M, interface{}) {
		filter := bson.M{"incidentId": item.IncidentID, "createdAt": item.CreatedAt}
		return filter, item
	})
}

// aiQueueItem mirrors the AI queue JSONL line shape.
type aiQueueItem struct {
	IncidentID    string     `json:"incidentId" bson:"incidentId"`
	PromptVersion string     `json:"promptVersion" bson:"promptVersion"`
	Status        string     `json:"status" bson:"status"`
	CreatedAt     time.Time  `json:"createdAt" bson:"createdAt"`
	ClaimedAt     *time.Time `json:"claimedAt,omitempty" bson:"claimedAt,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	LastError     string     `json:"lastError,omitempty" bson:"lastError,omitempty"`
}

// aiResultItem mirrors the AI result JSONL line shape.
type aiResultItem struct {
	IncidentID  string    `json:"incidentId" bson:"incidentId"`
	Analysis    string    `json:"analysis" bson:"analysis"`
	Suggestions []string  `json:"suggestions" bson:"suggestions"`
	Severity    string    `json:"severity" bson:"severity"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
}

// loadGeneric reads a JSONL file into a slice using json.Unmarshal.
func loadGeneric[T any](path string) ([]T, error) {
	return jsonl.Load[T](path)
}

// bulkUpsert performs batched upserts (1000 per batch) and returns count of upserted docs.
func bulkUpsert[T any](coll *mongo.Collection, items []T, keyFn func(T) (bson.M, interface{})) (int, error) {
	const batchSize = 1000
	total := 0

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		models := make([]mongo.WriteModel, len(batch))
		for j, item := range batch {
			filter, replacement := keyFn(item)
			models[j] = mongo.NewReplaceOneModel().
				SetFilter(filter).
				SetReplacement(replacement).
				SetUpsert(true)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		result, err := coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
		cancel()
		if err != nil {
			return total, fmt.Errorf("bulk write at offset %d: %w", i, err)
		}
		total += int(result.UpsertedCount) + int(result.ModifiedCount)
	}

	return total, nil
}

// loadJSON is a helper for single-file JSON (not JSONL) if needed in the future.
func loadJSON[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, err
	}
	return result, nil
}
