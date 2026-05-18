/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-18 23:30:00
 * @FilePath: \protoc-go-inject-tag\swagger\proto_parser.go
 * @Description: Proto文件解析器，统一处理 message/field 标签提取和 service/rpc 定义提取
 *
 * 支持的注解格式：
 *   - // @inject_tag: validate:"required"
 *   - // @gotags: json:"name" validate:"required,min=3"
 *   - // @inject_tags: json:"province" validate:"required" @gotags: gorm:"type:varchar(50)"
 *   - 同一行多个标签用空格分隔，每个标签独立解析
 *
 * 解析能力：
 *   1. ParseProtoFile — 从 proto 文件提取 @inject_tag/@gotags 注解字段
 *   2. ParseProtoRPCs — 从 proto 文件提取 service/rpc 定义
 *   3. BuildOperationRequestIndex — 合并多个 proto 文件的 RPC 映射
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// ══════════════════════════════════════════════════════════════════════════════
// 公共类型定义
// ══════════════════════════════════════════════════════════════════════════════

// FieldTag 字段的Go标签信息
type FieldTag struct {
	Validate string // validate标签值，如 "required,min=3,max=50"
	GoTags   string // 完整的原始标签字符串
}

// ProtoFieldTags 从proto文件解析出的单个字段标签信息
type ProtoFieldTags struct {
	PackageName string
	MessageName string // proto message名称
	FieldName   string // 字段名称（snake_case）
	JSONName    string
	Tags        FieldTag // 解析后的标签
}

// ProtoRPC 单个 RPC 方法的元信息
type ProtoRPC struct {
	PackageName  string
	ServiceName  string
	MethodName   string
	RequestType  string
	ResponseType string
	OperationID  string
}

// ══════════════════════════════════════════════════════════════════════════════
// 公共正则表达式
// ══════════════════════════════════════════════════════════════════════════════

var (
	// injectTagRegexp 匹配行内所有 @inject_tag/@gotags/@inject_tags 注解
	injectTagRegexp   = regexp.MustCompile(`@(?:inject_tag|gotags?|inject_tags?):\s*`)
	serviceLineRegexp = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	rpcLineRegexp     = regexp.MustCompile(`^rpc\s+(\w+)\s*\(\s*([\w.]+)\s*\)\s*returns\s*\(\s*([\w.]+)\s*\)`)
	protoJSONNameRegexp = regexp.MustCompile(`json_name\s*=\s*"([^"]+)"`)
)

// ══════════════════════════════════════════════════════════════════════════════
// 公共辅助函数
// ══════════════════════════════════════════════════════════════════════════════

// isSkippableProtoLine 判断一行是否可跳过（空行或纯注释）
func isSkippableProtoLine(trimmed string) bool {
	return trimmed == "" || strings.HasPrefix(trimmed, "//")
}

// extractPackageName 从 "package xxx;" 行中提取包名
func extractPackageName(line string) string {
	line = strings.TrimSpace(strings.TrimSuffix(line, ";"))
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "package" {
		return ""
	}
	return parts[1]
}

// shortTypeName 去掉类型的包前缀，如 "pkg.Type" → "Type"
func shortTypeName(typeName string) string {
	if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		return typeName[idx+1:]
	}
	return typeName
}

// ══════════════════════════════════════════════════════════════════════════════
// Message/Field 标签解析
// ══════════════════════════════════════════════════════════════════════════════

// protoFieldParseState 解析 proto 文件中 message/field 标签的状态
type protoFieldParseState struct {
	result       []ProtoFieldTags
	packageName  string
	currentMsg   string
	msgBodyDepth int
}

// ParseProtoFile 解析proto文件，提取所有带@inject_tag注解的字段
func ParseProtoFile(filename string) ([]ProtoFieldTags, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	state := &protoFieldParseState{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		state.processLine(scanner.Text())
	}
	return state.result, scanner.Err()
}

// processLine 处理一行 proto 字段定义
func (s *protoFieldParseState) processLine(line string) {
	trimmed := strings.TrimSpace(line)
	if isSkippableProtoLine(trimmed) {
		return
	}
	if s.tryPackage(trimmed) {
		return
	}
	s.tryMessageStart(trimmed)
	if s.currentMsg == "" {
		return
	}
	s.updateMessageDepth(trimmed)
	if s.currentMsg == "" {
		return
	}
	s.tryAppendFieldTag(line)
}

