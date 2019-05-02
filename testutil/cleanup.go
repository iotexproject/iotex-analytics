package testutil

import (
	"os"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// CleanupPath detects the existence of test DB file and removes it if found
func CleanupPath(t *testing.T, path string) {
	if fileutil.FileExists(path) && os.RemoveAll(path) != nil {
		t.Error("Fail to remove testDB file")
	}
}
