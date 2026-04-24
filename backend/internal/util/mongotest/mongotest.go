// Package mongotest provides helpers for integration tests that depend on
// a reachable MongoDB instance. The helpers call t.Skip() (rather than
// t.Fatal()) when the database is unavailable, so the broader test suite
// can run cleanly on machines / CI runners without Mongo.
package mongotest

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DefaultURI is used when MONGODB_URI is not set in the environment.
const DefaultURI = "mongodb://127.0.0.1:27017"

// URI returns the MongoDB URI to use for tests. It honours the MONGODB_URI
// environment variable and falls back to a localhost default.
func URI() string {
	if v := os.Getenv("MONGODB_URI"); v != "" {
		return v
	}
	return DefaultURI
}

// Connect returns a connected, pinged Mongo client. If the database is
// unreachable within selectionTimeout, the test is skipped (not failed).
// The returned client is automatically disconnected via t.Cleanup.
func Connect(t *testing.T, selectionTimeout time.Duration) *mongo.Client {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping mongo integration test in -short mode")
	}

	if selectionTimeout <= 0 {
		selectionTimeout = 2 * time.Second
	}

	uri := URI()
	opts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(selectionTimeout)

	client, err := mongo.Connect(opts)
	if err != nil {
		t.Skipf("mongo unreachable at %s: connect: %v", uri, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), selectionTimeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		t.Skipf("mongo unreachable at %s: ping: %v", uri, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	})

	return client
}
