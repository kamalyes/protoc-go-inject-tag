/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:29:19
 * @FilePath: \protoc-go-inject-tag\swagger\proto_parser.go
 * @Description: Proto文件解析器，从.proto文件中提取@inject_tag/@gotags注解
 *
 * 支持的注解格式：
 *   - // @inject_tag: validate:"required"
 *   - // @gotags: json:"name" validate:"required,min=3"
 *   - // @inject_tags: json:"province" validate:"required" @gotags: gorm:"type:varchar(50)"
 *   - 同一行多个标签用空格分隔，每个标签独立解析
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

// FieldTag 字段的Go标签信息
type FieldTag struct {
	Validate string // validate标签值，如 "required,min=3,max=50"
	GoTags   string // 完整的原始标签字符串
}

// ProtoFieldTags 从proto文件解析出的单个字段标签信息
type ProtoFieldTags struct {
	MessageName string   // proto message名称
	FieldName   string   // 字段名称（snake_case）
	Tags        FieldTag // 解析后的标签
}

// injectTagRegexp 匹配行内所有 @inject_tag/@gotags/@inject_tags 注解
// 支持同一行出现多个注解：@inject_tags: json:"x" @gotags: validate:"required"
var injectTagRegexp = regexp.MustCompile(`@(?:inject_tag|gotags?|inject_tags?):\s*`)

// ParseProtoFile 解析proto文件，提取所有带@inject_tag注解的字段
func ParseProtoFile(filename string) ([]ProtoFieldTags, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		result       []ProtoFieldTags
		currentMsg   string
		msgBodyDepth int
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// 跳过纯注释行和空行
		if strings.HasPrefix(trimmed, "//") || trimmed == "" {
			continue
		}

		// 检测message定义开始
		if strings.HasPrefix(trimmed, "message ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentMsg = parts[1]
				msgBodyDepth = 0
			}
		}

		if currentMsg == "" {
			continue
		}

		// 跟踪message大括号深度
		msgBodyDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
		if msgBodyDepth <= 0 {
			currentMsg = ""
			continue
		}

		// 查找所有 @inject_tag / @gotags 注解位置
		tagLocs := injectTagRegexp.FindAllStringIndex(line, -1)
		if len(tagLocs) == 0 {
			continue
		}

		// 提取字段名（从注解之前的代码部分）
		codePart := line[:tagLocs[0][0]]
		fieldName := extractFieldName(codePart)
		if fieldName == "" {
			continue
		}

		// 合并同一行所有标签
		var allGoTags []string
		for _, loc := range tagLocs {
			tagValueStart := loc[1]
			tagValue := extractTagValue(line[tagValueStart:])
			if tagValue != "" {
				allGoTags = append(allGoTags, tagValue)
			}
		}

		mergedTagStr := strings.Join(allGoTags, " ")

		tags := FieldTag{
			GoTags: mergedTagStr,
		}

		// 从合并后的标签中提取validate值
		goTagParts := parseGoTagString(mergedTagStr)
		if v, ok := goTagParts["validate"]; ok {
			tags.Validate = v
		}

		result = append(result, ProtoFieldTags{
			MessageName: currentMsg,
			FieldName:   fieldName,
			Tags:        tags,
		})
	}

	return result, scanner.Err()
}

// extractTagValue 从标签注解后的位置提取标签值，直到下一个@标记或行尾
// 例如从 `json:"province" validate:"required" @gotags: gorm:"type:varchar(50)"`
// 中提取 `json:"province" validate:"required"`
func extractTagValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// 查找下一个 @inject_tag/@gotags 标记的位置（如果有）
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

	// 去掉行内注释
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	// 去掉尾部分号
	line = strings.TrimRight(line, ";")
	line = strings.TrimSpace(line)

	// 必须包含 = 号才是字段定义
	eqIdx := strings.Index(line, "=")
	if eqIdx < 0 {
		return ""
	}

	// 提取等号前的部分，字段名就是等号前最后一个单词
	beforeEq := strings.TrimSpace(line[:eqIdx])
	fields := strings.Fields(beforeEq)
	if len(fields) < 2 {
		return ""
	}

	return fields[len(fields)-1]
}

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

		spaceIdx := strings.Index(tagStr, " ")
		colonIdx := strings.Index(tagStr, ":")

		if colonIdx >= 0 && (spaceIdx < 0 || colonIdx < spaceIdx) {
			key := tagStr[:colonIdx]
			tagStr = tagStr[colonIdx+1:]

			if len(tagStr) > 0 && tagStr[0] == '"' {
				endQuote := strings.Index(tagStr[1:], "\"")
				if endQuote >= 0 {
					result[key] = tagStr[1 : endQuote+1]
					tagStr = tagStr[endQuote+2:]
				} else {
					result[key] = tagStr[1:]
					tagStr = ""
				}
			} else {
				end := strings.Index(tagStr, " ")
				if end >= 0 {
					result[key] = tagStr[:end]
					tagStr = tagStr[end+1:]
				} else {
					result[key] = tagStr
					tagStr = ""
				}
			}
		} else if spaceIdx >= 0 {
			tagStr = tagStr[spaceIdx+1:]
		} else {
			break
		}
	}

	return result
}
