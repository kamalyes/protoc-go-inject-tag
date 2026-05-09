/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:28:57
 * @FilePath: \protoc-go-inject-tag\swagger\processor_test.go
 * @Description: Swagger处理器单元测试，覆盖所有分支
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func ptrInt64(v int64) *int64       { return &v }
func ptrFloat64(v float64) *float64 { return &v }

func newProps(pairs ...interface{}) *SchemaProperties {
	p := &SchemaProperties{}
	for i := 0; i+1 < len(pairs); i += 2 {
		*p = append(*p, schemaPropertyEntry{Key: pairs[i].(string), Value: pairs[i+1].(*swaggerProperty)})
	}
	return p
}

func newPaths(entries ...interface{}) *swaggerPathsObject {
	po := &swaggerPathsObject{}
	for i := 0; i+1 < len(entries); i += 2 {
		path := entries[i].(string)
		item := entries[i+1].(*swaggerPathItemObject)
		*po = append(*po, pathData{Path: path, PathItemObject: item})
	}
	return po
}

func pathItem(ops map[string]*swaggerOperation) *swaggerPathItemObject {
	item := &swaggerPathItemObject{}
	if op, ok := ops["get"]; ok {
		item.Get = op
	}
	if op, ok := ops["post"]; ok {
		item.Post = op
	}
	if op, ok := ops["put"]; ok {
		item.Put = op
	}
	if op, ok := ops["delete"]; ok {
		item.Delete = op
	}
	if op, ok := ops["patch"]; ok {
		item.Patch = op
	}
	return item
}