// tryPackage 尝试解析 proto 包定义
func (s *protoFieldParseState) tryPackage(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "package ") {
		return false
	}
	s.packageName = extractPackageName(trimmed)
	return true
}

// tryMessageStart 尝试解析 proto 消息定义
func (s *protoFieldParseState) tryMessageStart(trimmed string) {
	if !strings.HasPrefix(trimmed, "message ") {
		return
	}
	parts := strings.Fields(trimmed)
	if len(parts) >= 2 {
		s.currentMsg = parts[1]
		s.msgBodyDepth = 0
	}
}

// updateMessageDepth 更新 message 嵌套深度，深度归零时退出当前 message
func (s *protoFieldParseState) updateMessageDepth(trimmed string) {
	s.msgBodyDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
	if s.msgBodyDepth <= 0 {
		s.currentMsg = ""
	}
}

// tryAppendFieldTag 尝试从字段行中提取 @inject_tag 注解
func (s *protoFieldParseState) tryAppendFieldTag(line string) {
	tagLocs := injectTagRegexp.FindAllStringIndex(line, -1)
	if len(tagLocs) == 0 {
		return
	}
	codePart := line[:tagLocs[0][0]]
	fieldName := extractFieldName(codePart)
	if fieldName == "" {
		return
	}
	s.result = append(s.result, ProtoFieldTags{
		PackageName: s.packageName,
		MessageName: s.currentMsg,
		FieldName:   fieldName,
		JSONName:    extractProtoJSONName(codePart),
		Tags:        buildFieldTagFromLine(line, tagLocs),
	})
}

// buildFieldTagFromLine 从一行proto字段定义中构建 FieldTag 结构体
func buildFieldTagFromLine(line string, tagLocs [][]int) FieldTag {
	mergedTagStr := mergeInjectTagValues(line, tagLocs)
	tags := FieldTag{GoTags: mergedTagStr}
	if v, ok := parseGoTagString(mergedTagStr)["validate"]; ok {
		tags.Validate = v
	}
	return tags
}

// mergeInjectTagValues 合并同一行中多个 @inject_tag 注解的值
func mergeInjectTagValues(line string, tagLocs [][]int) string {
	var allGoTags []string
	for _, loc := range tagLocs {
		if tagValue := extractTagValue(line[loc[1]:]); tagValue != "" {
			allGoTags = append(allGoTags, tagValue)
		}
	}
	return strings.Join(allGoTags, " ")
}

// extractTagValue 从标签注解后的位置提取标签值，直到下一个@标记或行尾
// 例如从 `json:"province" validate:"required" @gotags: gorm:"type:varchar(50)"`
// 中提取 `json:"province" validate:"required"`
func extractTagValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	nextTag := injectTagRegexp.FindStringIndex(s)
	end := len(s)
	if nextTag != nil {
		end = nextTag[0]
	}

	return strings.TrimSpace(s[:end])
}

// extractFieldName 从proto字段定义行提取字段名
// 处理格式：[repeated|optional] type field_name = number;
// 通过定位 = 号来准确提取字段名
func extractFieldName(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return ""
	}

	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	line = strings.TrimRight(line, ";")
	line = strings.TrimSpace(line)

	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return ""
	}

	beforeEq := strings.TrimSpace(line[:eqIdx])
	fields := strings.Fields(beforeEq)
	if len(fields) < 2 {
		return ""
	}

	return fields[len(fields)-1]
}

// extractProtoJSONName 从字段行中提取 json_name 选项值
func extractProtoJSONName(line string) string {
	matches := protoJSONNameRegexp.FindStringSubmatch(line)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

// ══════════════════════════════════════════════════════════════════════════════
// Service/RPC 解析
// ══════════════════════════════════════════════════════════════════════════════

// protoRPCParseState 解析 proto 文件中 service/rpc 定义的状态
type protoRPCParseState struct {
	result       []ProtoRPC
	packageName  string
	serviceName  string
	serviceDepth int
}

// ParseProtoRPCs 解析 proto 文件中的 service/rpc 定义
func ParseProtoRPCs(filename string) ([]ProtoRPC, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	state := &protoRPCParseState{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		state.processLine(scanner.Text())
	}
	return state.result, scanner.Err()
}

// processLine 处理一行 proto service/rpc 定义
func (s *protoRPCParseState) processLine(line string) {
	trimmed := strings.TrimSpace(line)
	if isSkippableProtoLine(trimmed) {
		return
	}
	if s.tryPackage(trimmed) || s.tryServiceStart(trimmed) {
		return
	}
	s.updateServiceDepth(trimmed)
	if s.serviceName == "" {
		return
	}
	s.tryAppendRPC(trimmed)
}

// tryPackage 尝试解析 proto 包定义
func (s *protoRPCParseState) tryPackage(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "package ") {
		return false
	}
	s.packageName = extractPackageName(trimmed)
	return true
}

