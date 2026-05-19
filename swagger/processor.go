/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-18 23:01:33
 * @FilePath: /protoc-go-inject-tag/swagger/processor.go
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

// swaggerTagIndex swagger标签索引，按 message schema 查找字段标签
type swaggerTagIndex struct {
	schemas    map[string]map[string]FieldTag
	fieldIndex *protoFieldIndex
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

	expandedProtos, err := ExpandProtoFilesWithImports(protoFiles)
	if err != nil {
		return fmt.Errorf("展开 proto import 失败: %w", err)
	}

	protoTags, err := p.collectProtoTags(expandedProtos)
	if err != nil {
		return err
	}
	fieldIndex, err := BuildProtoFieldIndex(expandedProtos)
	if err != nil {
		return fmt.Errorf("构建 proto 字段索引失败: %w", err)
	}
	tagIndex := newSwaggerTagIndex(protoTags, fieldIndex)

	operationRequests, err := BuildOperationRequestIndex(expandedProtos)
	if err != nil {
		return fmt.Errorf("解析 proto RPC 失败: %w", err)
	}

	if p.verbose {
		fmt.Printf("📖 读取 swagger 文件: %s\n", swaggerFile)
	}

	doc, format, err := ReadSwaggerFile(swaggerFile)
	if err != nil {
		return err
	}

	modified := p.processDefinitions(doc, tagIndex)
	if p.processSwaggerParameters(doc, tagIndex, operationRequests) {
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

// collectProtoTags 收集 proto 文件中的 @inject_tag 注解
func (p *Processor) collectProtoTags(protoFiles []string) ([]ProtoFieldTags, error) {
	var protoTags []ProtoFieldTags
	for _, protoFile := range protoFiles {
		tags, err := ParseProtoFile(protoFile)
		if err != nil {
			return nil, fmt.Errorf("解析 proto 文件失败 %s: %w", protoFile, err)
		}
		p.logDiscoveredTags(tags)
		protoTags = append(protoTags, tags...)
	}
	return protoTags, nil
}

func (p *Processor) logDiscoveredTags(tags []ProtoFieldTags) {
	if !p.verbose {
		return
	}
	for _, t := range tags {
		fmt.Printf("  发现 %s.%s → validate:%q\n", t.MessageName, t.FieldName, t.Tags.Validate)
	}
}

// processDefinitions 处理 swagger 文件中的 definitions
func (p *Processor) processDefinitions(doc *swaggerDoc, tagIndex *swaggerTagIndex) bool {
	modified := false
	for defName, schema := range doc.Definitions {
		if schema == nil || schema.Properties == nil || schema.Properties.Len() == 0 {
			continue
		}
		msgTags, _ := tagIndex.lookupSchema(defName)
		for _, entry := range *schema.Properties {
			if p.processDefinitionProperty(defName, entry.Key, entry.Value, schema, msgTags) {
				modified = true
			}
		}
	}
	return modified
}

// processDefinitionProperty 处理 swagger 文件中的 definitions 属性
func (p *Processor) processDefinitionProperty(
	defName, propName string,
	prop *swaggerProperty,
	schema *swaggerSchema,
	msgTags map[string]FieldTag,
) bool {
	if prop == nil {
		return false
	}
	modified := stripPropertyInjectTags(prop)
	if p.applyPropertyValidate(defName, propName, prop, schema, msgTags) {
		modified = true
	}
	return modified
}

// stripPropertyInjectTags 从 swagger 属性中剥离 @inject_tag 注解
func stripPropertyInjectTags(prop *swaggerProperty) bool {
	modified := false
	if stripped, changed := stripInjectTagText(prop.Description); changed {
		prop.Description = stripped
		modified = true
	}
	if stripped, changed := stripInjectTagText(prop.Title); changed {
		prop.Title = stripped
		modified = true
	}
	return modified
}

// resolvePropertyValidate 解析 swagger 属性中的 validate 注解
func resolvePropertyValidate(prop *swaggerProperty, msgTags map[string]FieldTag, propName string) string {
	validateValue := extractValidateFromSwaggerText(prop.Description)
	if validateValue == "" {
		validateValue = extractValidateFromSwaggerText(prop.Title)
	}
	if validateValue != "" || msgTags == nil {
		return validateValue
	}
	if ft, found := msgTags[propName]; found {
		return ft.Validate
	}
	return ""
}

// applyPropertyValidate 应用 swagger 属性中的 validate 注解
func (p *Processor) applyPropertyValidate(
	defName, propName string,
	prop *swaggerProperty,
	schema *swaggerSchema,
	msgTags map[string]FieldTag,
) bool {
	validateValue := resolvePropertyValidate(prop, msgTags, propName)
	if validateValue == "" {
		return false
	}
	constraints := ParseValidateToSwagger(validateValue)
	if constraints == nil {
		return false
	}
	p.applyConstraints(prop, constraints)
	if constraints.Required {
		schema.Required = appendRequired(schema.Required, propName)
	}
	if p.verbose {
		fmt.Printf("  ✅ %s.%s → 已注入约束\n", defName, propName)
	}
	return true
}

// newSwaggerTagIndex 创建 swagger 标签索引
func newSwaggerTagIndex(tags []ProtoFieldTags, fieldIndex *protoFieldIndex) *swaggerTagIndex {
	index := &swaggerTagIndex{
		schemas:    make(map[string]map[string]FieldTag),
		fieldIndex: fieldIndex,
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
	}
	return index
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

func (i *swaggerTagIndex) lookupMessageField(packageName, messageName, fieldName string) (FieldTag, bool) {
	if i == nil || messageName == "" || fieldName == "" {
		return FieldTag{}, false
	}
	if ft, ok := i.lookupFieldInCandidates(packageName, messageName, fieldName); ok {
		return ft, true
	}
	return i.lookupFieldByMessageSuffix(messageName, fieldName)
}

func (i *swaggerTagIndex) lookupFieldInCandidates(packageName, messageName, fieldName string) (FieldTag, bool) {
	for _, schemaName := range openAPISchemaNameCandidates(packageName, messageName) {
		if tags, ok := i.schemas[schemaName]; ok {
			if ft, found := lookupFieldInTags(tags, fieldName); found {
				return ft, true
			}
		}
	}
	return FieldTag{}, false
}

func lookupFieldInTags(tags map[string]FieldTag, fieldName string) (FieldTag, bool) {
	for _, name := range parameterNameCandidates(fieldName) {
		if ft, found := tags[name]; found {
			return ft, true
		}
	}
	return FieldTag{}, false
}

func (i *swaggerTagIndex) lookupFieldByMessageSuffix(messageName, fieldName string) (FieldTag, bool) {
	var bestTag FieldTag
	bestLen := 0
	found := false
	for schemaName, tags := range i.schemas {
		if len(schemaName) <= bestLen || !strings.HasSuffix(schemaName, messageName) {
			continue
		}
		if ft, ok := lookupFieldInTags(tags, fieldName); ok {
			bestTag = ft
			bestLen = len(schemaName)
			found = true
		}
	}
	if found {
		return bestTag, true
	}
	return FieldTag{}, false
}

func parameterNameCandidates(name string) []string {
	return uniqueStrings([]string{name, jsonCamelCase(name)})
}

func validateForParameter(index *swaggerTagIndex, rpc ProtoRPC, paramName string) string {
	if index == nil || paramName == "" {
		return ""
	}
	if idx := strings.Index(paramName, "."); idx >= 0 {
		return validateForNestedParameter(index, rpc, paramName[:idx], paramName[idx+1:], paramName)
	}
	if ft, ok := index.lookupMessageField(rpc.PackageName, rpc.RequestType, paramName); ok {
		return ft.Validate
	}
	return ""
}

func validateForNestedParameter(index *swaggerTagIndex, rpc ProtoRPC, parent, child, fullName string) string {
	if index != nil && index.fieldIndex != nil && index.fieldIndex.childOmitsNestedRequired(rpc.RequestType, parent, child) {
		return index.fieldIndex.nestedChildValidate(rpc.RequestType, parent, child)
	}
	if parentTag, ok := index.lookupMessageField(rpc.PackageName, rpc.RequestType, parent); ok {
		if v := nestedParentValidate(parentTag, child); v != "" {
			return v
		}
	}
	if childTag, ok := index.lookupMessageField(rpc.PackageName, rpc.RequestType, fullName); ok {
		return childTag.Validate
	}
	return ""
}

func nestedParentValidate(parentTag FieldTag, child string) string {
	if parentTag.Validate == "" || child == "" {
		return ""
	}
	parentConstraints := ParseValidateToSwagger(parentTag.Validate)
	if parentConstraints != nil && parentConstraints.Required {
		return parentTag.Validate
	}
	return ""
}

// cleanupUnreferencedTags 清理顶层tags中未在paths下任何operation引用的标签
func (p *Processor) cleanupUnreferencedTags(doc *swaggerDoc) bool {
	if len(doc.Tags) == 0 || doc.Paths == nil {
		return false
	}

	filtered := filterReferencedTags(doc.Tags, collectReferencedTags(doc), p.verbose)
	if len(filtered) == len(doc.Tags) {
		return false
	}
	doc.Tags = filtered
	return true
}

func collectReferencedTags(doc *swaggerDoc) map[string]bool {
	usedTags := make(map[string]bool)
	for _, pd := range *doc.Paths {
		item := pd.PathItemObject
		if item == nil {
			continue
		}
		for _, op := range pathItemOperations(item) {
			if op == nil {
				continue
			}
			for _, tag := range op.Tags {
				usedTags[tag] = true
			}
		}
	}
	return usedTags
}

func filterReferencedTags(tags []swaggerTag, used map[string]bool, verbose bool) []swaggerTag {
	var filtered []swaggerTag
	for _, tag := range tags {
		if used[tag.Name] {
			filtered = append(filtered, tag)
			continue
		}
		if verbose {
			fmt.Printf("  🗑️ 移除未引用的tag: %s\n", tag.Name)
		}
	}
	return filtered
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

func (p *Processor) processSwaggerParameters(doc *swaggerDoc, index *swaggerTagIndex, operationRequests map[string]ProtoRPC) bool {
	if doc.Paths == nil {
		return false
	}
	modified := false
	for _, pd := range *doc.Paths {
		if p.processPathItemParameters(pd.PathItemObject, operationRequests, index) {
			modified = true
		}
	}
	return modified
}

func (p *Processor) processPathItemParameters(item *swaggerPathItemObject, operationRequests map[string]ProtoRPC, index *swaggerTagIndex) bool {
	if item == nil {
		return false
	}
	modified := false
	for _, op := range pathItemOperations(item) {
		if op == nil {
			continue
		}
		rpc, hasRPC := operationRequests[op.OperationId]
		for _, param := range op.Parameters {
			if p.processSwaggerParameter(param, rpc, hasRPC, index) {
				modified = true
			}
		}
	}
	return modified
}

func (p *Processor) processSwaggerParameter(param *swaggerParameter, rpc ProtoRPC, hasRPC bool, index *swaggerTagIndex) bool {
	if param == nil {
		return false
	}

	validateValue := extractValidateFromSwaggerText(param.Description)
	if validateValue == "" && hasRPC {
		validateValue = validateForParameter(index, rpc, param.Name)
	}

	modified := false
	if stripped, changed := stripInjectTagText(param.Description); changed {
		param.Description = stripped
		modified = true
	}

	if clearUnvalidatedQueryRequired(param, hasRPC, validateValue) {
		modified = true
		return modified
	}
	if validateValue == "" {
		return modified
	}
	if p.applyParameterValidate(param, validateValue) {
		return true
	}
	return modified
}

func (p *Processor) applyParameterValidate(param *swaggerParameter, validateValue string) bool {
	constraints := ParseValidateToSwagger(validateValue)
	if constraints == nil {
		return false
	}
	p.applyParameterConstraints(param, constraints)
	modified := false
	if param.In == "path" || param.In == "query" {
		if param.Required != constraints.Required {
			param.Required = constraints.Required
			modified = true
		}
	} else {
		modified = true
	}
	if p.verbose {
		fmt.Printf("  ✅ parameter %s → 已注入约束\n", param.Name)
	}
	return modified
}

// applyConstraints 将SwaggerConstraints应用到swagger属性上
func (p *Processor) applyConstraints(prop *swaggerProperty, c *SwaggerConstraints) {
	applyConstraintsToTarget(propertyTarget{prop}, c)
}

// applyParameterConstraints 将SwaggerConstraints应用到swagger参数上
func (p *Processor) applyParameterConstraints(param *swaggerParameter, c *SwaggerConstraints) {
	applyConstraintsToTarget(parameterTarget{param}, c)
}

// applyConstraintsToTarget 统一将约束应用到目标（消除 Property/Parameter 重复代码）
func applyConstraintsToTarget(target constraintTarget, c *SwaggerConstraints) {
	applyRangeConstraints(target, c)
	applyScalarConstraints(target, c)
	if c.Items != nil {
		if items := target.ItemsTarget(); items != nil {
			applyConstraintsToTarget(items, c.Items)
		}
	}
}

// applyScalarConstraints 应用标量约束（pattern/format/enum/minItems/maxItems）
func applyScalarConstraints(target constraintTarget, c *SwaggerConstraints) {
	if c.Pattern != "" {
		target.SetPattern(c.Pattern)
	}
	if c.Format != "" {
		target.SetFormatIfEmpty(c.Format)
	}
	if len(c.Enum) > 0 {
		target.SetEnum(enumToInterfaceSlice(c.Enum))
	}
	if c.MinItems != nil {
		target.SetMinItems(c.MinItems)
	}
	if c.MaxItems != nil {
		target.SetMaxItems(c.MaxItems)
	}
}

// enumToInterfaceSlice 将枚举值字符串切片转换为 interface{} 切片
func enumToInterfaceSlice(enum []string) []interface{} {
	enumVals := make([]interface{}, len(enum))
	for i, e := range enum {
		enumVals[i] = e
	}
	return enumVals
}

// applyRangeConstraints 根据类型应用范围约束
func applyRangeConstraints(target constraintTarget, c *SwaggerConstraints) {
	switch schemaConstraintKind(target.Type()) {
	case "string":
		applyStringRange(target, c)
	case "array":
		applyArrayRange(target, c)
	default:
		applyNumericRange(target, c)
	}
}

// applyStringRange 应用字符串范围约束（minLength/maxLength）
func applyStringRange(target constraintTarget, c *SwaggerConstraints) {
	if c.MinLength != nil {
		target.SetMinLength(c.MinLength)
	}
	if c.MaxLength != nil {
		target.SetMaxLength(c.MaxLength)
	}
	if c.Min != nil {
		target.SetMinLength(floatToInt64(c.Min))
	}
	if c.Max != nil {
		target.SetMaxLength(floatToInt64(c.Max))
	}
}

// applyArrayRange 应用数组范围约束（minItems/maxItems）
func applyArrayRange(target constraintTarget, c *SwaggerConstraints) {
	if c.MinLength != nil {
		target.SetMinItems(c.MinLength)
	}
	if c.MaxLength != nil {
		target.SetMaxItems(c.MaxLength)
	}
	if c.Min != nil {
		target.SetMinItems(floatToInt64(c.Min))
	}
	if c.Max != nil {
		target.SetMaxItems(floatToInt64(c.Max))
	}
}

// applyNumericRange 应用数值范围约束（minimum/maximum/exclusive）
func applyNumericRange(target constraintTarget, c *SwaggerConstraints) {
	if c.Min != nil {
		target.SetMinimum(c.Min)
	}
	if c.Max != nil {
		target.SetMaximum(c.Max)
	}
	if c.ExclusiveMinimum {
		target.SetExclusiveMinimum(true)
	}
	if c.ExclusiveMaximum {
		target.SetExclusiveMaximum(true)
	}
}

func clearUnvalidatedQueryRequired(param *swaggerParameter, hasRPC bool, validateValue string) bool {
	if param.In != "query" || !hasRPC || validateValue != "" || !param.Required {
		return false
	}
	param.Required = false
	return true
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
