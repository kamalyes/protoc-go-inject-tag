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

type ruleHandler func(c *SwaggerConstraints, value string)

var ruleHandlers = map[string]ruleHandler{
	"required": func(c *SwaggerConstraints, _ string) { c.Required = true },
	"min":      func(c *SwaggerConstraints, v string) { c.Min = parseFloat(v) },
	"max":      func(c *SwaggerConstraints, v string) { c.Max = parseFloat(v) },
	"len": func(c *SwaggerConstraints, v string) {
		if n := parseInt(v); n != nil {
			c.MinLength = n
			c.MaxLength = n
		}
	},
	"gte": func(c *SwaggerConstraints, v string) { c.Min = parseFloat(v) },
	"lte": func(c *SwaggerConstraints, v string) { c.Max = parseFloat(v) },
	"gt": func(c *SwaggerConstraints, v string) {
		if p := parseFloat(v); p != nil {
			*p += 0.0000001
			c.Min = p
		}
	},
	"lt": func(c *SwaggerConstraints, v string) {
		if p := parseFloat(v); p != nil {
			*p -= 0.0000001
			c.Max = p
		}
	},
	"email":    func(c *SwaggerConstraints, _ string) { c.Format = "email" },
	"url":      func(c *SwaggerConstraints, _ string) { c.Format = "uri" },
	"uri":      func(c *SwaggerConstraints, _ string) { c.Format = "uri" },
	"uuid":     func(c *SwaggerConstraints, _ string) { c.Format = "uuid" },
	"oneof":    func(c *SwaggerConstraints, v string) { c.Enum = strings.Fields(v) },
	"numeric":  func(c *SwaggerConstraints, _ string) { c.Pattern = "^[0-9]+$" },
	"alpha":    func(c *SwaggerConstraints, _ string) { c.Pattern = "^[a-zA-Z]+$" },
	"alphanum": func(c *SwaggerConstraints, _ string) { c.Pattern = "^[a-zA-Z0-9]+$" },
}

func parseFloat(s string) *float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return &v
	}
	return nil
}

func parseInt(s string) *int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return &v
	}
	return nil
}

func ParseValidateToSwagger(validateStr string) *SwaggerConstraints {
	if validateStr == "" {
		return nil
	}

	c := &SwaggerConstraints{}
	for _, rule := range splitRules(validateStr) {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		key, value := splitRule(rule)
		if handler, ok := ruleHandlers[key]; ok {
			handler(c, value)
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
