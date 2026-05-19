package swagger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedQueryOmitRequired(t *testing.T) {
	content := `syntax = "proto3";
package system;

message Paging {
  int32 page = 1;
  int32 size = 2;
  int32 total = 3;  // @inject_tag: swagger:"omitrequired"
}

message ListRequest {
  Paging page_request = 1;  // @inject_tag: validate:"required"
}

message ListResponse {}

service S {
  rpc List(ListRequest) returns (ListResponse) {}
}
`
	path := writeTempProto(t, content)
	idx, err := BuildProtoFieldIndex([]string{path})
	require.NoError(t, err)

	assert.True(t, idx.childOmitsNestedRequired("ListRequest", "page_request", "total"))
	assert.False(t, idx.childOmitsNestedRequired("ListRequest", "page_request", "page"))

	tagIndex := newSwaggerTagIndex([]ProtoFieldTags{
		{MessageName: "ListRequest", FieldName: "page_request", Tags: FieldTag{Validate: `required`}},
	}, idx)

	validate := validateForParameter(tagIndex, ProtoRPC{RequestType: "ListRequest"}, "page_request.total")
	assert.Empty(t, validate)

	validatePage := validateForParameter(tagIndex, ProtoRPC{RequestType: "ListRequest"}, "page_request.page")
	assert.Equal(t, `required`, validatePage)
}

func TestParseProtoAllFields(t *testing.T) {
	content := `syntax = "proto3";
package common;

message Paging {
  int32 page = 1;
  int32 total = 3;  // @inject_tag: swagger:"omitrequired"
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "p.proto")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	fields, err := ParseProtoAllFields(path)
	require.NoError(t, err)
	require.Len(t, fields, 2)
	assert.Equal(t, "Paging", fields[0].MessageName)
	assert.Equal(t, "int32", fields[0].FieldType)
	assert.True(t, fields[1].Tags.SwaggerOmitRequired)
}