func writeSwaggerYAML(t *testing.T, doc swaggerDoc) string {
	t.Helper()
	data, err := yaml.Marshal(doc)
	require.NoError(t, err)
	dir := t.TempDir()
	path := filepath.Join(dir, "test.swagger.yaml")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func writeSwaggerJSON(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.swagger.json")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func TestNewProcessor(t *testing.T) {
	p := NewProcessor(true)
	assert.True(t, p.verbose)
	p2 := NewProcessor(false)
	assert.False(t, p2.verbose)
}

func TestProcessor_ProcessFile_YAML(t *testing.T) {
	protoContent := `syntax = "proto3";

message LoginRequest {
  string username = 1;  // 用户名 @inject_tag: validate:"required,min=3,max=50"
  string password = 2;  // 密码 @inject_tag: validate:"required"
  string code = 3;      // 验证码 | [EN] Verification code @inject_tag: validate:"required"
}
`

	t.Run("inject constraints and strip tags", func(t *testing.T) {
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"LoginRequest": {
					Type: "object",
					Properties: newProps(
						"username", &swaggerProperty{Type: "string", Description: `用户名 @inject_tag: validate:"required,min=3,max=50"`},
						"password", &swaggerProperty{Type: "string", Description: "密码"},
						"code", &swaggerProperty{Type: "string", Description: `验证码 | [EN] Verification code @inject_tag: validate:"required"`},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		require.NotNil(t, doc.Definitions)
		require.NotNil(t, doc.Definitions["LoginRequest"])
		require.NotNil(t, doc.Definitions["LoginRequest"].Properties)

		userProp := doc.Definitions["LoginRequest"].Properties.Get("username")
		require.NotNil(t, userProp)
		assert.Equal(t, "用户名", userProp.Description)
		assert.NotNil(t, userProp.Minimum)
		assert.Equal(t, float64(3), *userProp.Minimum)
		assert.NotNil(t, userProp.Maximum)
		assert.Equal(t, float64(50), *userProp.Maximum)

		passProp := doc.Definitions["LoginRequest"].Properties.Get("password")
		require.NotNil(t, passProp)
		assert.Equal(t, "密码", passProp.Description)

		codeProp := doc.Definitions["LoginRequest"].Properties.Get("code")
		require.NotNil(t, codeProp)
		assert.Equal(t, "验证码 | [EN] Verification code", codeProp.Description)

		req := doc.Definitions["LoginRequest"].Required
		assert.Contains(t, req, "username")
		assert.Contains(t, req, "password")
		assert.Contains(t, req, "code")
	})

	t.Run("verbose mode with injection", func(t *testing.T) {
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"LoginRequest": {
					Type: "object",
					Properties: newProps(
						"username", &swaggerProperty{Type: "string", Description: "用户名"},
					),
				},
			},
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_ProcessFile_JSON(t *testing.T) {
	t.Run("inject constraints in JSON swagger", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message RegisterRequest {
  string email = 1;  // 邮箱 @inject_tag: validate:"required,email"
  string phone = 2;  // 手机号 @inject_tag: validate:"required,numeric,len=11"
}
`
		swagger := map[string]interface{}{
			"definitions": map[string]interface{}{
				"RegisterRequest": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"email": map[string]interface{}{
							"type":        "string",
							"description": "邮箱",
						},
						"phone": map[string]interface{}{
							"type":        "string",
							"description": "手机号",
						},
					},
				},
			},
		}
		swaggerBytes, err := json.Marshal(swagger)
		require.NoError(t, err)

		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerJSON(t, swaggerBytes)

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &result))

		defs := result["definitions"].(map[string]interface{})
		reqDef := defs["RegisterRequest"].(map[string]interface{})
		props := reqDef["properties"].(map[string]interface{})

		emailProp := props["email"].(map[string]interface{})
		assert.Equal(t, "email", emailProp["format"])
		assert.Equal(t, "邮箱", emailProp["description"])

		phoneProp := props["phone"].(map[string]interface{})
		assert.Equal(t, "^[0-9]+$", phoneProp["pattern"])
		assert.Equal(t, "手机号", phoneProp["description"])

		reqArr := reqDef["required"].([]interface{})
		assert.Contains(t, reqArr, "email")
		assert.Contains(t, reqArr, "phone")
	})
}

func TestProcessor_ProcessFile_StripOnly(t *testing.T) {
	t.Run("strip inject_tag from unmapped definitions", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"SomeType": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: `名称 @inject_tag: validate:"required"`},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "@inject_tag")
		assert.Contains(t, string(data), "名称")
	})

	t.Run("strip with verbose and nil property", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		yamlContent := `definitions:
  SomeMsg:
    type: object
    properties:
      field1:
        type: string
        description: "test @inject_tag: validate:\"required\""
`
		require.NoError(t, os.WriteFile(swaggerPath, []byte(yamlContent), 0644))

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "@inject_tag")
	})
}

func TestProcessor_ProcessFile_NoModification(t *testing.T) {
	t.Run("no changes needed", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions:
  NoTag:
    type: object
    properties:
      id:
        type: string
        description: "ID"
`), 0644))

		info, _ := os.Stat(swaggerPath)
		origMod := info.ModTime()

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		info2, _ := os.Stat(swaggerPath)
		assert.Equal(t, origMod, info2.ModTime())
	})

	t.Run("no changes with verbose", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions:
  NoTag:
    type: object
    properties:
      id:
        type: string
        description: "ID"
`), 0644))

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_ProcessFile_Errors(t *testing.T) {
	t.Run("swagger file not found", func(t *testing.T) {
		proc := NewProcessor(false)
		err := proc.ProcessFile("/nonexistent/file.yaml", []string{})
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "bad.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`{{{invalid yaml`), 0644))

		proc := NewProcessor(false)
		err := proc.ProcessFile(swaggerPath, []string{protoPath})
		assert.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "bad.swagger.json")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`{invalid json`), 0644))

		proc := NewProcessor(false)
		err := proc.ProcessFile(swaggerPath, []string{protoPath})
		assert.Error(t, err)
	})

	t.Run("proto file not found", func(t *testing.T) {
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions: {}`), 0644))

		proc := NewProcessor(false)
		err := proc.ProcessFile(swaggerPath, []string{"/nonexistent.proto"})
		assert.Error(t, err)
	})
}

func TestProcessor_DuplicateRequired(t *testing.T) {
	t.Run("do not duplicate required entries", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Dup {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Dup": {
					Type:     "object",
					Required: []string{"name"},
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: "名称"},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Equal(t, []string{"name"}, doc.Definitions["Dup"].Required)
	})
}