// tryServiceStart 尝试匹配 service 定义行
func (s *protoRPCParseState) tryServiceStart(trimmed string) bool {
	m := serviceLineRegexp.FindStringSubmatch(trimmed)
	if len(m) != 2 {
		return false
	}
	s.serviceName = m[1]
	s.serviceDepth = 1
	return true
}

// updateServiceDepth 更新 service 嵌套深度，深度归零时退出当前 service
func (s *protoRPCParseState) updateServiceDepth(trimmed string) {
	if s.serviceDepth <= 0 {
		return
	}
	s.serviceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
	if s.serviceDepth <= 0 {
		s.serviceName = ""
		s.serviceDepth = 0
	}
}

// tryAppendRPC 尝试从当前行提取 RPC 方法定义
func (s *protoRPCParseState) tryAppendRPC(trimmed string) {
	m := rpcLineRegexp.FindStringSubmatch(trimmed)
	if len(m) != 4 {
		return
	}
	s.result = append(s.result, ProtoRPC{
		PackageName:  s.packageName,
		ServiceName:  s.serviceName,
		MethodName:   m[1],
		RequestType:  shortTypeName(m[2]),
		ResponseType: shortTypeName(m[3]),
		OperationID:  s.serviceName + "_" + m[1],
	})
}

// BuildOperationRequestIndex 合并多个 proto 文件的 RPC 映射
// 返回以 OperationID 为键的 RPC 信息映射
func BuildOperationRequestIndex(protoFiles []string) (map[string]ProtoRPC, error) {
	index := make(map[string]ProtoRPC)
	for _, protoFile := range protoFiles {
		rpcs, err := ParseProtoRPCs(protoFile)
		if err != nil {
			return nil, err
		}
		for _, rpc := range rpcs {
			index[rpc.OperationID] = rpc
		}
	}
	return index, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// Go struct tag 解析辅助函数
// ══════════════════════════════════════════════════════════════════════════════

// parseGoTagString 解析Go struct tag字符串为key-value映射
// 例如 `json:"name" validate:"required"` → {"json":"name", "validate":"required"}
func parseGoTagString(tagStr string) map[string]string {
	result := make(map[string]string)
	tagStr = strings.TrimSpace(tagStr)
	for len(tagStr) > 0 {
		tagStr = strings.TrimSpace(tagStr)
		if tagStr == "" {
			break
		}
		key, value, rest, ok := parseNextGoTagPair(tagStr)
		if ok {
			result[key] = value
		}
		if rest == "" {
			break
		}
		tagStr = rest
	}
	return result
}

// parseNextGoTagPair 解析下一个 Go struct tag 键值对
func parseNextGoTagPair(tagStr string) (key, value, rest string, ok bool) {
	spaceIdx := strings.Index(tagStr, " ")
	colonIdx := strings.Index(tagStr, ":")
	if colonIdx < 0 || (spaceIdx >= 0 && spaceIdx < colonIdx) {
		if spaceIdx < 0 {
			return "", "", "", false
		}
		return "", "", strings.TrimSpace(tagStr[spaceIdx+1:]), false
	}
	key = tagStr[:colonIdx]
	remainder := tagStr[colonIdx+1:]
	if len(remainder) > 0 && remainder[0] == '"' {
		value, rest = parseQuotedGoTagValue(remainder)
		return key, value, rest, true
	}
	value, rest = parseUnquotedGoTagValue(remainder)
	return key, value, rest, true
}

// parseQuotedGoTagValue 解析被引号包裹的 Go struct tag 值
func parseQuotedGoTagValue(tagStr string) (value, rest string) {
	endQuote := strings.Index(tagStr[1:], "\"")
	if endQuote >= 0 {
		return tagStr[1 : endQuote+1], tagStr[endQuote+2:]
	}
	return tagStr[1:], ""
}

// parseUnquotedGoTagValue 解析未被引号包裹的 Go struct tag 值
func parseUnquotedGoTagValue(tagStr string) (value, rest string) {
	if end := strings.Index(tagStr, " "); end >= 0 {
		return tagStr[:end], tagStr[end+1:]
	}
	return tagStr, ""
}
