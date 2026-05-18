/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: Lion-3625 lsheng758@gmail.com
 * @LastEditTime: 2026-05-18 21:53:38
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

// BuildMessageValidateIndex messageName -> fieldName -> validate
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

// AuditSwaggerFile 对比单个 swagger 与 proto 索引
func AuditSwaggerFile(swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC) (AuditIssue, error) {
	doc, _, err := ReadSwaggerFile(swaggerFile)
	if err != nil {
		return AuditIssue{}, err
	}

	var result AuditIssue
	auditPathParameters(doc, swaggerFile, messages, operations, &result)
	auditDefinitions(doc, swaggerFile, messages, &result)
	return result, nil
}

func auditPathParameters(doc *swaggerDoc, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, result *AuditIssue) {
	if doc.Paths == nil {
		return
	}
	for _, pd := range *doc.Paths {
		auditPathItem(pd.PathItemObject, swaggerFile, messages, operations, result)
	}
}

func auditPathItem(item *swaggerPathItemObject, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, result *AuditIssue) {
	if item == nil {
		return
	}
	for _, op := range pathItemOperations(item) {
		auditOperation(op, swaggerFile, messages, operations, result)
	}
}

func pathItemOperations(item *swaggerPathItemObject) []*swaggerOperation {
	return []*swaggerOperation{item.Get, item.Delete, item.Post, item.Put, item.Patch, item.Head, item.Options}
}

func auditOperation(op *swaggerOperation, swaggerFile string, messages map[string]map[string]string, operations map[string]ProtoRPC, result *AuditIssue) {
	if op == nil {
		return
	}
	rpc, ok := operations[op.OperationId]
	if !ok {
		return
	}
	fields := messages[rpc.RequestType]
	for _, param := range op.Parameters {
		maybeRecordParameterMismatch(result, swaggerFile, op.OperationId, rpc.RequestType, fields, param)
	}
}

func maybeRecordParameterMismatch(result *AuditIssue, swaggerFile, operationID, requestType string, fields map[string]string, param *swaggerParameter) {
	if !shouldAuditParameter(param) {
		return
	}
	expected := expectedParamRequired(fields, param.Name)
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

func shouldAuditParameter(param *swaggerParameter) bool {
	if param == nil {
		return false
	}
	return isAuditableParameter(param.In)
}

func isAuditableParameter(paramIn string) bool {
	return paramIn == "query" || paramIn == "path"
}

func auditDefinitions(doc *swaggerDoc, swaggerFile string, messages map[string]map[string]string, result *AuditIssue) {
	if doc.Definitions == nil {
		return
	}
	for defName, schema := range doc.Definitions {
		auditDefinitionSchema(defName, schema, swaggerFile, messages, result)
	}
}

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

func schemaHasRequiredFields(schema *swaggerSchema) bool {
	return schema != nil && len(schema.Required) > 0
}

func requiredFieldSet(required []string) map[string]bool {
	set := make(map[string]bool, len(required))
	for _, f := range required {
		set[f] = true
	}
	return set
}

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

func expectedParamRequired(fields map[string]string, paramName string) bool {
	if fields == nil {
		return false
	}
	if idx := strings.Index(paramName, "."); idx >= 0 {
		return hasValidateRequired(fields[paramName[:idx]])
	}
	return hasValidateRequired(fields[paramName])
}

func lookupFieldValidate(fields map[string]string, paramName string) string {
	if fields == nil {
		return ""
	}
	if idx := strings.Index(paramName, "."); idx >= 0 {
		return fields[paramName[:idx]]
	}
	return fields[paramName]
}

func hasValidateRequired(validate string) bool {
	if validate == "" {
		return false
	}
	c := ParseValidateToSwagger(validate)
	return c != nil && c.Required
}

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

func longestCandidateMatch(defName, msgName string) int {
	bestLen := 0
	for _, cand := range openAPISchemaNameCandidates("", msgName) {
		if length := matchedCandidateLength(defName, cand); length > bestLen {
			bestLen = length
		}
	}
	return bestLen
}

func matchedCandidateLength(defName, cand string) int {
	if !definitionMatchesCandidate(defName, cand) {
		return 0
	}
	return len(cand)
}

func definitionMatchesCandidate(defName, cand string) bool {
	if cand == "" {
		return false
	}
	if defName == cand {
		return true
	}
	return strings.HasSuffix(defName, cand)
}
