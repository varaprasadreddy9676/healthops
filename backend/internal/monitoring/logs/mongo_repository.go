package logs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Compile-time interface check.
var _ Repository = (*MongoRepository)(nil)

const mongoLogOpTimeout = 5 * time.Second

// MongoRepository implements Repository backed by MongoDB.
type MongoRepository struct {
	entries  *mongo.Collection
	families *mongo.Collection
}

// NewMongoRepository constructs a Mongo-backed log repository.
func NewMongoRepository(client *mongo.Client, dbName, prefix string) (*MongoRepository, error) {
	if client == nil {
		return nil, errors.New("mongo client is nil")
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	db := client.Database(dbName)
	repo := &MongoRepository{
		entries:  db.Collection(prefix + "_log_entries"),
		families: db.Collection(prefix + "_log_families"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(ctx); err != nil {
		fmt.Printf("WARNING: MongoDB log repository index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("log_ts"),
		},
		{
			Keys:    bson.D{{Key: "source", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("log_source_ts"),
		},
		{
			Keys:    bson.D{{Key: "familyId", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().SetName("log_family_ts"),
		},
		{
			Keys:    bson.D{{Key: "fingerprint", Value: 1}},
			Options: options.Index().SetName("log_fingerprint"),
		},
	})
	if err != nil && !mongoLogIndexExists(err) {
		return fmt.Errorf("create log_entries indexes: %w", err)
	}

	_, err = r.families.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "fingerprint", Value: 1}},
			Options: options.Index().SetName("fam_fingerprint").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "lastSeenAt", Value: -1}},
			Options: options.Index().SetName("fam_status_lastseen"),
		},
		{
			Keys:    bson.D{{Key: "occurrenceCount", Value: -1}},
			Options: options.Index().SetName("fam_count"),
		},
	})
	if err != nil && !mongoLogIndexExists(err) {
		return fmt.Errorf("create log_families indexes: %w", err)
	}

	return nil
}

func mongoLogIndexExists(err error) bool {
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Name == "IndexOptionsConflict" || cmdErr.Code == 85
	}
	return false
}

// IngestEntries processes and stores log entries, upserting error families.
func (r *MongoRepository) IngestEntries(entries []LogEntry) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := range entries {
		entry := &entries[i]

		if entry.Fingerprint == "" {
			entry.Fingerprint = ComputeFingerprint(entry.Message, entry.StackTrace, entry.Source)
		}
		if entry.Category == "" {
			entry.Category = InferEntryCategory(*entry)
		}

		// Upsert error family by fingerprint.
		var family ErrorFamily
		err := r.families.FindOne(ctx, bson.M{"fingerprint": entry.Fingerprint}).Decode(&family)
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return fmt.Errorf("find family by fingerprint: %w", err)
			}
			// Create new family.
			family = ErrorFamily{
				ID:              fmt.Sprintf("fam-%s", entry.Fingerprint[:16]),
				Fingerprint:     entry.Fingerprint,
				Title:           truncate(entry.Message, 120),
				Pattern:         ExtractPattern(entry.Message),
				Source:          entry.Source,
				FirstSeenAt:     entry.Timestamp,
				LastSeenAt:      entry.Timestamp,
				OccurrenceCount: 1,
				Status:          "active",
				Severity:        levelToSeverity(entry.Level),
				Category:        entry.Category,
				SampleMessages:  []string{truncate(entry.Message, 200)},
			}
			if entry.Server != "" {
				family.Servers = []string{entry.Server}
			}
			if _, err := r.families.InsertOne(ctx, family); err != nil {
				if mongo.IsDuplicateKeyError(err) {
					// Race: another goroutine inserted first — re-read.
					if err2 := r.families.FindOne(ctx, bson.M{"fingerprint": entry.Fingerprint}).Decode(&family); err2 != nil {
						return fmt.Errorf("re-read family after dup: %w", err2)
					}
					goto updateFamily
				}
				return fmt.Errorf("insert family: %w", err)
			}
			entry.FamilyID = family.ID
			if _, err := r.entries.InsertOne(ctx, *entry); err != nil {
				return fmt.Errorf("insert log entry: %w", err)
			}
			continue
		}

	updateFamily:
		// Update existing family.
		update := bson.M{
			"$inc": bson.M{"occurrenceCount": 1},
		}
		setFields := bson.M{}
		if entry.Timestamp.After(family.LastSeenAt) {
			setFields["lastSeenAt"] = entry.Timestamp
		}
		if entry.Timestamp.Before(family.FirstSeenAt) {
			setFields["firstSeenAt"] = entry.Timestamp
		}
		if (family.Category == "" || family.Category == CategoryUnknown) && entry.Category != "" {
			setFields["category"] = entry.Category
		}
		if len(setFields) > 0 {
			update["$set"] = setFields
		}

		// Add sample message (cap at 5) and server (dedup).
		pushOps := bson.M{}
		if len(family.SampleMessages) < 5 {
			pushOps["sampleMessages"] = truncate(entry.Message, 200)
		}
		if entry.Server != "" && !contains(family.Servers, entry.Server) {
			pushOps["servers"] = entry.Server
		}
		if len(pushOps) > 0 {
			addToSet := bson.M{}
			for k, v := range pushOps {
				addToSet[k] = v
			}
			update["$addToSet"] = addToSet
		}

		if _, err := r.families.UpdateOne(ctx, bson.M{"_id": family.ID}, update); err != nil {
			return fmt.Errorf("update family %s: %w", family.ID, err)
		}

		entry.FamilyID = family.ID
		if _, err := r.entries.InsertOne(ctx, *entry); err != nil {
			return fmt.Errorf("insert log entry: %w", err)
		}
	}

	return nil
}

