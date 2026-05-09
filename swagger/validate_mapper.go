/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 21:17:27
 * @FilePath: \protoc-go-inject-tag\swagger\validate_mapper.go
 * @Description: validate标签到Swagger约束的映射器
 *
 * 将Go struct validate标签规则转换为OpenAPI/Swagger schema约束字段
 * 支持的validate规则：required, min, max, len, gte, lte, gt, lt, email, url, uri, uuid, oneof, numeric, alpha, alphanum, dive, omitempty
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"strconv"
	"strings"
)

// SwaggerConstraints 从validate标签解析出的Swagger约束集合
type SwaggerConstraints struct {
	Required  bool     // 是否必填
	MinLength *int64   // 字符串最小长度
	MaxLength *int64   // 字符串最大长度
	Min       *float64 // 数值最小值
	Max       *float64 // 数值最大值
	Pattern   string   // 正则表达式模式
	Enum      []string // 枚举值列表
	Format    string   // Swagger格式（email/uri/uuid等）
	MinItems  *int64   // 数组最小元素数
	MaxItems  *int64   // 数组最大元素数
}

// ParseValidateToSwagger 将validate标签字符串解析为SwaggerConstraints
// 例如 "required,min=3,max=50" → {Required:true, Min:3, Max:50}
func ParseValidateToSwagger(validateStr string) *SwaggerConstraints {
	if validateStr == "" {
		return nil
	}

	c := &SwaggerConstraints{}
	rules := splitRules(validateStr)

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		key, value := splitRule(rule)

		switch key {
		case "required":
			c.Required = true
		case "omitempty":
			// 忽略，不影响swagger约束
		case "min":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				c.Min = &v
			}
		case "max":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				c.Max = &v
			}
		case "len":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				c.MinLength = &v
				c.MaxLength = &v
			}
		case "gte":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				c.Min = &v
			}
		case "lte":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				c.Max = &v
			}
		case "gt":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				v += 0.0000001
				c.Min = &v
			}
		case "lt":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				v -= 0.0000001
				c.Max = &v
			}
		case "email":
			c.Format = "email"
		case "url", "uri":
			c.Format = "uri"
		case "uuid":
			c.Format = "uuid"
		case "oneof":
			c.Enum = strings.Fields(value)
		case "numeric":
			c.Pattern = "^[0-9]+$"
		case "alpha":
			c.Pattern = "^[a-zA-Z]+$"
		case "alphanum":
			c.Pattern = "^[a-zA-Z0-9]+$"
		case "dive":
			// 仅对数组元素生效，顶层忽略
		}
	}

	return c
}

// splitRules 将validate标签按逗号拆分为独立规则（支持引号内逗号）
// 例如 "required,min=2,max=50" → ["required", "min=2", "max=50"]
func splitRules(validateStr string) []string {
	var rules []string
	var current strings.Builder
	inQuote := false
	escapeNext := false

	for _, ch := range validateStr {
		if escapeNext {
			current.WriteRune(ch)
			escapeNext = false
			continue
		}
		if ch == '\\' {
			escapeNext = true
			continue
		}
		if ch == '"' {
			inQuote = !inQuote
			current.WriteRune(ch)
			continue
		}
		if ch == ',' && !inQuote {
			rules = append(rules, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		rules = append(rules, current.String())
	}
	return rules
}

// splitRule 将单条规则拆分为key和value（按首个=号分割）
// 例如 "min=3" → ("min", "3"), "required" → ("required", "")
func splitRule(rule string) (string, string) {
	if idx := strings.Index(rule, "="); idx >= 0 {
		return rule[:idx], rule[idx+1:]
	}
	return rule, ""
}
