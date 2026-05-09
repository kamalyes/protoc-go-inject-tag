/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 12:19:32
 * @FilePath: \protoc-go-inject-tag\swagger\proto_parser_test.go
 * @Description:
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProtoFile(t *testing.T) {
	t.Run("basic inject_tag", func(t *testing.T) {
		content := `syntax = "proto3";

message LoginRequest {
  string username = 1;  // 用户名 @inject_tag: validate:"required,min=3,max=50"
  string password = 2;  // 密码 @inject_tag: validate:"required"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 2)

		assert.Equal(t, "LoginRequest", tags[0].MessageName)
		assert.Equal(t, "username", tags[0].FieldName)
		assert.Equal(t, `required,min=3,max=50`, tags[0].Tags.Validate)
		assert.Equal(t, `validate:"required,min=3,max=50"`, tags[0].Tags.GoTags)

		assert.Equal(t, "LoginRequest", tags[1].MessageName)
		assert.Equal(t, "password", tags[1].FieldName)
		assert.Equal(t, `required`, tags[1].Tags.Validate)
	})

	t.Run("gotags annotation", func(t *testing.T) {
		content := `syntax = "proto3";

message User {
  string name = 1;  // 名称 @gotags: json:"name" validate:"required"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "User", tags[0].MessageName)
		assert.Equal(t, "name", tags[0].FieldName)
		assert.Equal(t, `required`, tags[0].Tags.Validate)
	})

	t.Run("inject_tags annotation", func(t *testing.T) {
		content := `syntax = "proto3";

message Address {
  string province = 1;  // 省份 @inject_tags: json:"province" validate:"required"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "province", tags[0].FieldName)
		assert.Equal(t, `required`, tags[0].Tags.Validate)
	})

	t.Run("multiple messages", func(t *testing.T) {
		content := `syntax = "proto3";

message LoginRequest {
  string username = 1;  // 用户名 @inject_tag: validate:"required"
}

message RegisterRequest {
  string email = 1;  // 邮箱 @inject_tag: validate:"required,email"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 2)
		assert.Equal(t, "LoginRequest", tags[0].MessageName)
		assert.Equal(t, "username", tags[0].FieldName)
		assert.Equal(t, "RegisterRequest", tags[1].MessageName)
		assert.Equal(t, "email", tags[1].FieldName)
	})

	t.Run("repeated field", func(t *testing.T) {
		content := `syntax = "proto3";

message ListRequest {
  repeated string ids = 1;  // ID列表 @inject_tag: validate:"required,dive"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "ids", tags[0].FieldName)
	})

	t.Run("optional field", func(t *testing.T) {
		content := `syntax = "proto3";

message OptRequest {
  optional string note = 1;  // 备注 @inject_tag: validate:"omitempty,max=200"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "note", tags[0].FieldName)
	})

	t.Run("skip comment-only lines", func(t *testing.T) {
		content := `syntax = "proto3";
// 这是纯注释行 @inject_tag: validate:"required"

message Empty {
  int32 id = 1;
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		assert.Len(t, tags, 0)
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempProto(t, "syntax = \"proto3\";\n")
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		assert.Len(t, tags, 0)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ParseProtoFile(filepath.Join(os.TempDir(), "nonexistent.proto"))
		assert.Error(t, err)
	})

	t.Run("no tag annotation", func(t *testing.T) {
		content := `syntax = "proto3";

message NoTag {
  string name = 1;  // 普通注释，没有tag
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		assert.Len(t, tags, 0)
	})

	t.Run("multiple tags on same line", func(t *testing.T) {
		content := `syntax = "proto3";

message MultiTag {
  string province = 1;  // 省份 @inject_tags: json:"province" validate:"required,min=2,max=50" @gotags: gorm:"type:varchar(50)"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "province", tags[0].FieldName)
	})

	t.Run("message with no fields", func(t *testing.T) {
		content := `syntax = "proto3";

message Empty {}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		assert.Len(t, tags, 0)
	})

	t.Run("field without equals", func(t *testing.T) {
		content := `syntax = "proto3";

message BadSyntax {
  reserved 1 to 3;
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		assert.Len(t, tags, 0)
	})

	t.Run("close brace resets message", func(t *testing.T) {
		content := `syntax = "proto3";

message Msg1 {
  string a = 1;  // @inject_tag: validate:"required"
}

message Msg2 {
  string b = 1;  // @inject_tag: validate:"required"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 2)
		assert.Equal(t, "Msg1", tags[0].MessageName)
		assert.Equal(t, "Msg2", tags[1].MessageName)
	})

	t.Run("field with no validate", func(t *testing.T) {
		content := `syntax = "proto3";

message NoValidate {
  string name = 1;  // @inject_tag: json:"name" gorm:"type:varchar(50)"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "", tags[0].Tags.Validate)
	})

	t.Run("gotags without validate", func(t *testing.T) {
		content := `syntax = "proto3";

message GoTagsOnly {
  string name = 1;  // @gotags: json:"name"
}
`
		path := writeTempProto(t, content)
		tags, err := ParseProtoFile(path)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		assert.Equal(t, "", tags[0].Tags.Validate)
	})
}

func TestExtractFieldName(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple field", "string code = 2;", "code"},
		{"with comment", "string code = 2;  // 验证码 @inject_tag: validate:\"required\"", "code"},
		{"repeated field", "repeated string items = 1;", "items"},
		{"optional field", "optional int32 count = 3;", "count"},
		{"comment line", "// string skip = 1;", ""},
		{"empty line", "", ""},
		{"message line", "message Foo {", ""},
		{"complex type", "map<string, int32> meta = 5;", "meta"},
		{"message with trailing comment", "string code = 2; // some comment", "code"},
		{"with spaces", "  string  name  =  1;  ", "name"},
		{"no equals sign", "reserved 1 to 3;", ""},
		{"only equals", "=", ""},
		{"one field before equals", "name =", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := extractFieldName(c.input)
			assert.Equal(t, c.expected, result)
		})
	}
}

func TestParseGoTagString(t *testing.T) {
	t.Run("single tag", func(t *testing.T) {
		result := parseGoTagString(`json:"name"`)
		assert.Equal(t, "name", result["json"])
	})

	t.Run("multiple tags", func(t *testing.T) {
		result := parseGoTagString(`json:"province" validate:"required,min=2,max=50"`)
		assert.Equal(t, "province", result["json"])
		assert.Equal(t, "required,min=2,max=50", result["validate"])
	})

	t.Run("unquoted value", func(t *testing.T) {
		result := parseGoTagString(`validate:"required"`)
		assert.Equal(t, "required", result["validate"])
	})

	t.Run("empty string", func(t *testing.T) {
		result := parseGoTagString("")
		assert.Empty(t, result)
	})

	t.Run("unclosed quote", func(t *testing.T) {
		result := parseGoTagString(`json:"name`)
		assert.Equal(t, "name", result["json"])
	})

	t.Run("unquoted value no space", func(t *testing.T) {
		result := parseGoTagString(`foo:bar`)
		assert.Equal(t, "bar", result["foo"])
	})

	t.Run("unquoted value to space", func(t *testing.T) {
		result := parseGoTagString(`foo:bar next:baz`)
		assert.Equal(t, "bar", result["foo"])
		assert.Equal(t, "baz", result["next"])
	})

	t.Run("only space", func(t *testing.T) {
		result := parseGoTagString("   ")
		assert.Empty(t, result)
	})

	t.Run("colon before space without quote", func(t *testing.T) {
		result := parseGoTagString(`gorm:type:varchar validate:"required"`)
		assert.Equal(t, `type:varchar`, result["gorm"])
		assert.Equal(t, "required", result["validate"])
	})
}

func TestExtractTagValue(t *testing.T) {
	t.Run("value until next tag", func(t *testing.T) {
		val := extractTagValue(`json:"province" validate:"required" @gotags: gorm:"type:varchar(50)"`)
		assert.Equal(t, `json:"province" validate:"required"`, val)
	})

	t.Run("value to end of line", func(t *testing.T) {
		val := extractTagValue(`json:"name" validate:"required"`)
		assert.Equal(t, `json:"name" validate:"required"`, val)
	})

	t.Run("empty string", func(t *testing.T) {
		val := extractTagValue("")
		assert.Equal(t, "", val)
	})

	t.Run("whitespace only", func(t *testing.T) {
		val := extractTagValue("   ")
		assert.Equal(t, "", val)
	})
}

func writeTempProto(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.proto")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
