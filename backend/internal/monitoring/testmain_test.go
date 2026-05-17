package monitoring

import (
	"os"
	"path/filepath"
	"testing"

	"medics-health-check/backend/internal/monitoring/cryptoutil"
)

// TestMain initializes the shared secrets encryption key in a temp dir so that
// tests exercising password encryption/decryption paths work without writing
// to the production data directory.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "healthops-test-crypto-*")
	if err != nil {
		panic("create test crypto dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)
	if err := cryptoutil.Init(filepath.Join(tmpDir, "data")); err != nil {
		panic("init test crypto: " + err.Error())
	}
	os.Exit(m.Run())
}