// RecentEntries returns recent entries, optionally filtered by source.
func (r *MongoRepository) RecentEntries(source string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	filter := bson.M{}
	if source != "" {
		filter["source"] = source
	}

	cur, err := r.entries.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("find recent entries: %w", err)
	}
	defer cur.Close(ctx)

	var result []LogEntry
	if err := cur.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode recent entries: %w", err)
	}
	return result, nil
}

// EntriesByFamily returns entries belonging to a specific family.
func (r *MongoRepository) EntriesByFamily(familyID string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	cur, err := r.entries.Find(
		ctx,
		bson.M{"familyId": familyID},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("find entries by family: %w", err)
	}
	defer cur.Close(ctx)

	var result []LogEntry
	if err := cur.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode entries by family: %w", err)
	}
	return result, nil
}

// GetFamily returns a single error family by ID.
func (r *MongoRepository) GetFamily(id string) (*ErrorFamily, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	var family ErrorFamily
	if err := r.families.FindOne(ctx, bson.M{"_id": id}).Decode(&family); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("family not found: %s", id)
		}
		return nil, fmt.Errorf("get family %s: %w", id, err)
	}
	return &family, nil
}

// ListFamilies returns families sorted by last seen (newest first), optionally filtered by status.
func (r *MongoRepository) ListFamilies(status string, limit int) ([]ErrorFamily, error) {
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}

	cur, err := r.families.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "lastSeenAt", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list families: %w", err)
	}
	defer cur.Close(ctx)

	var result []ErrorFamily
	if err := cur.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("decode families: %w", err)
	}

	for i := range result {
		result[i].Category = effectiveFamilyCategory(result[i])
	}
	return result, nil
}

// UpdateFamily replaces a family document by ID.
func (r *MongoRepository) UpdateFamily(family ErrorFamily) error {
	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	res, err := r.families.ReplaceOne(ctx, bson.M{"_id": family.ID}, family)
	if err != nil {
		return fmt.Errorf("update family %s: %w", family.ID, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("family not found: %s", family.ID)
	}
	return nil
}

// FamilyStats returns aggregated stats about error families.
func (r *MongoRepository) FamilyStats() LogFamilyStats {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats := LogFamilyStats{
		CategoryCounts: make(map[string]int),
		SeverityCounts: make(map[string]int),
	}

	// Total and active families.
	total, _ := r.families.CountDocuments(ctx, bson.M{})
	stats.TotalFamilies = int(total)

	active, _ := r.families.CountDocuments(ctx, bson.M{"status": "active"})
	stats.ActiveFamilies = int(active)

	// Total entries.
	entryCount, _ := r.entries.CountDocuments(ctx, bson.M{})
	stats.TotalEntries = int(entryCount)

	// Category counts via aggregation.
	catPipeline := bson.A{
		bson.M{"$group": bson.M{"_id": "$category", "count": bson.M{"$sum": 1}}},
	}
	catCur, err := r.families.Aggregate(ctx, catPipeline)
	if err == nil {
		defer catCur.Close(ctx)
		for catCur.Next(ctx) {
			var row struct {
				ID    string `bson:"_id"`
				Count int    `bson:"count"`
			}
			if catCur.Decode(&row) == nil && row.ID != "" {
				stats.CategoryCounts[row.ID] = row.Count
			}
		}
	}

	// Severity counts via aggregation.
	sevPipeline := bson.A{
		bson.M{"$group": bson.M{"_id": "$severity", "count": bson.M{"$sum": 1}}},
	}
	sevCur, err := r.families.Aggregate(ctx, sevPipeline)
	if err == nil {
		defer sevCur.Close(ctx)
		for sevCur.Next(ctx) {
			var row struct {
				ID    string `bson:"_id"`
				Count int    `bson:"count"`
			}
			if sevCur.Decode(&row) == nil && row.ID != "" {
				stats.SeverityCounts[row.ID] = row.Count
			}
		}
	}

	// Top 10 families by occurrence count.
	topPipeline := bson.A{
		bson.M{"$match": bson.M{"status": "active"}},
		bson.M{"$sort": bson.M{"occurrenceCount": -1}},
		bson.M{"$limit": 10},
	}
	topCur, err := r.families.Aggregate(ctx, topPipeline)
	if err == nil {
		defer topCur.Close(ctx)
		var topFamilies []ErrorFamily
		if topCur.All(ctx, &topFamilies) == nil {
			for i := range topFamilies {
				topFamilies[i].Category = effectiveFamilyCategory(topFamilies[i])
			}
			stats.TopFamilies = topFamilies
		}
	}
	if stats.TopFamilies == nil {
		stats.TopFamilies = []ErrorFamily{}
	}

	return stats
}

// PruneBefore removes entries and families older than cutoff.
func (r *MongoRepository) PruneBefore(cutoff time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := r.entries.DeleteMany(ctx, bson.M{"timestamp": bson.M{"$lt": cutoff}}); err != nil {
		return fmt.Errorf("prune log entries: %w", err)
	}
	if _, err := r.families.DeleteMany(ctx, bson.M{"lastSeenAt": bson.M{"$lt": cutoff}}); err != nil {
		return fmt.Errorf("prune log families: %w", err)
	}
	return nil
}

// TotalEntries returns total ingested log entry count.
func (r *MongoRepository) TotalEntries() int {
	ctx, cancel := context.WithTimeout(context.Background(), mongoLogOpTimeout)
	defer cancel()

	count, _ := r.entries.CountDocuments(ctx, bson.M{})
	return int(count)
}