func TestProcessor_NilSchemaAndProperty(t *testing.T) {
	t.Run("skip nil schema", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions:
  NilSchema:
    type: object
`), 0644))

		proc := NewProcessor(false)
		assert.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})

	t.Run("skip nil property", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yaml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions:
  TestMsg:
    type: object
    properties:
      normal:
        type: string
        description: "normal field"
`), 0644))

		proc := NewProcessor(false)
		assert.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_ExistingFormatNotOverwritten(t *testing.T) {
	t.Run("keep existing format", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Email {
  string addr = 1;  // 邮箱 @inject_tag: validate:"required,email"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Email": {
					Type: "object",
					Properties: newProps(
						"addr", &swaggerProperty{Type: "string", Format: "hostname"},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Equal(t, "hostname", doc.Definitions["Email"].Properties.Get("addr").Format)
	})
}

func TestProcessor_YML(t *testing.T) {
	t.Run("process .yml extension", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Yml {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.yml")
		require.NoError(t, os.WriteFile(swaggerPath, []byte(`definitions:
  Yml:
    type: object
    properties:
      name:
        type: string
        description: "名称"
`), 0644))

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Contains(t, doc.Definitions["Yml"].Required, "name")
	})
}

func TestProcessor_ApplyConstraints(t *testing.T) {
	t.Run("all constraints", func(t *testing.T) {
		proc := NewProcessor(false)
		prop := &swaggerProperty{}
		c := &SwaggerConstraints{
			Required:         true,
			MinLength:        ptrInt64(2),
			MaxLength:        ptrInt64(100),
			Min:              ptrFloat64(0),
			Max:              ptrFloat64(999),
			ExclusiveMinimum: true,
			ExclusiveMaximum: true,
			Pattern:          "^[a-z]+$",
			Format:           "email",
			Enum:             []string{"A", "B", "C"},
			MinItems:         ptrInt64(1),
			MaxItems:         ptrInt64(10),
		}
		proc.applyConstraints(prop, c)

		assert.NotNil(t, prop.MinLength)
		assert.Equal(t, int64(2), *prop.MinLength)
		assert.NotNil(t, prop.MaxLength)
		assert.Equal(t, int64(100), *prop.MaxLength)
		assert.NotNil(t, prop.Minimum)
		assert.Equal(t, float64(0), *prop.Minimum)
		assert.True(t, prop.ExclusiveMinimum)
		assert.NotNil(t, prop.Maximum)
		assert.Equal(t, float64(999), *prop.Maximum)
		assert.True(t, prop.ExclusiveMaximum)
		assert.Equal(t, "^[a-z]+$", prop.Pattern)
		assert.Equal(t, "email", prop.Format)
		enumSlice, ok := prop.Enum.([]interface{})
		require.True(t, ok)
		assert.Len(t, enumSlice, 3)
		assert.Equal(t, "A", enumSlice[0])
		assert.NotNil(t, prop.MinItems)
		assert.Equal(t, int64(1), *prop.MinItems)
		assert.NotNil(t, prop.MaxItems)
		assert.Equal(t, int64(10), *prop.MaxItems)
	})

	t.Run("format not overwritten when already set", func(t *testing.T) {
		proc := NewProcessor(false)
		prop := &swaggerProperty{Format: "uri"}
		c := &SwaggerConstraints{Format: "email"}
		proc.applyConstraints(prop, c)
		assert.Equal(t, "uri", prop.Format)
	})

	t.Run("empty constraints no change", func(t *testing.T) {
		proc := NewProcessor(false)
		prop := &swaggerProperty{Type: "string"}
		c := &SwaggerConstraints{}
		proc.applyConstraints(prop, c)
		assert.Equal(t, "string", prop.Type)
		assert.Nil(t, prop.MinLength)
		assert.Nil(t, prop.MaxLength)
		assert.Nil(t, prop.Minimum)
		assert.Nil(t, prop.Maximum)
		assert.Empty(t, prop.Pattern)
		assert.Empty(t, prop.Format)
		assert.Nil(t, prop.Enum)
	})
}

func TestProcessor_CleanupUnreferencedTags(t *testing.T) {
	t.Run("remove unreferenced tags", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Test {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{
				{Name: "Used", Description: "used tag"},
				{Name: "Orphan", Description: "not used"},
				{Name: "AlsoOrphan", Description: "not used either"},
			},
			Paths: newPaths(
				"/api/test", pathItem(map[string]*swaggerOperation{
					"get": {Summary: "test", Tags: []string{"Used"}},
				}),
			),
			Definitions: map[string]*swaggerSchema{
				"Test": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: "名称"},
					),
				},
			},
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		require.Len(t, doc.Tags, 1)
		assert.Equal(t, "Used", doc.Tags[0].Name)
	})

	t.Run("all tags referenced", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{
				{Name: "A"},
				{Name: "B"},
			},
			Paths: newPaths(
				"/a", pathItem(map[string]*swaggerOperation{"get": {Tags: []string{"A"}}}),
				"/b", pathItem(map[string]*swaggerOperation{"get": {Tags: []string{"B"}}}),
			),
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Len(t, doc.Tags, 2)
	})

	t.Run("no tags defined", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})

	t.Run("nil operation in path", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{{Name: "Orphan"}},
			Paths: newPaths(
				"/test", &swaggerPathItemObject{},
			),
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})

	t.Run("all tags orphaned", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{{Name: "A"}, {Name: "B"}},
			Paths: newPaths(
				"/test", pathItem(map[string]*swaggerOperation{"get": {Summary: "no tags"}}),
			),
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Empty(t, doc.Tags)
	})
}

