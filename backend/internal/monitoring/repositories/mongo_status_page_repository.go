package repositories

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"health-ops/backend/internal/monitoring"
)

// mongoStatusPageConfig is the BSON-tagged version of StatusPageConfig.
type mongoStatusPageConfig struct {
	ID              string                      `bson:"_id"`
	Name            string                      `bson:"name"`
	Slug            string                      `bson:"slug"`
	Description     string                      `bson:"description,omitempty"`
	LogoURL         string                      `bson:"logoUrl,omitempty"`
	FaviconURL      string                      `bson:"faviconUrl,omitempty"`
	CustomDomain    string                      `bson:"customDomain,omitempty"`
	IsPublic        bool                        `bson:"isPublic"`
	ShowIncidents   bool                        `bson:"showIncidents"`
	ShowUptime      bool                        `bson:"showUptime"`
	UptimeDays      int                         `bson:"uptimeDays"`
	Components      []mongoStatusPageComponent  `bson:"components"`
	AnnouncementMsg string                      `bson:"announcement,omitempty"`
	CreatedAt       time.Time                   `bson:"createdAt"`
	UpdatedAt       time.Time                   `bson:"updatedAt"`
}

type mongoStatusPageComponent struct {
	ID          string   `bson:"id"`
	Name        string   `bson:"name"`
	Description string   `bson:"description,omitempty"`
	CheckIDs    []string `bson:"checkIds,omitempty"`
	Tags        []string `bson:"tags,omitempty"`
	Servers     []string `bson:"servers,omitempty"`
	Order       int      `bson:"order"`
	Group       string   `bson:"group,omitempty"`
}

func toMongoStatusPage(cfg monitoring.StatusPageConfig) mongoStatusPageConfig {
	comps := make([]mongoStatusPageComponent, len(cfg.Components))
	for i, c := range cfg.Components {
		comps[i] = mongoStatusPageComponent{
			ID: c.ID, Name: c.Name, Description: c.Description,
			CheckIDs: c.CheckIDs, Tags: c.Tags, Servers: c.Servers,
			Order: c.Order, Group: c.Group,
		}
	}
	return mongoStatusPageConfig{
		ID: cfg.ID, Name: cfg.Name, Slug: cfg.Slug,
		Description: cfg.Description, LogoURL: cfg.LogoURL,
		FaviconURL: cfg.FaviconURL, CustomDomain: cfg.CustomDomain,
		IsPublic: cfg.IsPublic, ShowIncidents: cfg.ShowIncidents,
		ShowUptime: cfg.ShowUptime, UptimeDays: cfg.UptimeDays,
		Components: comps, AnnouncementMsg: cfg.AnnouncementMsg,
		CreatedAt: cfg.CreatedAt, UpdatedAt: cfg.UpdatedAt,
	}
}

func fromMongoStatusPage(m mongoStatusPageConfig) monitoring.StatusPageConfig {
	comps := make([]monitoring.StatusPageComponent, len(m.Components))
	for i, c := range m.Components {
		comps[i] = monitoring.StatusPageComponent{
			ID: c.ID, Name: c.Name, Description: c.Description,
			CheckIDs: c.CheckIDs, Tags: c.Tags, Servers: c.Servers,
			Order: c.Order, Group: c.Group,
		}
	}
	return monitoring.StatusPageConfig{
		ID: m.ID, Name: m.Name, Slug: m.Slug,
		Description: m.Description, LogoURL: m.LogoURL,
		FaviconURL: m.FaviconURL, CustomDomain: m.CustomDomain,
		IsPublic: m.IsPublic, ShowIncidents: m.ShowIncidents,
		ShowUptime: m.ShowUptime, UptimeDays: m.UptimeDays,
		Components: comps, AnnouncementMsg: m.AnnouncementMsg,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

// MongoStatusPageRepository implements StatusPageRepository backed by MongoDB.
type MongoStatusPageRepository struct {
	col *mongo.Collection
}

// Compile-time check.
var _ monitoring.StatusPageRepository = (*MongoStatusPageRepository)(nil)

// NewMongoStatusPageRepository creates the repository and ensures indexes.
func NewMongoStatusPageRepository(db *mongo.Database, prefix string) (*MongoStatusPageRepository, error) {
	col := db.Collection(prefix + "_status_pages")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "isPublic", Value: 1}}},
	}
	if _, err := col.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create status page indexes: %w", err)
	}
	return &MongoStatusPageRepository{col: col}, nil
}

func (r *MongoStatusPageRepository) Create(cfg monitoring.StatusPageConfig) (*monitoring.StatusPageConfig, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("status page name is required")
	}
	if cfg.Slug == "" {
		return nil, fmt.Errorf("status page slug is required")
	}

	now := time.Now().UTC()
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("sp-%d", now.UnixNano())
	}
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	if cfg.UptimeDays == 0 {
		cfg.UptimeDays = 90
	}
	if cfg.Components == nil {
		cfg.Components = []monitoring.StatusPageComponent{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc := toMongoStatusPage(cfg)
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("slug %q already in use", cfg.Slug)
		}
		return nil, fmt.Errorf("insert status page: %w", err)
	}
	return &cfg, nil
}

