package shared_index

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gocql/gocql"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

func TestQueueEnqueueAndDequeue(t *testing.T) {
	q := newIndexQueue(testLogger)
	if err := q.connect(nil); err != nil {
		t.Skipf("ScyllaDB unavailable: %v", err)
	}
	defer q.close()

	indexName := "test_queue_" + time.Now().Format("150405")

	// Enqueue two items.
	if err := q.Enqueue(indexName, "doc1", `{"id":"doc1","title":"Hello"}`, "", "id", []string{"title"}, "index"); err != nil {
		t.Fatalf("enqueue doc1: %v", err)
	}
	if err := q.Enqueue(indexName, "doc2", `{"id":"doc2","title":"World"}`, "", "id", []string{"title"}, "index"); err != nil {
		t.Fatalf("enqueue doc2: %v", err)
	}

	// Dequeue.
	items, err := q.DequeueBatch(indexName, 10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(items))
	}
	t.Logf("dequeued %d items", len(items))

	// Check index names listing.
	names, err := q.DequeueAllIndexNames()
	if err != nil {
		t.Fatalf("list names: %v", err)
	}
	found := false
	for _, n := range names {
		if n == indexName {
			found = true
		}
	}
	if !found {
		t.Error("index name not found in listing")
	}

	// Clean up.
	var ids []gocql.UUID
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if err := q.DeleteProcessed(indexName, ids); err != nil {
		t.Fatalf("delete processed: %v", err)
	}

	// Verify empty.
	items2, _ := q.DequeueBatch(indexName, 10)
	if len(items2) != 0 {
		t.Errorf("expected 0 items after cleanup, got %d", len(items2))
	}
}

func TestSharedIndexEnqueueAndSearch(t *testing.T) {
	tmp := t.TempDir()
	si := New(Config{
		Group:         "test",
		LocalIndexDir: tmp,
		PollInterval:  100 * time.Millisecond,
		SyncInterval:  1 * time.Second,
	}, testLogger)

	// Connect queue only (skip lease/MinIO for unit test).
	if err := si.queue.connect(nil); err != nil {
		t.Skipf("ScyllaDB unavailable: %v", err)
	}
	defer si.queue.close()

	indexName := "test_search_" + time.Now().Format("150405")

	// Enqueue a document.
	if err := si.Enqueue(indexName, "doc1", `{"id":"doc1","title":"Mesh Ready"}`, "", "id", []string{"title"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Simulate writer: process queue.
	idx, err := si.openIndex(indexName)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	items, err := si.queue.DequeueBatch(indexName, 10)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	for _, item := range items {
		if err := si.applyItem(idx, item); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	// Search.
	results, err := si.Search(indexName, "Mesh", []string{"title"}, 0, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 search result")
	}
	t.Logf("search result: docID=%s score=%.2f data=%s", results[0].DocID, results[0].Score, results[0].Data)

	// Cleanup queue.
	var ids []gocql.UUID
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	_ = si.queue.DeleteProcessed(indexName, ids)
}

