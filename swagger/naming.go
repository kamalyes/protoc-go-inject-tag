/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-13 23:33:52
 * @FilePath: \protoc-go-inject-tag\swagger\naming.go
 * @Description: Swagger命名策略 - 用于生成 OpenAPI 规范中的名称，如模型、操作、参数等的名称
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import "strings"

// openAPISchemaNameCandidates 生成 OpenAPI schema 名称的候选列表
// 包含原始名称、包名前缀组合等多种形式，用于匹配 swagger definition
func openAPISchemaNameCandidates(packageName, messageName string) []string {
	if messageName == "" {
		return nil
	}

	candidates := []string{messageName}
	packageName = strings.Trim(packageName, ".")
	if packageName == "" {
		return uniqueStrings(candidates)
	}

	parts := strings.Split(packageName, ".")
	for start := len(parts) - 1; start >= 0; start-- {
		candidates = append(candidates, legacyOpenAPIName(parts[start:], messageName))
	}
	for start := len(parts) - 1; start >= 0; start-- {
		candidates = append(candidates, strings.Join(append(append([]string{}, parts[start:]...), messageName), "."))
	}
	candidates = append(candidates, packageName+"."+messageName)

	return uniqueStrings(candidates)
}

// legacyOpenAPIName 生成 grpc-gateway 风格的 OpenAPI 名称（包名部分首字母大写拼接）
func legacyOpenAPIName(packageParts []string, messageName string) string {
	components := append(append([]string{}, packageParts...), messageName)
	firstNonEmpty := -1
	for i, component := range components {
		if component != "" {
			firstNonEmpty = i
			break
		}
	}
	for i := firstNonEmpty + 1; i >= 0 && i < len(components); i++ {
		if components[i] == strings.ToLower(components[i]) {
			components[i] = upperFirstASCII(components[i])
		}
	}
	return strings.Join(components, "")
}

// fieldNameCandidates 生成字段名称的候选列表，用于匹配 swagger property
// 包含原始名、jsonCamelCase 名、json tag 名等
func fieldNameCandidates(field ProtoFieldTags) []string {
	candidates := []string{field.FieldName, jsonCamelCase(field.FieldName)}
	if field.JSONName != "" {
		candidates = append(candidates, field.JSONName)
	}
	if jsonTag := parseGoTagString(field.Tags.GoTags)["json"]; jsonTag != "" {
		name := strings.Split(jsonTag, ",")[0]
		if name != "" && name != "-" {
			candidates = append(candidates, name)
		}
	}
	return uniqueStrings(candidates)
}

// jsonCamelCase 将 snake_case 转换为 lowerCamelCase
func jsonCamelCase(s string) string {
	var b []byte
	wasUnderscore := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '_' {
			if wasUnderscore && 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b = append(b, c)
		}
		wasUnderscore = c == '_'
	}
	return string(b)
}

// upperFirstASCII 将字符串首字母转为大写
func upperFirstASCII(s string) string {
	if s == "" {
		return s
	}
	if 'a' <= s[0] && s[0] <= 'z' {
		return string(s[0]-('a'-'A')) + s[1:]
	}
	return s
}

// uniqueStrings 去除字符串切片中的重复项和空值
func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}