func TestProcessor_ExampleAddressScenario(t *testing.T) {
	t.Run("Address with mixed validate rules", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Address {
  string province = 1;  // 省份 | [EN] Province // @inject_tags: json:"province" validate:"required,min=2,max=50"
  string city = 2;      // 城市 | [EN] City // @gotags: json:"city" validate:"required,min=2,max=50"
  string street = 3;    // 街道地址 | [EN] Street address // @gotags: json:"street" validate:"required,min=5,max=200"
  string postal_code = 4; // 邮政编码 | [EN] Postal code // @gotags: json:"postal_code" validate:"omitempty,len=6,numeric"
  bool is_default = 5;  // 是否默认地址 | [EN] Is default address // @gotags: json:"is_default"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Address": {
					Type: "object",
					Properties: newProps(
						"province", &swaggerProperty{Type: "string", Description: `省份 | [EN] Province @inject_tags: json:"province" validate:"required,min=2,max=50"`},
						"city", &swaggerProperty{Type: "string", Description: `城市 | [EN] City @gotags: json:"city" validate:"required,min=2,max=50"`},
						"street", &swaggerProperty{Type: "string", Description: `街道地址 | [EN] Street address @gotags: json:"street" validate:"required,min=5,max=200"`},
						"postal_code", &swaggerProperty{Type: "string", Description: `邮政编码 | [EN] Postal code @gotags: json:"postal_code" validate:"omitempty,len=6,numeric"`},
						"is_default", &swaggerProperty{Type: "boolean", Description: `是否默认地址 | [EN] Is default address @gotags: json:"is_default"`},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		addr := doc.Definitions["Address"]
		require.NotNil(t, addr)

		prov := addr.Properties.Get("province")
		assert.Equal(t, "省份 | [EN] Province", prov.Description)
		assert.NotNil(t, prov.Minimum)
		assert.Equal(t, float64(2), *prov.Minimum)
		assert.NotNil(t, prov.Maximum)
		assert.Equal(t, float64(50), *prov.Maximum)

		postal := addr.Properties.Get("postal_code")
		assert.Equal(t, "邮政编码 | [EN] Postal code", postal.Description)
		assert.NotNil(t, postal.MinLength)
		assert.Equal(t, int64(6), *postal.MinLength)
		assert.NotNil(t, postal.MaxLength)
		assert.Equal(t, int64(6), *postal.MaxLength)
		assert.Equal(t, "^[0-9]+$", postal.Pattern)

		isDef := addr.Properties.Get("is_default")
		assert.Equal(t, "是否默认地址 | [EN] Is default address", isDef.Description)

		assert.Contains(t, addr.Required, "province")
		assert.Contains(t, addr.Required, "city")
		assert.Contains(t, addr.Required, "street")
		assert.NotContains(t, addr.Required, "postal_code")
		assert.NotContains(t, addr.Required, "is_default")
	})
}

func TestProcessor_UserScenario(t *testing.T) {
	t.Run("User with uuid email url alphanum oneof", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message User {
  string id = 1;        // 用户ID @inject_tag: validate:"required,uuid"
  string username = 2;  // 用户名 @inject_tag: validate:"required,min=3,max=50,alphanum"
  string email = 3;     // 邮箱 @inject_tag: validate:"required,email"
  string avatar_url = 4; // 头像URL @inject_tag: validate:"omitempty,url"
  string status = 5;    // 状态 @inject_tag: validate:"required,oneof=active inactive suspended"
  string bio = 6;       // 简介 @inject_tag: validate:"omitempty,max=500"
  string phone = 7;     // 手机号 @inject_tag: validate:"omitempty,len=11,numeric"
  int32 age = 8;        // 年龄 @inject_tag: validate:"omitempty,gte=0,lte=150"
  int32 weight = 9;     // 权重 @inject_tag: validate:"gt=0,lt=100"
  string code = 10;     // 代码 @inject_tag: validate:"alpha"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"User": {
					Type: "object",
					Properties: newProps(
						"id", &swaggerProperty{Type: "string", Description: "用户ID"},
						"username", &swaggerProperty{Type: "string", Description: "用户名"},
						"email", &swaggerProperty{Type: "string", Description: "邮箱"},
						"avatar_url", &swaggerProperty{Type: "string", Description: "头像URL"},
						"status", &swaggerProperty{Type: "string", Description: "状态"},
						"bio", &swaggerProperty{Type: "string", Description: "简介"},
						"phone", &swaggerProperty{Type: "string", Description: "手机号"},
						"age", &swaggerProperty{Type: "integer", Description: "年龄"},
						"weight", &swaggerProperty{Type: "integer", Description: "权重"},
						"code", &swaggerProperty{Type: "string", Description: "代码"},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		user := doc.Definitions["User"]
		require.NotNil(t, user)

		assert.Equal(t, "uuid", user.Properties.Get("id").Format)
		assert.Equal(t, "email", user.Properties.Get("email").Format)
		assert.Equal(t, "uri", user.Properties.Get("avatar_url").Format)
		assert.Equal(t, "^[a-zA-Z0-9]+$", user.Properties.Get("username").Pattern)
		assert.Equal(t, "^[0-9]+$", user.Properties.Get("phone").Pattern)
		assert.Equal(t, "^[a-zA-Z]+$", user.Properties.Get("code").Pattern)
		assert.NotNil(t, user.Properties.Get("age").Minimum)
		assert.Equal(t, float64(0), *user.Properties.Get("age").Minimum)
		assert.False(t, user.Properties.Get("age").ExclusiveMinimum)
		assert.NotNil(t, user.Properties.Get("age").Maximum)
		assert.Equal(t, float64(150), *user.Properties.Get("age").Maximum)
		assert.False(t, user.Properties.Get("age").ExclusiveMaximum)

		assert.NotNil(t, user.Properties.Get("weight").Minimum)
		assert.Equal(t, float64(0), *user.Properties.Get("weight").Minimum)
		assert.True(t, user.Properties.Get("weight").ExclusiveMinimum)
		assert.NotNil(t, user.Properties.Get("weight").Maximum)
		assert.Equal(t, float64(100), *user.Properties.Get("weight").Maximum)
		assert.True(t, user.Properties.Get("weight").ExclusiveMaximum)

		enumVals, ok := user.Properties.Get("status").Enum.([]interface{})
		require.True(t, ok)
		require.Len(t, enumVals, 3)
		assert.Equal(t, "active", enumVals[0])

		assert.Contains(t, user.Required, "id")
		assert.Contains(t, user.Required, "username")
		assert.Contains(t, user.Required, "email")
		assert.Contains(t, user.Required, "status")
	})
}

func TestProcessor_MixedTags(t *testing.T) {
	t.Run("tag cleanup with constraints injection", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Req {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{
				{Name: "Used"},
				{Name: "Unused", Description: "should be removed"},
			},
			Paths: newPaths(
				"/test", pathItem(map[string]*swaggerOperation{
					"post": {Summary: "test", Tags: []string{"Used"}},
				}),
			),
			Definitions: map[string]*swaggerSchema{
				"Req": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: "名称"},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Len(t, doc.Tags, 1)
		assert.Equal(t, "Used", doc.Tags[0].Name)
		assert.Contains(t, doc.Definitions["Req"].Required, "name")
	})
}

func TestProcessor_VerboseWithTags(t *testing.T) {
	t.Run("verbose mode for tag cleanup", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Tags: []swaggerTag{
				{Name: "Used"},
				{Name: "Orphan"},
			},
			Paths: newPaths(
				"/test", pathItem(map[string]*swaggerOperation{
					"get": {Tags: []string{"Used"}},
				}),
			),
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_JSONExtension(t *testing.T) {
	t.Run("process .json swagger file", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message JsonMsg {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		dir := t.TempDir()
		swaggerPath := filepath.Join(dir, "test.swagger.json")
		jsonContent := `{
  "definitions": {
    "JsonMsg": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "description": "名称"
        }
      }
    }
  }
}`
		require.NoError(t, os.WriteFile(swaggerPath, []byte(jsonContent), 0644))

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, json.Unmarshal(data, &doc))
		assert.Contains(t, doc.Definitions["JsonMsg"].Required, "name")
	})
}

func TestProcessor_VerboseStrip(t *testing.T) {
	t.Run("verbose strip inject_tag from unmatched schema", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Unmatched": {
					Type: "object",
					Properties: newProps(
						"field", &swaggerProperty{Type: "string", Description: `描述 @inject_tag: validate:"required"`},
					),
				},
			},
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_NoModifyVerbose(t *testing.T) {
	t.Run("verbose mode with no modification needed", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Simple": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: "name"},
					),
				},
			},
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))
	})
}

