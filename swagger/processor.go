/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:28:57
 * @FilePath: \protoc-go-inject-tag\swagger\processor.go
 * @Description: Swagger文档后处理器，将proto @inject_tag约束注入swagger schema并剥离注释
 *
 * 处理流程：
 * 1. 解析proto文件中的@inject_tag/@gotags注解，提取validate标签
 * 2. 读取swagger YAML/JSON文件（由 format.go 提供）
 * 3. 遍历definitions，将validate约束转换为swagger schema约束（required/minLength/maxLength/minimum/maximum/pattern/enum/format）
 * 4. 从description/title中剥离@inject_tag/@gotags文本
 * 5. 写回修改后的swagger文件
 *
 * 结构体定义见 types.go，序列化辅助见 format.go
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"fmt"
	"regexp"
	"strings"
)

// injectTagInDescRegex 匹配description/title中的@inject_tag文本，用于从swagger中剥离
// 支持匹配前面的可选 "//" 注释前缀（来自proto注释经swagger生成后残留）
var injectTagInDescRegex = regexp.MustCompile(`\s*(?://\s*)?@(?:inject_tag|gotags?|inject_tags?):\s*[^\n]+`)

// Processor swagger后处理器
type Processor struct {
	verbose bool
}

// swaggerTagIndex swagger标签索引，用于快速查找字段标签
type swaggerTagIndex struct {
	schemas map[string]map[string]FieldTag
	fields  map[string]*fieldTagBucket
}

// fieldTagBucket 字段标签桶，用于存储字段标签和validate约束
// 支持冲突检测（如多个字段有相同validate约束）
type fieldTagBucket struct {
	tag      FieldTag
	validate string
	conflict bool
}

// NewProcessor 创建swagger后处理器
func NewProcessor(verbose bool) *Processor {
	return &Processor{verbose: verbose}
}

// ProcessFile 处理单个swagger文件：注入约束 + 剥离@inject_tag注释
func (p *Processor) ProcessFile(swaggerFile string, protoFiles []string) error {
	if p.verbose {
		fmt.Printf("📖 解析 proto 文件的 @inject_tag 注解...\n")
	}

	var protoTags []ProtoFieldTags
	for _, protoFile := range protoFiles {
		tags, err := ParseProtoFile(protoFile)
		if err != nil {
			return fmt.Errorf("解析 proto 文件失败 %s: %w", protoFile, err)
		}
		for _, t := range tags {
			if p.verbose {
				fmt.Printf("  发现 %s.%s → validate:%q\n", t.MessageName, t.FieldName, t.Tags.Validate)
			}
		}
		protoTags = append(protoTags, tags...)
	}
	tagIndex := newSwaggerTagIndex(protoTags)

	if p.verbose {
		fmt.Printf("📖 读取 swagger 文件: %s\n", swaggerFile)
	}

	doc, format, err := ReadSwaggerFile(swaggerFile)
	if err != nil {
		return err
	}

	modified := false

	for defName, schema := range doc.Definitions {
		if schema == nil || schema.Properties == nil || schema.Properties.Len() == 0 {
			continue
		}

		msgTags, _ := tagIndex.lookupSchema(defName)

		for _, entry := range *schema.Properties {
			propName := entry.Key
			prop := entry.Value
			if prop == nil {
				continue
			}

			validateValue := extractValidateFromSwaggerText(prop.Description)
			if validateValue == "" {
				validateValue = extractValidateFromSwaggerText(prop.Title)
			}
			if validateValue == "" && msgTags != nil {
				if ft, found := msgTags[propName]; found {
					validateValue = ft.Validate
				}
			}

			if stripped, changed := stripInjectTagText(prop.Description); changed {
				prop.Description = stripped
				modified = true
			}
			if stripped, changed := stripInjectTagText(prop.Title); changed {
				prop.Title = stripped
				modified = true
			}

			if validateValue != "" {
				constraints := ParseValidateToSwagger(validateValue)
				if constraints != nil {
					p.applyConstraints(prop, constraints)
					if constraints.Required {
						schema.Required = appendRequired(schema.Required, propName)
					}
					modified = true
					if p.verbose {
						fmt.Printf("  ✅ %s.%s → 已注入约束\n", defName, propName)
					}
				}
			}
		}
	}

	if p.processSwaggerParameters(doc, tagIndex) {
		modified = true
	}

	if p.cleanupUnreferencedTags(doc) {
		modified = true
	}

	if !modified {
		if p.verbose {
			fmt.Printf("  swagger 文件无需修改\n")
		}
		return nil
	}

	if err := WriteSwaggerFile(swaggerFile, doc, format); err != nil {
		return err
	}

	if p.verbose {
		fmt.Printf("  ✅ 已更新 swagger 文件: %s\n", swaggerFile)
	}

	return nil
}

