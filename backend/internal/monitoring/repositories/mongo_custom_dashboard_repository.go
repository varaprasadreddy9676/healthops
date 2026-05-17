package repositories

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"medics-health-check/backend/internal/monitoring"
)

// mongoCustomDashboard is the BSON-tagged version of CustomDashboard.
type mongoCustomDashboard struct {
	ID          string                 `bson:"_id"`
	Name        string                 `bson:"name"`
	Description string                 `bson:"description,omitempty"`
	Owner       string                 `bson:"owner,omitempty"`
	IsDefault   bool                   `bson:"isDefault,omitempty"`
	Visibility  string                 `bson:"visibility"`
	CheckIDs    []string               `bson:"checkIds,omitempty"`
	Tags        []string               `bson:"tags,omitempty"`
	Servers     []string               `bson:"servers,omitempty"`
	Widgets     []mongoDashboardWidget `bson:"widgets"`
	RefreshSec  int                    `bson:"refreshSeconds,omitempty"`
	CreatedAt   time.Time              `bson:"createdAt"`
	UpdatedAt   time.Time              `bson:"updatedAt"`
}

type mongoDashboardWidget struct {
	ID       string                 `bson:"id"`
	Type     string                 `bson:"type"`
	Title    string                 `bson:"title"`
	Position mongoWidgetPosition    `bson:"position"`
	Config   map[string]interface{} `bson:"config,omitempty"`
}

type mongoWidgetPosition struct {
	X      int `bson:"x"`
	Y      int `bson:"y"`
	Width  int `bson:"w"`
	Height int `bson:"h"`
}

func toMongoDashboard(d monitoring.CustomDashboard) mongoCustomDashboard {
	widgets := make([]mongoDashboardWidget, len(d.Widgets))
	for i, w := range d.Widgets {
		widgets[i] = mongoDashboardWidget{
			ID:    w.ID,
			Type:  string(w.Type),
			Title: w.Title,
			Position: mongoWidgetPosition{
				X: w.Position.X, Y: w.Position.Y,
				Width: w.Position.Width, Height: w.Position.Height,
			},
			Config: w.Config,
		}
	}
	return mongoCustomDashboard{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Owner:       d.Owner,
		IsDefault:   d.IsDefault,
		Visibility:  d.Visibility,
		CheckIDs:    d.CheckIDs,
		Tags:        d.Tags,
		Servers:     d.Servers,
		Widgets:     widgets,
		RefreshSec:  d.RefreshSec,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func fromMongoDashboard(m mongoCustomDashboard) monitoring.CustomDashboard {
	widgets := make([]monitoring.DashboardWidget, len(m.Widgets))
	for i, w := range m.Widgets {
		widgets[i] = monitoring.DashboardWidget{
			ID:    w.ID,
			Type:  monitoring.WidgetType(w.Type),
			Title: w.Title,
			Position: monitoring.WidgetPosition{
				X: w.Position.X, Y: w.Position.Y,
				Width: w.Position.Width, Height: w.Position.Height,
			},
			Config: w.Config,
		}
	}
	return monitoring.CustomDashboard{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Owner:       m.Owner,
		IsDefault:   m.IsDefault,
		Visibility:  m.Visibility,
		CheckIDs:    m.CheckIDs,
		Tags:        m.Tags,
		Servers:     m.Servers,
		Widgets:     widgets,
		RefreshSec:  m.RefreshSec,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// MongoCustomDashboardRepository implements CustomDashboardRepository backed by MongoDB.
type MongoCustomDashboardRepository struct {
	col *mongo.Collection
}

// Compile-time check.
var _ monitoring.CustomDashboardRepository = (*MongoCustomDashboardRepository)(nil)

// NewMongoCustomDashboardRepository creates the repository and ensures indexes.
func NewMongoCustomDashboardRepository(db *mongo.Database, prefix string) (*MongoCustomDashboardRepository, error) {
	col := db.Collection(prefix + "_custom_dashboards")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "owner", Value: 1}}},
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
	}
	if _, err := col.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create custom dashboard indexes: %w", err)
	}
	return &MongoCustomDashboardRepository{col: col}, nil
}

