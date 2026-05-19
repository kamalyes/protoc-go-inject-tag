/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-18 23:33:51
 * @FilePath: \protoc-go-inject-tag\swagger\audit.go
 * @Description: Swagger与proto validate一致性检查
 *
 * Copyright (c) 2026 by Lion-3625, All Rights Reserved.
 */
package swagger

import "strings"

// ParameterRequiredIssue query/path 参数 required 与 proto 不一致
type ParameterRequiredIssue struct {
	SwaggerFile     string
	OperationID     string
	RequestType     string
	ParamName       string
	ParamIn         string
	SwaggerRequired bool
	ProtoExpected   bool
	ProtoValidate   string
}

// DefinitionRequiredIssue definition.required 与 proto 字段 validate 不一致
type DefinitionRequiredIssue struct {
	SwaggerFile    string
	DefinitionName string
	FieldName      string
	InRequiredList bool
	ProtoExpected  bool
	ProtoValidate  string
}

// AuditIssue 审计结果
type AuditIssue struct {
	Parameters  []ParameterRequiredIssue
	Definitions []DefinitionRequiredIssue
}

// BuildMessageValidateIndex 构建消息名称到字段校验规则的索引
// 返回 messageName -> fieldName -> validate 的映射
func BuildMessageValidateIndex(protoFiles []string) (map[string]map[string]string, error) {
	index := make(map[string]map[string]string)
	for _, pf := range protoFiles {
		tags, err := ParseProtoFile(pf)
		if err != nil {
			return nil, err
		}
		mergeProtoFieldTags(index, tags)
	}
	return index, nil
}

// mergeProtoFieldTags 将解析出的字段标签合并到索引中
func mergeProtoFieldTags(index map[string]map[string]string, tags []ProtoFieldTags) {
	for _, t := range tags {
		if t.Tags.Validate == "" {
			continue
		}
		if index[t.MessageName] == nil {
			index[t.MessageName] = make(map[string]string)
		}
		index[t.MessageName][t.FieldName] = t.Tags.Validate
	}
}

// AuditSwaggerFile 对比单个 swagger 文件与 proto 索引，检查 required 一致性
func AuditSwaggerFile(swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, fieldIndex *protoFieldIndex) (AuditIssue, error) {
	doc, _, err := ReadSwaggerFile(swaggerFile)
	if err != nil {
		return AuditIssue{}, err
	}

	var result AuditIssue
	auditPathParameters(doc, swaggerFile, messages, operations, fieldIndex, &result)
	auditDefinitions(doc, swaggerFile, messages, &result)
	return result, nil
}

// auditPathParameters 审计路径参数的 required 一致性
func auditPathParameters(doc *swaggerDoc, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, fieldIndex *protoFieldIndex, result *AuditIssue) {
	if doc.Paths == nil {
		return
	}
	for _, pd := range *doc.Paths {
		auditPathItem(pd.PathItemObject, swaggerFile, messages, operations, fieldIndex, result)
	}
}

// auditPathItem 审计单个路径项下的所有操作
func auditPathItem(item *swaggerPathItemObject, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, fieldIndex *protoFieldIndex, result *AuditIssue) {
	if item == nil {
		return
	}
	for _, op := range pathItemOperations(item) {
		auditOperation(op, swaggerFile, messages, operations, fieldIndex, result)
	}
}

// pathItemOperations 收集路径项下的所有 HTTP 方法操作
func pathItemOperations(item *swaggerPathItemObject) []*swaggerOperation {
	return []*swaggerOperation{item.Get, item.Delete, item.Post, item.Put, item.Patch, item.Head, item.Options}
}

// auditOperation 审计单个操作的参数 required 一致性
func auditOperation(op *swaggerOperation, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, fieldIndex *protoFieldIndex, result *AuditIssue) {
	if op == nil {
		return
	}
	rpc, ok := operations[op.OperationId]
	if !ok {
		return
	}
	fields := messages[rpc.RequestType]
	for _, param := range op.Parameters {
		maybeRecordParameterMismatch(result, swaggerFile, op.OperationId, rpc.RequestType, fields, fieldIndex, param)
	}
}

// maybeRecordParameterMismatch 检查参数的 required 状态是否与 proto 一致，不一致则记录
func maybeRecordParameterMismatch(result *AuditIssue, swaggerFile, operationID, requestType string, fields map[string]string, fieldIndex *protoFieldIndex, param *swaggerParameter) {
	if !shouldAuditParameter(param) {
		return
	}
	expected := expectedParamRequired(fields, fieldIndex, requestType, param.Name)
	if param.Required == expected {
		return
	}
	result.Parameters = append(result.Parameters, ParameterRequiredIssue{
		SwaggerFile:     swaggerFile,
		OperationID:     operationID,
		RequestType:     requestType,
		ParamName:       param.Name,
		ParamIn:         param.In,
		SwaggerRequired: param.Required,
		ProtoExpected:   expected,
		ProtoValidate:   lookupFieldValidate(fields, param.Name),
	})
}