func newSwaggerTagIndex(tags []ProtoFieldTags) *swaggerTagIndex {
	index := &swaggerTagIndex{
		schemas: make(map[string]map[string]FieldTag),
		fields:  make(map[string]*fieldTagBucket),
	}
	for _, tag := range tags {
		if tag.Tags.Validate == "" {
			continue
		}
		for _, schemaName := range openAPISchemaNameCandidates(tag.PackageName, tag.MessageName) {
			if index.schemas[schemaName] == nil {
				index.schemas[schemaName] = make(map[string]FieldTag)
			}
			for _, fieldName := range fieldNameCandidates(tag) {
				index.schemas[schemaName][fieldName] = tag.Tags
			}
		}
		for _, fieldName := range fieldNameCandidates(tag) {
			index.addField(fieldName, tag.Tags)
		}
	}
	return index
}

func (i *swaggerTagIndex) addField(fieldName string, tag FieldTag) {
	if fieldName == "" || tag.Validate == "" {
		return
	}
	if bucket, ok := i.fields[fieldName]; ok {
		if bucket.validate != tag.Validate {
			bucket.conflict = true
		}
		return
	}
	i.fields[fieldName] = &fieldTagBucket{tag: tag, validate: tag.Validate}
}

func (i *swaggerTagIndex) lookupSchema(defName string) (map[string]FieldTag, bool) {
	if i == nil {
		return nil, false
	}
	if tags, ok := i.schemas[defName]; ok {
		return tags, true
	}

	var bestTags map[string]FieldTag
	bestLen := 0
	for schemaName, tags := range i.schemas {
		if len(schemaName) <= bestLen {
			continue
		}
		if strings.HasSuffix(defName, schemaName) {
			bestTags = tags
			bestLen = len(schemaName)
		}
	}
	if bestTags != nil {
		return bestTags, true
	}
	return nil, false
}

func (i *swaggerTagIndex) lookupField(fieldName string) (FieldTag, bool) {
	if i == nil {
		return FieldTag{}, false
	}
	for _, name := range parameterNameCandidates(fieldName) {
		if bucket, ok := i.fields[name]; ok && !bucket.conflict {
			return bucket.tag, true
		}
	}
	return FieldTag{}, false
}

func parameterNameCandidates(name string) []string {
	return uniqueStrings([]string{name, jsonCamelCase(name)})
}

// cleanupUnreferencedTags 清理顶层tags中未在paths下任何operation引用的标签
func (p *Processor) cleanupUnreferencedTags(doc *swaggerDoc) bool {
	if len(doc.Tags) == 0 || doc.Paths == nil {
		return false
	}

	usedTags := make(map[string]bool)
	for _, pd := range *doc.Paths {
		item := pd.PathItemObject
		if item == nil {
			continue
		}
		for _, op := range []*swaggerOperation{item.Get, item.Delete, item.Post, item.Put, item.Patch, item.Head, item.Options} {
			if op == nil {
				continue
			}
			for _, tag := range op.Tags {
				usedTags[tag] = true
			}
		}
	}

	var filtered []swaggerTag
	for _, tag := range doc.Tags {
		if usedTags[tag.Name] {
			filtered = append(filtered, tag)
		} else if p.verbose {
			fmt.Printf("  🗑️ 移除未引用的tag: %s\n", tag.Name)
		}
	}

	if len(filtered) == len(doc.Tags) {
		return false
	}

	doc.Tags = filtered
	return true
}