func (r *MongoStatusPageRepository) Get(id string) (*monitoring.StatusPageConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc mongoStatusPageConfig
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("status page %q not found", id)
		}
		return nil, fmt.Errorf("get status page: %w", err)
	}
	result := fromMongoStatusPage(doc)
	return &result, nil
}

func (r *MongoStatusPageRepository) GetBySlug(slug string) (*monitoring.StatusPageConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc mongoStatusPageConfig
	if err := r.col.FindOne(ctx, bson.M{"slug": slug}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("status page with slug %q not found", slug)
		}
		return nil, fmt.Errorf("get status page by slug: %w", err)
	}
	result := fromMongoStatusPage(doc)
	return &result, nil
}

func (r *MongoStatusPageRepository) List() []monitoring.StatusPageConfig {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var docs []mongoStatusPageConfig
	if err := cursor.All(ctx, &docs); err != nil {
		return nil
	}

	result := make([]monitoring.StatusPageConfig, len(docs))
	for i, doc := range docs {
		result[i] = fromMongoStatusPage(doc)
	}
	return result
}

func (r *MongoStatusPageRepository) Update(id string, update monitoring.StatusPageConfig) (*monitoring.StatusPageConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setFields := bson.M{
		"name":          update.Name,
		"description":   update.Description,
		"logoUrl":       update.LogoURL,
		"isPublic":      update.IsPublic,
		"showIncidents": update.ShowIncidents,
		"showUptime":    update.ShowUptime,
		"announcement":  update.AnnouncementMsg,
		"updatedAt":     time.Now().UTC(),
	}
	if update.Slug != "" {
		setFields["slug"] = update.Slug
	}
	if update.UptimeDays > 0 {
		setFields["uptimeDays"] = update.UptimeDays
	}
	if update.Components != nil {
		comps := make([]mongoStatusPageComponent, len(update.Components))
		for i, c := range update.Components {
			comps[i] = mongoStatusPageComponent{
				ID: c.ID, Name: c.Name, Description: c.Description,
				CheckIDs: c.CheckIDs, Tags: c.Tags, Servers: c.Servers,
				Order: c.Order, Group: c.Group,
			}
		}
		setFields["components"] = comps
	}

	res := r.col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": setFields},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var doc mongoStatusPageConfig
	if err := res.Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("status page %q not found", id)
		}
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("slug %q already in use", update.Slug)
		}
		return nil, fmt.Errorf("update status page: %w", err)
	}
	result := fromMongoStatusPage(doc)
	return &result, nil
}

func (r *MongoStatusPageRepository) UpdatePartial(id string, update monitoring.StatusPageConfigUpdate) (*monitoring.StatusPageConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setFields := bson.M{"updatedAt": time.Now().UTC()}
	if update.Name != nil && *update.Name != "" {
		setFields["name"] = *update.Name
	}
	if update.Slug != nil && *update.Slug != "" {
		setFields["slug"] = *update.Slug
	}
	if update.Description != nil {
		setFields["description"] = *update.Description
	}
	if update.LogoURL != nil {
		setFields["logoUrl"] = *update.LogoURL
	}
	if update.Components != nil {
		comps := make([]mongoStatusPageComponent, len(update.Components))
		for i, c := range update.Components {
			comps[i] = mongoStatusPageComponent{
				ID: c.ID, Name: c.Name, Description: c.Description,
				CheckIDs: c.CheckIDs, Tags: c.Tags, Servers: c.Servers,
				Order: c.Order, Group: c.Group,
			}
		}
		setFields["components"] = comps
	}
	if update.UptimeDays != nil && *update.UptimeDays > 0 {
		setFields["uptimeDays"] = *update.UptimeDays
	}
	if update.IsPublic != nil {
		setFields["isPublic"] = *update.IsPublic
	}
	if update.ShowIncidents != nil {
		setFields["showIncidents"] = *update.ShowIncidents
	}
	if update.ShowUptime != nil {
		setFields["showUptime"] = *update.ShowUptime
	}
	if update.AnnouncementMsg != nil {
		setFields["announcement"] = *update.AnnouncementMsg
	}

	res := r.col.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": setFields},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var doc mongoStatusPageConfig
	if err := res.Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("status page %q not found", id)
		}
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("slug already in use")
		}
		return nil, fmt.Errorf("partial update status page: %w", err)
	}
	result := fromMongoStatusPage(doc)
	return &result, nil
}

func (r *MongoStatusPageRepository) Delete(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete status page: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("status page %q not found", id)
	}
	return nil
}
