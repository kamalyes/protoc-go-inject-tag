/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: Lion-3625 lsheng758@gmail.com
 * @LastEditTime: 2026-05-18 21:53:25
 * @FilePath: \protoc-go-inject-tag\swagger\check_test.go
 * @Description: 测试 swagger 与 proto validate一致性检查
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

func TestRunCheck_NoPenetration(t *testing.T) {
	protoContent := `syntax = "proto3";
package system;

message DictGroupCreateRequest {
  string name = 1;  // @inject_tag: validate:"required"
}

message Paging {
  int32 page = 1;
  int32 size = 2;
}

message DictGroupListRequest {
  Paging page_request = 1;  // @inject_tag: validate:"required"
  string name = 2;
}

message DictGroupListResponse {}

service SystemService {
  rpc DictGroupList(DictGroupListRequest) returns (DictGroupListResponse) {
    option (google.api.http) = { get: "/v1/groups" };
  }
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	require.NoError(t, os.WriteFile(protoPath, []byte(protoContent), 0644))

	swaggerPath := writeSwaggerYAML(t, swaggerDoc{
		Paths: newPaths("/v1/groups", pathItem(map[string]*swaggerOperation{
			"get": {
				OperationId: "SystemService_DictGroupList",
				Responses:   map[string]*swaggerResponse{"200": {Description: "ok"}},
				Parameters: []*swaggerParameter{
					{Name: "page_request.page", In: "query", Required: true, Type: "integer"},
					{Name: "name", In: "query", Required: false, Type: "string"},
				},
			},
		})),
	})

	report, err := RunCheck(CheckOptions{
		ProtoFiles:   []string{protoPath},
		SwaggerFiles: []string{swaggerPath},
	})
	require.NoError(t, err)
	assert.False(t, report.HasErrors())
	assert.Empty(t, report.QueryPenetration)
}

func TestRunCheck_DetectsPenetration(t *testing.T) {
	protoContent := `syntax = "proto3";
package system;

message DictGroupCreateRequest {
  string name = 1;  // @inject_tag: validate:"required"
}

message DictGroupListRequest {
  string name = 1;
}

message DictGroupListResponse {}

service SystemService {
  rpc DictGroupList(DictGroupListRequest) returns (DictGroupListResponse) {}
}
`
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "test.proto")
	require.NoError(t, os.WriteFile(protoPath, []byte(protoContent), 0644))

	swaggerPath := writeSwaggerYAML(t, swaggerDoc{
		Paths: newPaths("/v1/groups", pathItem(map[string]*swaggerOperation{
			"get": {
				OperationId: "SystemService_DictGroupList",
				Responses:   map[string]*swaggerResponse{"200": {Description: "ok"}},
				Parameters: []*swaggerParameter{
					{Name: "name", In: "query", Required: true, Type: "string"},
				},
			},
		})),
	})

	report, err := RunCheck(CheckOptions{
		ProtoFiles:   []string{protoPath},
		SwaggerFiles: []string{swaggerPath},
	})
	require.NoError(t, err)
	require.True(t, report.HasErrors())
	require.Len(t, report.QueryPenetration, 1)
	assert.Equal(t, "name", report.QueryPenetration[0].ParamName)
}