// stripInjectTagFromSchema 从schema的所有属性description/title中剥离@inject_tag文本
func (p *Processor) stripInjectTagFromSchema(props *SchemaProperties) bool {
	modified := false
	for _, entry := range *props {
		prop := entry.Value
		if prop == nil {
			continue
		}
		oldDesc := prop.Description
		prop.Description = injectTagInDescRegex.ReplaceAllString(prop.Description, "")
		prop.Description = strings.TrimSpace(prop.Description)
		if prop.Description != oldDesc {
			modified = true
			if p.verbose {
				fmt.Printf("  🧹 剥离了 @inject_tag 文本\n")
			}
		}
		oldTitle := prop.Title
		prop.Title = injectTagInDescRegex.ReplaceAllString(prop.Title, "")
		prop.Title = strings.TrimSpace(prop.Title)
		if prop.Title != oldTitle {
			modified = true
		}
	}
	return modified
}

func stripInjectTagText(text string) (string, bool) {
	stripped := injectTagInDescRegex.ReplaceAllString(text, "")
	stripped = strings.TrimSpace(stripped)
	return stripped, stripped != text
}

func extractValidateFromSwaggerText(text string) string {
	tagLocs := injectTagRegexp.FindAllStringIndex(text, -1)
	for _, loc := range tagLocs {
		tagValue := extractTagValue(text[loc[1]:])
		if tagValue == "" {
			continue
		}
		if value := parseGoTagString(tagValue)["validate"]; value != "" {
			return value
		}
	}
	return ""
}

func appendRequired(required []string, fieldName string) []string {
	for _, item := range required {
		if item == fieldName {
			return required
		}
	}
	return append(required, fieldName)
}

func (p *Processor) processSwaggerParameters(doc *swaggerDoc, index *swaggerTagIndex) bool {
	if doc.Paths == nil {
		return false
	}

	modified := false
	for _, pd := range *doc.Paths {
		item := pd.PathItemObject
		if item == nil {
			continue
		}
		for _, op := range []*swaggerOperation{item.Get, item.Delete, item.Post, item.Put, item.Patch, item.Head, item.Options} {
			if op == nil {
				continue
			}
			for _, param := range op.Parameters {
				if param == nil {
					continue
				}
				validateValue := extractValidateFromSwaggerText(param.Description)
				if validateValue == "" {
					if ft, found := index.lookupField(param.Name); found {
						validateValue = ft.Validate
					}
				}
				if stripped, changed := stripInjectTagText(param.Description); changed {
					param.Description = stripped
					modified = true
				}
				if validateValue == "" {
					continue
				}
				constraints := ParseValidateToSwagger(validateValue)
				if constraints == nil {
					continue
				}
				p.applyParameterConstraints(param, constraints)
				if constraints.Required && param.In != "path" {
					param.Required = true
				}
				modified = true
				if p.verbose {
					fmt.Printf("  鉁?parameter %s 鈫?宸叉敞鍏ョ害鏉焅n", param.Name)
				}
			}
		}
	}
	return modified
}

// applyConstraints 将SwaggerConstraints应用到swagger属性上
func (p *Processor) applyConstraints(prop *swaggerProperty, c *SwaggerConstraints) {
	p.applyPropertyRangeConstraints(prop, c)
	if c.Pattern != "" {
		prop.Pattern = c.Pattern
	}
	if c.Format != "" && prop.Format == "" {
		prop.Format = c.Format
	}
	if len(c.Enum) > 0 {
		enumVals := make([]interface{}, len(c.Enum))
		for i, e := range c.Enum {
			enumVals[i] = e
		}
		prop.Enum = enumVals
	}
	if c.MinItems != nil {
		prop.MinItems = c.MinItems
	}
	if c.MaxItems != nil {
		prop.MaxItems = c.MaxItems
	}
	if c.Items != nil && prop.Items != nil {
		p.applyConstraints(prop.Items, c.Items)
	}
}

