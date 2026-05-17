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

// ErrServerMetricsRepoNotConfigured is returned when the server metrics
// repository is not initialized with a mongo client.
var ErrServerMetricsRepoNotConfigured = errors.New("server metrics repository is not configured")

const mongoServerMetricsOpTimeout = 5 * time.Second

// MongoServerMetricsRepository implements monitoring.ServerMetricsStore backed by MongoDB.
// Collection: <prefix>_server_metrics
type MongoServerMetricsRepository struct {
	collection *mongo.Collection
}

var _ monitoring.ServerMetricsStore = (*MongoServerMetricsRepository)(nil)

// mongoServerSnapshotDoc wraps ServerSnapshot fields for BSON storage with an auto-generated _id.
type mongoServerSnapshotDoc struct {
	ID                 bson.ObjectID         `bson:"_id"`
	ServerID           string                `bson:"serverId"`
	Timestamp          time.Time             `bson:"timestamp"`
	CPUUsagePercent    float64               `bson:"cpuPercent"`
	MemoryTotalMB      float64               `bson:"memoryTotalMB"`
	MemoryUsedMB       float64               `bson:"memoryUsedMB"`
	MemoryUsagePercent float64               `bson:"memoryPercent"`
	DiskTotalGB        float64               `bson:"diskTotalGB"`
	DiskUsedGB         float64               `bson:"diskUsedGB"`
	DiskUsagePercent   float64               `bson:"diskPercent"`
	LoadAvg1           float64               `bson:"loadAvg1"`
	LoadAvg5           float64               `bson:"loadAvg5"`
	LoadAvg15          float64               `bson:"loadAvg15"`
	UptimeHours        float64               `bson:"uptimeHours"`
	TopProcesses       []mongoProcessInfoDoc `bson:"topProcesses,omitempty"`
}

type mongoProcessInfoDoc struct {
	PID     int     `bson:"pid"`
	User    string  `bson:"user"`
	CPUPct  float64 `bson:"cpuPercent"`
	MemPct  float64 `bson:"memPercent"`
	MemMB   float64 `bson:"memMB"`
	Command string  `bson:"command"`
}

// NewMongoServerMetricsRepository constructs a Mongo-backed server metrics repository.
func NewMongoServerMetricsRepository(client *mongo.Client, dbName, prefix string) (*MongoServerMetricsRepository, error) {
	if client == nil {
		return nil, ErrServerMetricsRepoNotConfigured
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	repo := &MongoServerMetricsRepository{
		collection: client.Database(dbName).Collection(prefix + "_server_metrics"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		fmt.Printf("WARNING: MongoDB server metrics index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoServerMetricsRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "serverId", Value: 1}, {Key: "timestamp", Value: -1}},
			Options: options.Index().
				SetName("srvmetrics_server_ts"),
		},
		{
			Keys: bson.D{{Key: "timestamp", Value: 1}},
			Options: options.Index().
				SetName("srvmetrics_ts"),
		},
	})
	return err
}

// Save stores a server snapshot.
func (r *MongoServerMetricsRepository) Save(snap monitoring.ServerSnapshot) error {
	if r == nil || r.collection == nil {
		return ErrServerMetricsRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoServerMetricsOpTimeout)
	defer cancel()

	doc := toMongoSnapshotDoc(snap)
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("save server snapshot for %s: %w", snap.ServerID, err)
	}
	return nil
}

// GetSnapshots returns snapshots for a server within a time range, sorted by timestamp ascending.
func (r *MongoServerMetricsRepository) GetSnapshots(serverID string, since, until time.Time) ([]monitoring.ServerSnapshot, error) {
	if r == nil || r.collection == nil {
		return nil, ErrServerMetricsRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoServerMetricsOpTimeout)
	defer cancel()

	filter := bson.M{"serverId": serverID}
	tsFilter := bson.M{}
	if !since.IsZero() {
		tsFilter["$gte"] = since
	}
	if !until.IsZero() {
		tsFilter["$lte"] = until
	}
	if len(tsFilter) > 0 {
		filter["timestamp"] = tsFilter
	}

	cur, err := r.collection.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("get server snapshots for %s: %w", serverID, err)
	}
	defer cur.Close(ctx)

	out := make([]monitoring.ServerSnapshot, 0)
	for cur.Next(ctx) {
		var doc mongoServerSnapshotDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode server snapshot: %w", err)
		}
		out = append(out, fromMongoSnapshotDoc(doc))
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("iterate server snapshots: %w", err)
	}
	return out, nil
}