// shouldAuditParameter 判断参数是否需要审计（仅 query/path 类型）
func shouldAuditParameter(param *swaggerParameter) bool {
	if param == nil {
		return false
	}
	return isAuditableParameter(param.In)
}

// isAuditableParameter 判断参数位置是否可审计
func isAuditableParameter(paramIn string) bool {
	return paramIn == "query" || paramIn == "path"
}

// auditDefinitions 审计 definitions 中的 required 字段一致性
func auditDefinitions(doc *swaggerDoc, swaggerFile string, messages map[string]map[string]string, result *AuditIssue) {
	if doc.Definitions == nil {
		return
	}
	for defName, schema := range doc.Definitions {
		auditDefinitionSchema(defName, schema, swaggerFile, messages, result)
	}
}

// auditDefinitionSchema 审计单个 definition 的 required 字段
func auditDefinitionSchema(defName string, schema *swaggerSchema, swaggerFile string, messages map[string]map[string]string, result *AuditIssue) {
	if !schemaHasRequiredFields(schema) {
		return
	}
	fields := messages[findMessageForDefinition(defName, messages)]
	if fields == nil {
		return
	}
	requiredSet := requiredFieldSet(schema.Required)
	for fieldName, validate := range fields {
		maybeRecordDefinitionMismatch(result, swaggerFile, defName, fieldName, validate, requiredSet[fieldName])
	}
}

// schemaHasRequiredFields 判断 schema 是否包含 required 字段列表
func schemaHasRequiredFields(schema *swaggerSchema) bool {
	return schema != nil && len(schema.Required) > 0
}

// requiredFieldSet 将 required 字段列表转为集合
func requiredFieldSet(required []string) map[string]bool {
	set := make(map[string]bool, len(required))
	for _, f := range required {
		set[f] = true
	}
	return set
}

// maybeRecordDefinitionMismatch 检查 definition 字段的 required 状态是否与 proto 一致
func maybeRecordDefinitionMismatch(result *AuditIssue, swaggerFile, defName, fieldName, validate string, inRequired bool) {
	want := hasValidateRequired(validate)
	if inRequired == want {
		return
	}
	result.Definitions = append(result.Definitions, DefinitionRequiredIssue{
		SwaggerFile:    swaggerFile,
		DefinitionName: defName,
		FieldName:      fieldName,
		InRequiredList: inRequired,
		ProtoExpected:  want,
		ProtoValidate:  validate,
	})
}

// expectedParamRequired 根据 proto validate 规则判断参数是否应为 required
func expectedParamRequired(fields map[string]string, fieldIndex *protoFieldIndex, requestType, paramName string) bool {
	if idx := strings.Index(paramName, "."); idx >= 0 {
		parent := paramName[:idx]
		child := paramName[idx+1:]
		if fieldIndex != nil && fieldIndex.childOmitsNestedRequired(requestType, parent, child) {
			return hasValidateRequired(fieldIndex.nestedChildValidate(requestType, parent, child))
		}
		if fields != nil {
			return hasValidateRequired(fields[parent])
		}
		return false
	}
	if fields != nil {
		return hasValidateRequired(fields[paramName])
	}
	return false
}

// lookupFieldValidate 查找字段的 validate 规则值
func lookupFieldValidate(fields map[string]string, paramName string) string {
	if fields == nil {
		return ""
	}
	if idx := strings.Index(paramName, "."); idx >= 0 {
		return fields[paramName[:idx]]
	}
	return fields[paramName]
}

// hasValidateRequired 判断 validate 规则中是否包含 required
func hasValidateRequired(validate string) bool {
	if validate == "" {
		return false
	}
	c := ParseValidateToSwagger(validate)
	return c != nil && c.Required
}

// findMessageForDefinition 根据 definition 名称查找最匹配的 proto message
func findMessageForDefinition(defName string, messages map[string]map[string]string) string {
	var best string
	bestLen := 0
	for msgName := range messages {
		length := longestCandidateMatch(defName, msgName)
		if length <= bestLen {
			continue
		}
		best = msgName
		bestLen = length
	}
	return best
}

// longestCandidateMatch 在 definition 名称中查找与候选名称最长匹配的长度
func longestCandidateMatch(defName, msgName string) int {
	bestLen := 0
	for _, cand := range openAPISchemaNameCandidates("", msgName) {
		if length := matchedCandidateLength(defName, cand); length > bestLen {
			bestLen = length
		}
	}
	return bestLen
}

// matchedCandidateLength 返回候选名称与 definition 名称匹配的长度，不匹配返回 0
func matchedCandidateLength(defName, cand string) int {
	if !definitionMatchesCandidate(defName, cand) {
		return 0
	}
	return len(cand)
}

// definitionMatchesCandidate 判断 definition 名称是否匹配候选名称（完全匹配或后缀匹配）
func definitionMatchesCandidate(defName, cand string) bool {
	if cand == "" {
		return false
	}
	if defName == cand {
		return true
	}
	return strings.HasSuffix(defName, cand)
}
