package swagger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandProtoFilesWithImports(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "proto", "common")
	systemDir := filepath.Join(root, "proto", "system")
	require.NoError(t, os.MkdirAll(commonDir, 0755))
	require.NoError(t, os.MkdirAll(systemDir, 0755))

	pagingProto := `syntax = "proto3";
package common;
message Paging {
  int32 total = 3;  // @inject_tag: swagger:"omitrequired"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(commonDir, "pagination.proto"), []byte(pagingProto), 0644))

	systemProto := `syntax = "proto3";
package system;
import "proto/common/pagination.proto";
message ListRequest {
  common.Paging page_request = 1;
}
`
	systemPath := filepath.Join(systemDir, "system.proto")
	require.NoError(t, os.WriteFile(systemPath, []byte(systemProto), 0644))

	expanded, err := ExpandProtoFilesWithImports([]string{systemPath})
	require.NoError(t, err)
	assert.Contains(t, expanded, systemPath)
	assert.Contains(t, expanded, filepath.Join(commonDir, "pagination.proto"))
}