func TestProcessor_StripInjectTagVerbose(t *testing.T) {
	t.Run("strip with verbose output", func(t *testing.T) {
		protoPath := writeTempProto(t, `syntax = "proto3";`)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"StripVerbose": {
					Type: "object",
					Properties: newProps(
						"field1", &swaggerProperty{Type: "string", Description: "描述 @inject_tags: json:\"field1\" validate:\"required\""},
						"field2", &swaggerProperty{Type: "string", Description: "no inject tag"},
					),
				},
			},
		})

		proc := NewProcessor(true)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))
		assert.Equal(t, "描述", doc.Definitions["StripVerbose"].Properties.Get("field1").Description)
		assert.Equal(t, "no inject tag", doc.Definitions["StripVerbose"].Properties.Get("field2").Description)
	})
}

func TestProcessor_StripDoubleSlashPrefix(t *testing.T) {
	t.Run("strip // @inject_tag from title", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message TmpKey {
  string id = 1;  // 临时密钥ID | [EN] Temporary key ID // @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"TmpKey": {
					Type: "object",
					Properties: newProps(
						"id", &swaggerProperty{
							Type:        "string",
							Title:       `临时密钥ID | [EN] Temporary key ID  // @inject_tag: validate:"required"`,
							Description: `临时密钥ID | [EN] Temporary key ID  // @inject_tag: validate:"required"`,
						},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))

		idProp := doc.Definitions["TmpKey"].Properties.Get("id")
		require.NotNil(t, idProp)
		assert.Equal(t, "临时密钥ID | [EN] Temporary key ID", idProp.Title)
		assert.Equal(t, "临时密钥ID | [EN] Temporary key ID", idProp.Description)
		assert.Contains(t, doc.Definitions["TmpKey"].Required, "id")
	})

	t.Run("strip // @gotags from description", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Msg {
  string name = 1;  // 名称 // @gotags: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Msg": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: `名称 // @gotags: validate:"required"`},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))

		nameProp := doc.Definitions["Msg"].Properties.Get("name")
		require.NotNil(t, nameProp)
		assert.Equal(t, "名称", nameProp.Description)
	})

	t.Run("strip without // prefix still works", func(t *testing.T) {
		protoContent := `syntax = "proto3";

message Msg2 {
  string name = 1;  // 名称 @inject_tag: validate:"required"
}
`
		protoPath := writeTempProto(t, protoContent)
		swaggerPath := writeSwaggerYAML(t, swaggerDoc{
			Definitions: map[string]*swaggerSchema{
				"Msg2": {
					Type: "object",
					Properties: newProps(
						"name", &swaggerProperty{Type: "string", Description: `名称 @inject_tag: validate:"required"`},
					),
				},
			},
		})

		proc := NewProcessor(false)
		require.NoError(t, proc.ProcessFile(swaggerPath, []string{protoPath}))

		data, err := os.ReadFile(swaggerPath)
		require.NoError(t, err)

		var doc swaggerDoc
		require.NoError(t, yaml.Unmarshal(data, &doc))

		nameProp := doc.Definitions["Msg2"].Properties.Get("name")
		require.NotNil(t, nameProp)
		assert.Equal(t, "名称", nameProp.Description)
	})
}
