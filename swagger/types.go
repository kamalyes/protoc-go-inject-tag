/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:29:26
 * @FilePath: \protoc-go-inject-tag\swagger\types.go
 * @Description: Swagger/OpenAPI v2 类型定义，参照 grpc-gateway protoc-gen-openapiv2 types.go 结构
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// swaggerInfo swagger文档信息
// http://swagger.io/specification/#infoObject
type swaggerInfo struct {
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

// swaggerTag swagger顶层标签定义
// http://swagger.io/specification/#tagObject
type swaggerTag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// swaggerDoc swagger文档顶层结构，字段顺序与protoc-gen-openapiv2 openapiSwaggerObject一致
// http://swagger.io/specification/#swaggerObject
type swaggerDoc struct {
	Swagger     string                    `json:"swagger" yaml:"swagger"`
	Info        *swaggerInfo              `json:"info,omitempty" yaml:"info,omitempty"`
	Tags        []swaggerTag              `json:"tags,omitempty" yaml:"tags,omitempty"`
	Host        string                    `json:"host,omitempty" yaml:"host,omitempty"`
	BasePath    string                    `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Schemes     []string                  `json:"schemes,omitempty" yaml:"schemes,omitempty"`
	Consumes    []string                  `json:"consumes,omitempty" yaml:"consumes,omitempty"`
	Produces    []string                  `json:"produces,omitempty" yaml:"produces,omitempty"`
	Paths       *swaggerPathsObject       `json:"paths,omitempty" yaml:"paths,omitempty"`
	Definitions map[string]*swaggerSchema `json:"definitions,omitempty" yaml:"definitions,omitempty"`
}

// pathData 路径条目，保持路径定义顺序
type pathData struct {
	Path           string
	PathItemObject *swaggerPathItemObject
}

// swaggerPathsObject 有序路径集合，参照grpc-gateway openapiPathsObject保持插入顺序
// http://swagger.io/specification/#pathsObject
type swaggerPathsObject []pathData

func (po swaggerPathsObject) MarshalYAML() (interface{}, error) {
	n := yaml.Node{
		Kind:    yaml.MappingNode,
		Content: make([]*yaml.Node, len(po)*2),
	}
	for i, pd := range po {
		keyNode := yaml.Node{}
		keyNode.SetString(pd.Path)
		valueNode := yaml.Node{}
		if err := valueNode.Encode(pd.PathItemObject); err != nil {
			return nil, err
		}
		n.Content[i*2+0] = &keyNode
		n.Content[i*2+1] = &valueNode
	}
	return n, nil
}

func (po *swaggerPathsObject) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("swaggerPathsObject: 期望MappingNode，得到 %d", value.Kind)
	}
	*po = make(swaggerPathsObject, 0, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		var path string
		if err := value.Content[i].Decode(&path); err != nil {
			return err
		}
		item := &swaggerPathItemObject{}
		if err := value.Content[i+1].Decode(item); err != nil {
			return err
		}
		*po = append(*po, pathData{Path: path, PathItemObject: item})
	}
	return nil
}

func (po swaggerPathsObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, pd := range po {
		if i != 0 {
			buf.WriteString(",")
		}
		key, err := json.Marshal(pd.Path)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		val, err := json.Marshal(pd.PathItemObject)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func (po *swaggerPathsObject) UnmarshalJSON(data []byte) error {
	var raw map[string]*swaggerPathItemObject
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*po = make(swaggerPathsObject, 0, len(raw))
	var order []string
	var orderData map[string]json.RawMessage
	if err := json.Unmarshal(data, &orderData); err == nil {
		for k := range orderData {
			order = append(order, k)
		}
	}
	for _, k := range order {
		*po = append(*po, pathData{Path: k, PathItemObject: raw[k]})
	}
	return nil
}

// GetPaths 获取所有路径（保持顺序）
func (po swaggerPathsObject) GetPaths() []string {
	paths := make([]string, len(po))
	for i, pd := range po {
		paths[i] = pd.Path
	}
	return paths
}

// GetPathItem 获取指定路径的PathItem
func (po swaggerPathsObject) GetPathItem(path string) *swaggerPathItemObject {
	for i := range po {
		if po[i].Path == path {
			return po[i].PathItemObject
		}
	}
	return nil
}

// swaggerPathItemObject 路径项对象，字段顺序与protoc-gen-openapiv2 openapiPathItemObject一致
// 使用结构体字段保持HTTP方法顺序（而非map的字母排序）
// http://swagger.io/specification/#pathItemObject
type swaggerPathItemObject struct {
	Get     *swaggerOperation `json:"get,omitempty" yaml:"get,omitempty"`
	Delete  *swaggerOperation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Post    *swaggerOperation `json:"post,omitempty" yaml:"post,omitempty"`
	Put     *swaggerOperation `json:"put,omitempty" yaml:"put,omitempty"`
	Patch   *swaggerOperation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Head    *swaggerOperation `json:"head,omitempty" yaml:"head,omitempty"`
	Options *swaggerOperation `json:"options,omitempty" yaml:"options,omitempty"`
}

// schemaPropertyEntry 有序属性条目，保持proto字段定义顺序
type schemaPropertyEntry struct {
	Key   string           `json:"key" yaml:"key"`
	Value *swaggerProperty `json:"value" yaml:"value"`
}

// SchemaProperties 有序属性集合，参照grpc-gateway openapiSchemaObjectProperties保持插入顺序
type SchemaProperties []schemaPropertyEntry

func (p SchemaProperties) MarshalYAML() (interface{}, error) {
	n := yaml.Node{
		Kind:    yaml.MappingNode,
		Content: make([]*yaml.Node, len(p)*2),
	}
	for i, v := range p {
		keyNode := yaml.Node{}
		if err := keyNode.Encode(v.Key); err != nil {
			return nil, err
		}
		valueNode := yaml.Node{}
		if err := valueNode.Encode(v.Value); err != nil {
			return nil, err
		}
		n.Content[i*2+0] = &keyNode
		n.Content[i*2+1] = &valueNode
	}
	return n, nil
}

func (p *SchemaProperties) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("SchemaProperties: 期望MappingNode，得到 %d", value.Kind)
	}
	*p = make(SchemaProperties, 0, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		var key string
		if err := value.Content[i].Decode(&key); err != nil {
			return err
		}
		val := &swaggerProperty{}
		if err := value.Content[i+1].Decode(val); err != nil {
			return err
		}
		*p = append(*p, schemaPropertyEntry{Key: key, Value: val})
	}
	return nil
}

func (p SchemaProperties) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, kv := range p {
		if i != 0 {
			buf.WriteString(",")
		}
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		val, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func (p *SchemaProperties) UnmarshalJSON(data []byte) error {
	var raw map[string]*swaggerProperty
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = make(SchemaProperties, 0, len(raw))
	var order []string
	var orderData map[string]json.RawMessage
	if err := json.Unmarshal(data, &orderData); err == nil {
		for k := range orderData {
			order = append(order, k)
		}
	}
	for _, k := range order {
		*p = append(*p, schemaPropertyEntry{Key: k, Value: raw[k]})
	}
	return nil
}

func (p SchemaProperties) Get(key string) *swaggerProperty {
	for i := range p {
		if p[i].Key == key {
			return p[i].Value
		}
	}
	return nil
}

func (p *SchemaProperties) Set(key string, value *swaggerProperty) {
	for i := range *p {
		if (*p)[i].Key == key {
			(*p)[i].Value = value
			return
		}
	}
	*p = append(*p, schemaPropertyEntry{Key: key, Value: value})
}

func (p SchemaProperties) Keys() []string {
	keys := make([]string, len(p))
	for i, e := range p {
		keys[i] = e.Key
	}
	return keys
}

func (p SchemaProperties) Len() int { return len(p) }

// swaggerSchema swagger定义模式，字段顺序与protoc-gen-openapiv2 openapiSchemaObject一致
// http://swagger.io/specification/#definitionsObject
type swaggerSchema struct {
	// schemaCore部分（type/format/$ref/items）
	Type                 string               `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string               `json:"format,omitempty" yaml:"format,omitempty"`
	Ref                  string               `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Items                *swaggerProperty     `json:"items,omitempty" yaml:"items,omitempty"`
	Enum                 interface{}          `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default              interface{}          `json:"default,omitempty" yaml:"default,omitempty"`
	Properties           *SchemaProperties    `json:"properties,omitempty" yaml:"properties,omitempty"`
	AdditionalProperties *swaggerProperty     `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	Description          string               `json:"description,omitempty" yaml:"description,omitempty"`
	Title                string               `json:"title,omitempty" yaml:"title,omitempty"`
	ExternalDocs         *swaggerExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	ReadOnly             bool                 `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	Required             []string             `json:"required,omitempty" yaml:"required,omitempty"`
	AllOf                []*swaggerProperty   `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

// swaggerProperty swagger属性定义，字段顺序与protoc-gen-openapiv2 openapiSchemaObject一致
// http://swagger.io/specification/#schemaObject
type swaggerProperty struct {
	// schemaCore部分
	Type                 string               `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string               `json:"format,omitempty" yaml:"format,omitempty"`
	Ref                  string               `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Items                *swaggerProperty     `json:"items,omitempty" yaml:"items,omitempty"`
	Enum                 interface{}          `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default              interface{}          `json:"default,omitempty" yaml:"default,omitempty"`
	Properties           *SchemaProperties    `json:"properties,omitempty" yaml:"properties,omitempty"`
	AdditionalProperties *swaggerProperty     `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	Description          string               `json:"description,omitempty" yaml:"description,omitempty"`
	Title                string               `json:"title,omitempty" yaml:"title,omitempty"`
	ExternalDocs         *swaggerExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	ReadOnly             bool                 `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	Minimum              *float64             `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum              *float64             `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinLength            *int64               `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength            *int64               `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern              string               `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Required             bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	AllOf                []*swaggerProperty   `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

// swaggerExternalDocs swagger外部文档引用
// http://swagger.io/specification/#externalDocumentationObject
type swaggerExternalDocs struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url" yaml:"url"`
}

// swaggerOperation swagger操作对象，字段顺序与protoc-gen-openapiv2 openapiOperationObject一致
// http://swagger.io/specification/#operationObject
type swaggerOperation struct {
	Summary     string                      `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string                      `json:"description,omitempty" yaml:"description,omitempty"`
	OperationId string                      `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Responses   map[string]*swaggerResponse `json:"responses" yaml:"responses"`
	Parameters  []*swaggerParameter         `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Tags        []string                    `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// swaggerParameter swagger参数，字段顺序与protoc-gen-openapiv2 openapiParameterObject一致
// http://swagger.io/specification/#parameterObject
type swaggerParameter struct {
	Name        string           `json:"name" yaml:"name"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	In          string           `json:"in,omitempty" yaml:"in,omitempty"`
	Required    bool             `json:"required" yaml:"required"`
	Type        string           `json:"type,omitempty" yaml:"type,omitempty"`
	Format      string           `json:"format,omitempty" yaml:"format,omitempty"`
	Items       *swaggerProperty `json:"items,omitempty" yaml:"items,omitempty"`
	Enum        interface{}      `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     interface{}      `json:"default,omitempty" yaml:"default,omitempty"`
	Pattern     string           `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Schema      *swaggerProperty `json:"schema,omitempty" yaml:"schema,omitempty"`
}

// swaggerResponse swagger响应对象
// http://swagger.io/specification/#responseObject
type swaggerResponse struct {
	Description string           `json:"description" yaml:"description"`
	Schema      *swaggerProperty `json:"schema,omitempty" yaml:"schema,omitempty"`
}
