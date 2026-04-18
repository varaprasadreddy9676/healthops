package monitoring

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
)

// Test helpers

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func boolPtr(b bool) *bool {
	return &b
}

// newTestLogger creates a test logger that writes to a buffer
func newTestLogger() *log.Logger {
	return log.New(&bytes.Buffer{}, "", log.LstdFlags)
}
