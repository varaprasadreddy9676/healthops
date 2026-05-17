package rca

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoReportRepository implements ReportRepository with MongoDB backing.
type MongoReportRepository struct {
	collection *mongo.Collection
}

var _ ReportRepository = (*MongoReportRepository)(nil)
var _ ReportRepository = (*FileReportRepository)(nil)

// NewMongoReportRepository creates a MongoDB-backed report repository.
func NewMongoReportRepository(client *mongo.Client, dbName, prefix string) (*MongoReportRepository, error) {
	if client == nil {
		return nil, fmt.Errorf("rca repository: mongo client is nil")
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	collection := client.Database(dbName).Collection(prefix + "_rca_reports")
	repo := &MongoReportRepository{collection: collection}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("create rca indexes: %w", err)
	}

	return repo, nil
}

func (r *MongoReportRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "incidentId", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index().SetName("rca_incident"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("rca_created"),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoReportRepository) Save(report RCAReport) error {
	if r == nil || r.collection == nil {
		return fmt.Errorf("rca repository not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Replace().SetUpsert(true)
	_, err := r.collection.ReplaceOne(ctx, bson.M{"_id": report.ID}, report, opts)
	if err != nil {
		return fmt.Errorf("save rca report: %w", err)
	}
	return nil
}

func (r *MongoReportRepository) GetReport(id string) (*RCAReport, error) {
	if r == nil || r.collection == nil {
		return nil, fmt.Errorf("rca repository not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var report RCAReport
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&report)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("get rca report: %w", err)
	}
	return &report, nil
}

func (r *MongoReportRepository) ReportsForIncident(incidentID string) ([]RCAReport, error) {
	if r == nil || r.collection == nil {
		return nil, fmt.Errorf("rca repository not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := r.collection.Find(ctx, bson.M{"incidentId": incidentID}, opts)
	if err != nil {
		return nil, fmt.Errorf("find rca reports for incident: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []RCAReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("decode rca reports: %w", err)
	}
	return reports, nil
}

func (r *MongoReportRepository) AllReports(limit int) ([]RCAReport, error) {
	if r == nil || r.collection == nil {
		return nil, fmt.Errorf("rca repository not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("find all rca reports: %w", err)
	}
	defer cursor.Close(ctx)

	var reports []RCAReport
	if err := cursor.All(ctx, &reports); err != nil {
		return nil, fmt.Errorf("decode rca reports: %w", err)
	}
	return reports, nil
}

func (r *MongoReportRepository) PruneBefore(cutoff time.Time) error {
	if r == nil || r.collection == nil {
		return fmt.Errorf("rca repository not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := r.collection.DeleteMany(ctx, bson.M{
		"createdAt": bson.M{"$lt": cutoff},
	})
	if err != nil {
		return fmt.Errorf("prune rca reports: %w", err)
	}
	return nil
}
