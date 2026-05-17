package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"health-ops/backend/internal/monitoring/remediation"
)

const remediationOpTimeout = 5 * time.Second

// MongoRemediationRepository implements remediation.Repository backed by MongoDB.
type MongoRemediationRepository struct {
	config   *mongo.Collection
	actions  *mongo.Collection
	attempts *mongo.Collection
}

// NewMongoRemediationRepository creates a Mongo-backed remediation repository.
func NewMongoRemediationRepository(client *mongo.Client, dbName, prefix string) (*MongoRemediationRepository, error) {
	if client == nil {
		return nil, errors.New("mongo client is required")
	}
	if dbName == "" {
		dbName = "healthops"
	}
	if prefix == "" {
		prefix = "healthops"
	}

	db := client.Database(dbName)
	repo := &MongoRemediationRepository{
		config:   db.Collection(prefix + "_remediation_config"),
		actions:  db.Collection(prefix + "_remediation_actions"),
		attempts: db.Collection(prefix + "_remediation_attempts"),
	}

	indexCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := repo.ensureIndexes(indexCtx); err != nil {
		fmt.Printf("WARNING: remediation index creation deferred: %v\n", err)
	}

	return repo, nil
}

func (r *MongoRemediationRepository) ensureIndexes(ctx context.Context) error {
	_, err := r.attempts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "checkId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("rem_attempts_checkId_createdAt"),
		},
		{
			Keys:    bson.D{{Key: "incidentId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("rem_attempts_incidentId_createdAt"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("rem_attempts_createdAt"),
		},
	})
	return err
}

// ---------- Global Config ----------

const configDocID = "global"

func (r *MongoRemediationRepository) GetConfig() (remediation.GlobalConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	var doc struct {
		remediation.GlobalConfig `bson:",inline"`
		ID                       string `bson:"_id"`
	}
	err := r.config.FindOne(ctx, bson.M{"_id": configDocID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return remediation.DefaultGlobalConfig(), nil
	}
	if err != nil {
		return remediation.GlobalConfig{}, fmt.Errorf("get remediation config: %w", err)
	}
	return doc.GlobalConfig, nil
}

func (r *MongoRemediationRepository) SaveConfig(cfg remediation.GlobalConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	doc := bson.M{
		"_id":              configDocID,
		"enabled":          cfg.Enabled,
		"dryRun":           cfg.DryRun,
		"maxConcurrent":    cfg.MaxConcurrent,
		"outputLimitBytes": cfg.OutputLimitBytes,
		"updatedAt":        cfg.UpdatedAt,
	}

	opts := options.Replace().SetUpsert(true)
	_, err := r.config.ReplaceOne(ctx, bson.M{"_id": configDocID}, doc, opts)
	if err != nil {
		return fmt.Errorf("save remediation config: %w", err)
	}
	return nil
}

// ---------- Allowed Actions ----------

func (r *MongoRemediationRepository) ListActions() ([]remediation.AllowedAction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	cursor, err := r.actions.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list remediation actions: %w", err)
	}
	defer cursor.Close(ctx)

	var actions []remediation.AllowedAction
	if err := cursor.All(ctx, &actions); err != nil {
		return nil, fmt.Errorf("decode remediation actions: %w", err)
	}
	if actions == nil {
		actions = []remediation.AllowedAction{}
	}
	return actions, nil
}

func (r *MongoRemediationRepository) GetAction(id string) (remediation.AllowedAction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	var action remediation.AllowedAction
	err := r.actions.FindOne(ctx, bson.M{"_id": id}).Decode(&action)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return remediation.AllowedAction{}, fmt.Errorf("remediation action %q not found", id)
	}
	if err != nil {
		return remediation.AllowedAction{}, fmt.Errorf("get remediation action: %w", err)
	}
	return action, nil
}

func (r *MongoRemediationRepository) CreateAction(action remediation.AllowedAction) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	_, err := r.actions.InsertOne(ctx, action)
	if err != nil {
		return fmt.Errorf("create remediation action: %w", err)
	}
	return nil
}

func (r *MongoRemediationRepository) UpdateAction(action remediation.AllowedAction) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	action.UpdatedAt = time.Now().UTC()
	_, err := r.actions.ReplaceOne(ctx, bson.M{"_id": action.ID}, action)
	if err != nil {
		return fmt.Errorf("update remediation action: %w", err)
	}
	return nil
}

func (r *MongoRemediationRepository) DeleteAction(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	_, err := r.actions.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete remediation action: %w", err)
	}
	return nil
}

// ---------- Attempts ----------

func (r *MongoRemediationRepository) ListAttempts(filter remediation.AttemptFilter) ([]remediation.Attempt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	query := bson.M{}
	if filter.CheckID != "" {
		query["checkId"] = filter.CheckID
	}
	if filter.IncidentID != "" {
		query["incidentId"] = filter.IncidentID
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	skip := filter.Offset
	if skip < 0 {
		skip = 0
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(skip))

	cursor, err := r.attempts.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("list remediation attempts: %w", err)
	}
	defer cursor.Close(ctx)

	var attempts []remediation.Attempt
	if err := cursor.All(ctx, &attempts); err != nil {
		return nil, fmt.Errorf("decode remediation attempts: %w", err)
	}
	if attempts == nil {
		attempts = []remediation.Attempt{}
	}
	return attempts, nil
}

func (r *MongoRemediationRepository) GetAttempt(id string) (remediation.Attempt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	var attempt remediation.Attempt
	err := r.attempts.FindOne(ctx, bson.M{"_id": id}).Decode(&attempt)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return remediation.Attempt{}, fmt.Errorf("remediation attempt %q not found", id)
	}
	if err != nil {
		return remediation.Attempt{}, fmt.Errorf("get remediation attempt: %w", err)
	}
	return attempt, nil
}

func (r *MongoRemediationRepository) CreateAttempt(attempt remediation.Attempt) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	_, err := r.attempts.InsertOne(ctx, attempt)
	if err != nil {
		return fmt.Errorf("create remediation attempt: %w", err)
	}
	return nil
}

func (r *MongoRemediationRepository) UpdateAttempt(id string, mutator func(*remediation.Attempt)) error {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	var attempt remediation.Attempt
	err := r.attempts.FindOne(ctx, bson.M{"_id": id}).Decode(&attempt)
	if err != nil {
		return fmt.Errorf("find attempt for update: %w", err)
	}

	mutator(&attempt)

	_, err = r.attempts.ReplaceOne(ctx, bson.M{"_id": id}, attempt)
	if err != nil {
		return fmt.Errorf("update remediation attempt: %w", err)
	}
	return nil
}

func (r *MongoRemediationRepository) CountAttempts(checkID, incidentID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	query := bson.M{}
	if checkID != "" {
		query["checkId"] = checkID
	}
	if incidentID != "" {
		query["incidentId"] = incidentID
	}

	count, err := r.attempts.CountDocuments(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("count remediation attempts: %w", err)
	}
	return int(count), nil
}

func (r *MongoRemediationRepository) LastAttempt(checkID string) (*remediation.Attempt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remediationOpTimeout)
	defer cancel()

	var attempt remediation.Attempt
	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	err := r.attempts.FindOne(ctx, bson.M{"checkId": checkID}, opts).Decode(&attempt)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last remediation attempt: %w", err)
	}
	return &attempt, nil
}

func (r *MongoRemediationRepository) PruneBefore(cutoff time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := r.attempts.DeleteMany(ctx, bson.M{"createdAt": bson.M{"$lt": cutoff}})
	if err != nil {
		return fmt.Errorf("prune remediation attempts: %w", err)
	}
	return nil
}