func (r *MongoCustomDashboardRepository) Create(d monitoring.CustomDashboard) (*monitoring.CustomDashboard, error) {
	if d.Name == "" {
		return nil, fmt.Errorf("dashboard name is required")
	}
	now := time.Now().UTC()
	if d.ID == "" {
		d.ID = fmt.Sprintf("dash-%d", now.UnixNano())
	}
	d.CreatedAt = now
	d.UpdatedAt = now
	if d.Visibility == "" {
		d.Visibility = "private"
	}
	if d.Widgets == nil {
		d.Widgets = []monitoring.DashboardWidget{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc := toMongoDashboard(d)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, fmt.Errorf("insert dashboard: %w", err)
	}
	return &d, nil
}

func (r *MongoCustomDashboardRepository) Get(id string) (*monitoring.CustomDashboard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc mongoCustomDashboard
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("dashboard %q not found", id)
		}
		return nil, fmt.Errorf("get dashboard: %w", err)
	}
	result := fromMongoDashboard(doc)
	return &result, nil
}

func (r *MongoCustomDashboardRepository) List(owner string) []monitoring.CustomDashboard {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if owner != "" {
		filter["$or"] = bson.A{
			bson.M{"owner": owner},
			bson.M{"visibility": "public"},
			bson.M{"visibility": "team"},
		}
	}

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var docs []mongoCustomDashboard
	if err := cursor.All(ctx, &docs); err != nil {
		return nil
	}

	result := make([]monitoring.CustomDashboard, len(docs))
	for i, doc := range docs {
		result[i] = fromMongoDashboard(doc)
	}
	return result
}

func (r *MongoCustomDashboardRepository) Update(id string, update monitoring.CustomDashboard) (*monitoring.CustomDashboard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setFields := bson.M{"updatedAt": time.Now().UTC()}
	if update.Name != "" {
		setFields["name"] = update.Name
	}
	if update.Description != "" {
		setFields["description"] = update.Description
	}
	if update.Owner != "" {
		setFields["owner"] = update.Owner
	}
	if update.Visibility != "" {
		setFields["visibility"] = update.Visibility
	}
	if update.CheckIDs != nil {
		setFields["checkIds"] = update.CheckIDs
	}
	if update.Tags != nil {
		setFields["tags"] = update.Tags
	}
	if update.Servers != nil {
		setFields["servers"] = update.Servers
	}
	if update.Widgets != nil {
		widgets := make([]mongoDashboardWidget, len(update.Widgets))
		for i, w := range update.Widgets {
			widgets[i] = mongoDashboardWidget{
				ID: w.ID, Type: string(w.Type), Title: w.Title,
				Position: mongoWidgetPosition{X: w.Position.X, Y: w.Position.Y, Width: w.Position.Width, Height: w.Position.Height},
				Config:   w.Config,
			}
		}
		setFields["widgets"] = widgets
	}
	if update.RefreshSec > 0 {
		setFields["refreshSeconds"] = update.RefreshSec
	}

	res := r.col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": setFields},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var doc mongoCustomDashboard
	if err := res.Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("dashboard %q not found", id)
		}
		return nil, fmt.Errorf("update dashboard: %w", err)
	}
	result := fromMongoDashboard(doc)
	return &result, nil
}

func (r *MongoCustomDashboardRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete dashboard: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("dashboard %q not found", id)
	}
	return nil
}

func (r *MongoCustomDashboardRepository) Duplicate(id, newName string) (*monitoring.CustomDashboard, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc mongoCustomDashboard
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("dashboard %q not found", id)
		}
		return nil, fmt.Errorf("find dashboard to duplicate: %w", err)
	}

	now := time.Now().UTC()
	doc.ID = fmt.Sprintf("dash-%d", now.UnixNano())
	doc.Name = newName
	doc.IsDefault = false
	doc.CreatedAt = now
	doc.UpdatedAt = now

	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return nil, fmt.Errorf("insert duplicated dashboard: %w", err)
	}
	result := fromMongoDashboard(doc)
	return &result, nil
}