func (p *Processor) applyPropertyRangeConstraints(prop *swaggerProperty, c *SwaggerConstraints) {
	switch schemaConstraintKind(prop.Type) {
	case "string":
		if c.MinLength != nil {
			prop.MinLength = c.MinLength
		}
		if c.MaxLength != nil {
			prop.MaxLength = c.MaxLength
		}
		if c.Min != nil {
			prop.MinLength = floatToInt64(c.Min)
		}
		if c.Max != nil {
			prop.MaxLength = floatToInt64(c.Max)
		}
	case "array":
		if c.MinLength != nil {
			prop.MinItems = c.MinLength
		}
		if c.MaxLength != nil {
			prop.MaxItems = c.MaxLength
		}
		if c.Min != nil {
			prop.MinItems = floatToInt64(c.Min)
		}
		if c.Max != nil {
			prop.MaxItems = floatToInt64(c.Max)
		}
	default:
		if c.Min != nil {
			prop.Minimum = c.Min
		}
		if c.Max != nil {
			prop.Maximum = c.Max
		}
		if c.ExclusiveMinimum {
			prop.ExclusiveMinimum = true
		}
		if c.ExclusiveMaximum {
			prop.ExclusiveMaximum = true
		}
	}
}

func (p *Processor) applyParameterConstraints(param *swaggerParameter, c *SwaggerConstraints) {
	p.applyParameterRangeConstraints(param, c)
	if c.Pattern != "" {
		param.Pattern = c.Pattern
	}
	if c.Format != "" && param.Format == "" {
		param.Format = c.Format
	}
	if len(c.Enum) > 0 {
		enumVals := make([]interface{}, len(c.Enum))
		for i, e := range c.Enum {
			enumVals[i] = e
		}
		param.Enum = enumVals
	}
	if c.MinItems != nil {
		param.MinItems = c.MinItems
	}
	if c.MaxItems != nil {
		param.MaxItems = c.MaxItems
	}
	if c.Items != nil && param.Items != nil {
		p.applyConstraints(param.Items, c.Items)
	}
}

func (p *Processor) applyParameterRangeConstraints(param *swaggerParameter, c *SwaggerConstraints) {
	switch schemaConstraintKind(param.Type) {
	case "string":
		if c.MinLength != nil {
			param.MinLength = c.MinLength
		}
		if c.MaxLength != nil {
			param.MaxLength = c.MaxLength
		}
		if c.Min != nil {
			param.MinLength = floatToInt64(c.Min)
		}
		if c.Max != nil {
			param.MaxLength = floatToInt64(c.Max)
		}
	case "array":
		if c.MinLength != nil {
			param.MinItems = c.MinLength
		}
		if c.MaxLength != nil {
			param.MaxItems = c.MaxLength
		}
		if c.Min != nil {
			param.MinItems = floatToInt64(c.Min)
		}
		if c.Max != nil {
			param.MaxItems = floatToInt64(c.Max)
		}
	default:
		if c.Min != nil {
			param.Minimum = c.Min
		}
		if c.Max != nil {
			param.Maximum = c.Max
		}
		if c.ExclusiveMinimum {
			param.ExclusiveMinimum = true
		}
		if c.ExclusiveMaximum {
			param.ExclusiveMaximum = true
		}
	}
}

func schemaConstraintKind(schemaType string) string {
	switch schemaType {
	case "string":
		return "string"
	case "array":
		return "array"
	case "integer", "number":
		return "number"
	default:
		return ""
	}
}

func floatToInt64(v *float64) *int64 {
	if v == nil {
		return nil
	}
	i := int64(*v)
	return &i
}
