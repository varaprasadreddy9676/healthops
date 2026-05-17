package repositories

import (
	"context"
	"testing"
	"time"

	"health-ops/backend/internal/monitoring"
	"health-ops/backend/internal/util/mongotest"
)

func newTestOutbox(t *testing.T) *MongoNotificationOutbox {
	t.Helper()
	client := mongotest.Connect(t, 2*time.Second)
	repo, err := NewMongoNotificationOutbox(client, "healthops_test", "test")
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.collection.Drop(ctx); err != nil {
		t.Fatalf("drop collection: %v", err)
	}
	return repo
}

func sampleNotification(id string) monitoring.NotificationEvent {
	return monitoring.NotificationEvent{
		NotificationID: id,
		IncidentID:     "inc-" + id,
		Channel:        "email",
		PayloadJSON:    `{"to":"ops@example.com"}`,
		Status:         "pending",
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
}

func TestMongoNotificationOutbox_EnqueueAndList(t *testing.T) {
	repo := newTestOutbox(t)

	if err := repo.Enqueue(sampleNotification("a")); err != nil {
		t.Fatalf("enqueue a: %v", err)
	}
	if err := repo.Enqueue(sampleNotification("b")); err != nil {
		t.Fatalf("enqueue b: %v", err)
	}

	pending, err := repo.ListPending(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}
}

func TestMongoNotificationOutbox_EnqueueIdempotent(t *testing.T) {
	repo := newTestOutbox(t)
	evt := sampleNotification("x")
	if err := repo.Enqueue(evt); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if err := repo.Enqueue(evt); err != nil {
		t.Fatalf("duplicate enqueue should be no-op, got: %v", err)
	}
	pending, _ := repo.ListPending(10)
	if len(pending) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(pending))
	}
}

func TestMongoNotificationOutbox_MarkSent(t *testing.T) {
	repo := newTestOutbox(t)
	_ = repo.Enqueue(sampleNotification("s"))

	if err := repo.MarkSent("s"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	pending, _ := repo.ListPending(10)
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after mark sent, got %d", len(pending))
	}

	all := repo.AllNotifications()
	if len(all) != 1 || all[0].Status != "sent" {
		t.Fatalf("expected 1 sent notification, got %+v", all)
	}
	if all[0].SentAt == nil {
		t.Fatal("SentAt not set")
	}
}

func TestMongoNotificationOutbox_MarkFailed(t *testing.T) {
	repo := newTestOutbox(t)
	_ = repo.Enqueue(sampleNotification("f"))

	if err := repo.MarkFailed("f", "smtp down"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	all := repo.AllNotifications()
	if len(all) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(all))
	}
	if all[0].Status != "failed" || all[0].LastError != "smtp down" {
		t.Fatalf("unexpected after fail: %+v", all[0])
	}
	if all[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", all[0].RetryCount)
	}

	// fail again to verify retry count increments
	_ = repo.MarkFailed("f", "smtp still down")
	all = repo.AllNotifications()
	if all[0].RetryCount != 2 {
		t.Fatalf("expected retry count 2, got %d", all[0].RetryCount)
	}
}

func TestMongoNotificationOutbox_MarkSentMissing(t *testing.T) {
	repo := newTestOutbox(t)
	err := repo.MarkSent("missing")
	if err == nil {
		t.Fatal("expected error for missing notification")
	}
}

func TestMongoNotificationOutbox_PruneBefore(t *testing.T) {
	repo := newTestOutbox(t)

	old := sampleNotification("old")
	old.CreatedAt = time.Now().UTC().Add(-10 * 24 * time.Hour)
	_ = repo.Enqueue(old)

	_ = repo.Enqueue(sampleNotification("recent"))

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if err := repo.PruneBefore(cutoff); err != nil {
		t.Fatalf("prune: %v", err)
	}

	all := repo.AllNotifications()
	if len(all) != 1 {
		t.Fatalf("expected 1 notification after prune, got %d", len(all))
	}
	if all[0].NotificationID != "recent" {
		t.Fatalf("expected recent notification, got %s", all[0].NotificationID)
	}
}

func TestMongoNotificationOutbox_ListPendingLimit(t *testing.T) {
	repo := newTestOutbox(t)
	for i := 0; i < 5; i++ {
		evt := sampleNotification(string(rune('a' + i)))
		evt.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		_ = repo.Enqueue(evt)
	}

	pending, err := repo.ListPending(3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("expected 3 with limit, got %d", len(pending))
	}
	// FIFO ordering
	if pending[0].NotificationID != "a" {
		t.Fatalf("expected oldest first, got %s", pending[0].NotificationID)
	}
}