// GetLatest returns the most recent snapshot for a server, or nil if none.
func (r *MongoServerMetricsRepository) GetLatest(serverID string) (*monitoring.ServerSnapshot, error) {
	if r == nil || r.collection == nil {
		return nil, ErrServerMetricsRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), mongoServerMetricsOpTimeout)
	defer cancel()

	var doc mongoServerSnapshotDoc
	err := r.collection.FindOne(
		ctx,
		bson.M{"serverId": serverID},
		options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}}),
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest server snapshot for %s: %w", serverID, err)
	}

	snap := fromMongoSnapshotDoc(doc)
	return &snap, nil
}

// PruneBefore removes all snapshots with timestamp before the cutoff.
func (r *MongoServerMetricsRepository) PruneBefore(cutoff time.Time) error {
	if r == nil || r.collection == nil {
		return ErrServerMetricsRepoNotConfigured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := r.collection.DeleteMany(ctx, bson.M{"timestamp": bson.M{"$lt": cutoff}}); err != nil {
		return fmt.Errorf("prune server metrics before %v: %w", cutoff, err)
	}
	return nil
}

// --- conversion helpers ---

func toMongoSnapshotDoc(snap monitoring.ServerSnapshot) mongoServerSnapshotDoc {
	procs := make([]mongoProcessInfoDoc, len(snap.TopProcesses))
	for i, p := range snap.TopProcesses {
		procs[i] = mongoProcessInfoDoc{
			PID:     p.PID,
			User:    p.User,
			CPUPct:  p.CPUPct,
			MemPct:  p.MemPct,
			MemMB:   p.MemMB,
			Command: p.Command,
		}
	}
	return mongoServerSnapshotDoc{
		ID:                 bson.NewObjectID(),
		ServerID:           snap.ServerID,
		Timestamp:          snap.Timestamp,
		CPUUsagePercent:    snap.CPUUsagePercent,
		MemoryTotalMB:      snap.MemoryTotalMB,
		MemoryUsedMB:       snap.MemoryUsedMB,
		MemoryUsagePercent: snap.MemoryUsagePercent,
		DiskTotalGB:        snap.DiskTotalGB,
		DiskUsedGB:         snap.DiskUsedGB,
		DiskUsagePercent:   snap.DiskUsagePercent,
		LoadAvg1:           snap.LoadAvg1,
		LoadAvg5:           snap.LoadAvg5,
		LoadAvg15:          snap.LoadAvg15,
		UptimeHours:        snap.UptimeHours,
		TopProcesses:       procs,
	}
}

func fromMongoSnapshotDoc(doc mongoServerSnapshotDoc) monitoring.ServerSnapshot {
	procs := make([]monitoring.ProcessInfo, len(doc.TopProcesses))
	for i, p := range doc.TopProcesses {
		procs[i] = monitoring.ProcessInfo{
			PID:     p.PID,
			User:    p.User,
			CPUPct:  p.CPUPct,
			MemPct:  p.MemPct,
			MemMB:   p.MemMB,
			Command: p.Command,
		}
	}
	return monitoring.ServerSnapshot{
		ServerID:           doc.ServerID,
		Timestamp:          doc.Timestamp,
		CPUUsagePercent:    doc.CPUUsagePercent,
		MemoryTotalMB:      doc.MemoryTotalMB,
		MemoryUsedMB:       doc.MemoryUsedMB,
		MemoryUsagePercent: doc.MemoryUsagePercent,
		DiskTotalGB:        doc.DiskTotalGB,
		DiskUsedGB:         doc.DiskUsedGB,
		DiskUsagePercent:   doc.DiskUsagePercent,
		LoadAvg1:           doc.LoadAvg1,
		LoadAvg5:           doc.LoadAvg5,
		LoadAvg15:          doc.LoadAvg15,
		UptimeHours:        doc.UptimeHours,
		TopProcesses:       procs,
	}
}
