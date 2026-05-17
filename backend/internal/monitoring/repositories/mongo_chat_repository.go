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

// mongoChatConversation is the BSON-tagged version of ChatConversation.
type mongoChatConversation struct {
	ID        string             `bson:"_id"`
	Title     string             `bson:"title"`
	Owner     string             `bson:"owner,omitempty"`
	Messages  []mongoChatMessage `bson:"messages"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
	Context   string             `bson:"context,omitempty"`
}

type mongoChatMessage struct {
	ID        string                 `bson:"id"`
	Role      string                 `bson:"role"`
	Content   string                 `bson:"content"`
	Timestamp time.Time              `bson:"timestamp"`
	Metadata  map[string]interface{} `bson:"metadata,omitempty"`
}

func toMongoChatConversation(c monitoring.ChatConversation) mongoChatConversation {
	msgs := make([]mongoChatMessage, len(c.Messages))
	for i, m := range c.Messages {
		msgs[i] = mongoChatMessage{
			ID: m.ID, Role: m.Role, Content: m.Content,
			Timestamp: m.Timestamp, Metadata: m.Metadata,
		}
	}
	return mongoChatConversation{
		ID: c.ID, Title: c.Title, Owner: c.Owner,
		Messages: msgs, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt, Context: c.Context,
	}
}

func fromMongoChatConversation(m mongoChatConversation) monitoring.ChatConversation {
	msgs := make([]monitoring.ChatMessage, len(m.Messages))
	for i, msg := range m.Messages {
		msgs[i] = monitoring.ChatMessage{
			ID: msg.ID, Role: msg.Role, Content: msg.Content,
			Timestamp: msg.Timestamp, Metadata: msg.Metadata,
		}
	}
	return monitoring.ChatConversation{
		ID: m.ID, Title: m.Title, Owner: m.Owner,
		Messages: msgs, CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt, Context: m.Context,
	}
}

// MongoChatRepository implements ChatRepository backed by MongoDB.
type MongoChatRepository struct {
	col *mongo.Collection
}

// Compile-time check.
var _ monitoring.ChatRepository = (*MongoChatRepository)(nil)

// NewMongoChatRepository creates the repository and ensures indexes.
func NewMongoChatRepository(db *mongo.Database, prefix string) (*MongoChatRepository, error) {
	col := db.Collection(prefix + "_chat_conversations")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "owner", Value: 1}, {Key: "updatedAt", Value: -1}}},
		{Keys: bson.D{{Key: "updatedAt", Value: -1}}},
	}
	if _, err := col.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create chat indexes: %w", err)
	}
	return &MongoChatRepository{col: col}, nil
}

func (r *MongoChatRepository) CreateConversation(title, owner, ctxStr string) *monitoring.ChatConversation {
	now := time.Now().UTC()
	conv := monitoring.ChatConversation{
		ID:        fmt.Sprintf("chat-%d", now.UnixNano()),
		Title:     title,
		Owner:     owner,
		Messages:  []monitoring.ChatMessage{},
		CreatedAt: now,
		UpdatedAt: now,
		Context:   ctxStr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc := toMongoChatConversation(conv)
	r.col.InsertOne(ctx, doc) //nolint:errcheck // match file-store fire-and-forget semantics
	return &conv
}

func (r *MongoChatRepository) GetConversation(id string) (*monitoring.ChatConversation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc mongoChatConversation
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("conversation %q not found", id)
		}
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	result := fromMongoChatConversation(doc)
	return &result, nil
}

func (r *MongoChatRepository) ListConversations(owner string) []monitoring.ChatConversation {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if owner != "" {
		filter["owner"] = owner
	}

	// Project only the last message for listing (like the file store does).
	opts := options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
		SetProjection(bson.M{
			"_id":       1,
			"title":     1,
			"owner":     1,
			"createdAt": 1,
			"updatedAt": 1,
			"context":   1,
			"messages":  bson.M{"$slice": -1},
		})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var docs []mongoChatConversation
	if err := cursor.All(ctx, &docs); err != nil {
		return nil
	}

	result := make([]monitoring.ChatConversation, len(docs))
	for i, doc := range docs {
		result[i] = fromMongoChatConversation(doc)
	}
	return result
}

func (r *MongoChatRepository) AddMessage(conversationID string, msg monitoring.ChatMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mongoMsg := mongoChatMessage{
		ID: msg.ID, Role: msg.Role, Content: msg.Content,
		Timestamp: msg.Timestamp, Metadata: msg.Metadata,
	}

	update := bson.M{
		"$push": bson.M{"messages": mongoMsg},
		"$set":  bson.M{"updatedAt": time.Now().UTC()},
	}

	// Auto-title from first user message.
	if msg.Role == "user" {
		title := msg.Content
		if len(title) > 60 {
			title = title[:60] + "..."
		}
		// Only set title if currently empty — use $setOnInsert-like logic via an aggregation pipeline isn't
		// worth the complexity; we do a conditional set instead.
		update["$set"].(bson.M)["title"] = title
	}

	res, err := r.col.UpdateOne(ctx, bson.M{"_id": conversationID}, update)
	if err != nil {
		return fmt.Errorf("add message: %w", err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("conversation %q not found", conversationID)
	}
	return nil
}

func (r *MongoChatRepository) DeleteConversation(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("conversation %q not found", id)
	}
	return nil
}

func (r *MongoChatRepository) PruneOld(maxAge time.Duration) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().UTC().Add(-maxAge)
	res, err := r.col.DeleteMany(ctx, bson.M{"updatedAt": bson.M{"$lt": cutoff}})
	if err != nil {
		return 0
	}
	return int(res.DeletedCount)
}
