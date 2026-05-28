package evidence

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SignalEventRepository persists and queries SignalEvents in MongoDB.
type SignalEventRepository struct {
	coll *mongo.Collection
}

// NewSignalEventRepository creates a repository backed by the given collection.
func NewSignalEventRepository(client *mongo.Client, dbName, prefix string) (*SignalEventRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("mongo client is required")
	}
	coll := client.Database(dbName).Collection(prefix + "_signal_events")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "service", Value: 1}, {Key: "environment", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "incidentId", Value: 1}, {Key: "timestamp", Value: 1}}},
		{Keys: bson.D{{Key: "fingerprint", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "timestamp", Value: -1}}},
	})
	if err != nil {
		return nil, fmt.Errorf("create signal_events indexes: %w", err)
	}

	return &SignalEventRepository{coll: coll}, nil
}

// Insert stores a single SignalEvent.
func (r *SignalEventRepository) Insert(ctx context.Context, event SignalEvent) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": event.ID}, event, options.Replace().SetUpsert(true))
	return err
}

// InsertMany stores multiple SignalEvents.
func (r *SignalEventRepository) InsertMany(ctx context.Context, events []SignalEvent) error {
	if len(events) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(events))
	for i, e := range events {
		models[i] = mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": e.ID}).
			SetReplacement(e).
			SetUpsert(true)
	}
	_, err := r.coll.BulkWrite(ctx, models)
	return err
}

// FindByIncident returns events for a specific incident, ordered by timestamp.
func (r *SignalEventRepository) FindByIncident(ctx context.Context, incidentID string, limit int) ([]SignalEvent, error) {
	filter := bson.M{"incidentId": incidentID}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	return r.find(ctx, filter, opts)
}

// FindByTimeWindow returns events within a time window.
func (r *SignalEventRepository) FindByTimeWindow(ctx context.Context, start, end time.Time, limit int) ([]SignalEvent, error) {
	filter := bson.M{
		"timestamp": bson.M{"$gte": start, "$lte": end},
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	return r.find(ctx, filter, opts)
}

// PruneBefore deletes events older than the cutoff.
func (r *SignalEventRepository) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.coll.DeleteMany(ctx, bson.M{"timestamp": bson.M{"$lt": cutoff}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *SignalEventRepository) find(ctx context.Context, filter bson.M, opts *options.FindOptionsBuilder) ([]SignalEvent, error) {
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []SignalEvent
	for cur.Next(ctx) {
		var e SignalEvent
		if err := cur.Decode(&e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, cur.Err()
}

// --- IncidentEvent Repository ---

// IncidentEventRepository persists and queries IncidentEvents in MongoDB.
type IncidentEventRepository struct {
	coll *mongo.Collection
}

// NewIncidentEventRepository creates a repository backed by the given collection.
func NewIncidentEventRepository(client *mongo.Client, dbName, prefix string) (*IncidentEventRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("mongo client is required")
	}
	coll := client.Database(dbName).Collection(prefix + "_incident_events")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "incidentId", Value: 1}, {Key: "timestamp", Value: 1}}},
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "timestamp", Value: -1}}},
		{Keys: bson.D{{Key: "timestamp", Value: -1}}},
	})
	if err != nil {
		return nil, fmt.Errorf("create incident_events indexes: %w", err)
	}

	return &IncidentEventRepository{coll: coll}, nil
}

// Insert stores a single IncidentEvent.
func (r *IncidentEventRepository) Insert(ctx context.Context, event IncidentEvent) error {
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": event.ID}, event, options.Replace().SetUpsert(true))
	return err
}

// FindByIncident returns all timeline events for an incident, ordered chronologically.
func (r *IncidentEventRepository) FindByIncident(ctx context.Context, incidentID string) ([]IncidentEvent, error) {
	filter := bson.M{"incidentId": incidentID}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []IncidentEvent
	for cur.Next(ctx) {
		var e IncidentEvent
		if err := cur.Decode(&e); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, cur.Err()
}

// PruneBefore deletes events older than the cutoff.
func (r *IncidentEventRepository) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.coll.DeleteMany(ctx, bson.M{"timestamp": bson.M{"$lt": cutoff}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// --- IncidentBrief Repository ---

// BriefRepository persists AI Incident Briefs in MongoDB.
type BriefRepository struct {
	coll *mongo.Collection
}

// NewBriefRepository creates a repository for incident briefs.
func NewBriefRepository(client *mongo.Client, dbName, prefix string) (*BriefRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("mongo client is required")
	}
	coll := client.Database(dbName).Collection(prefix + "_incident_briefs")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "incidentId", Value: 1}, {Key: "generatedAt", Value: -1}}},
	})
	if err != nil {
		return nil, fmt.Errorf("create incident_briefs indexes: %w", err)
	}

	return &BriefRepository{coll: coll}, nil
}

// Save stores an incident brief. Uses incidentId + generatedAt as a
// composite key to keep history of regenerated briefs.
func (r *BriefRepository) Save(ctx context.Context, brief *IncidentBrief) error {
	id := fmt.Sprintf("brief_%s_%d", brief.IncidentID, brief.GeneratedAt.UnixMilli())
	doc := bson.M{
		"_id":                   id,
		"incidentId":            brief.IncidentID,
		"generatedAt":           brief.GeneratedAt,
		"likelyCause":           brief.LikelyCause,
		"confidence":            brief.Confidence,
		"evidenceSummary":       brief.EvidenceSummary,
		"evidenceLedger":        brief.EvidenceLedger,
		"evidenceLedgerSummary": brief.EvidenceLedgerSummary,
		"nextActions":           brief.NextActions,
		"impactSummary":         brief.ImpactSummary,
		"timeline":              brief.Timeline,
		"metadata":              brief.Metadata,
		"rawAiResponse":         brief.RawAIResponse,
	}
	_, err := r.coll.ReplaceOne(ctx, bson.M{"_id": id}, doc, options.Replace().SetUpsert(true))
	return err
}

// GetLatest returns the most recent brief for an incident.
func (r *BriefRepository) GetLatest(ctx context.Context, incidentID string) (*IncidentBrief, error) {
	filter := bson.M{"incidentId": incidentID}
	opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

	var brief IncidentBrief
	if err := r.coll.FindOne(ctx, filter, opts).Decode(&brief); err != nil {
		return nil, err
	}
	return &brief, nil
}

// ListByIncident returns all briefs for an incident, newest first.
func (r *BriefRepository) ListByIncident(ctx context.Context, incidentID string) ([]IncidentBrief, error) {
	filter := bson.M{"incidentId": incidentID}
	opts := options.Find().SetSort(bson.D{{Key: "generatedAt", Value: -1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var briefs []IncidentBrief
	for cur.Next(ctx) {
		var b IncidentBrief
		if err := cur.Decode(&b); err != nil {
			return nil, err
		}
		briefs = append(briefs, b)
	}
	return briefs, cur.Err()
}
