/*
 * @Author: Lion-3625 lsheng758@gmail.com
 * @Date: 2026-05-18 21:30:27
 * @LastEditors: Lion-3625 lsheng758@gmail.com
 * @LastEditTime: 2026-05-18 21:50:16
 * @FilePath: \protoc-go-inject-tag\swagger\proto_rpc_parser_test.go
 * @Description: 测试 proto rpc 解析器
 *
 * Copyright (c) 2026 by Lion-3625, All Rights Reserved.
 */

package swagger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProtoRPCs(t *testing.T) {
	content := `syntax = "proto3";
package system;

service SystemService {
  rpc DictGroupCreate(DictGroupCreateRequest) returns (DictGroupCreateResponse) {
    option (google.api.http) = { post: "/v1/groups" body: "*" };
  }
  rpc DictGroupList(DictGroupListRequest) returns (DictGroupListResponse) {
    option (google.api.http) = { get: "/v1/groups" };
  }
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.proto")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	rpcs, err := ParseProtoRPCs(path)
	require.NoError(t, err)
	require.Len(t, rpcs, 2)

	assert.Equal(t, "system", rpcs[0].PackageName)
	assert.Equal(t, "SystemService_DictGroupCreate", rpcs[0].OperationID)
	assert.Equal(t, "DictGroupListRequest", rpcs[1].RequestType)
	assert.Equal(t, "SystemService_DictGroupList", rpcs[1].OperationID)
}
