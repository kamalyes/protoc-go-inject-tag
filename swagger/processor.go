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
var injectTagInDescRegex = regexp.MustCompile(`\s*@(?:inject_tag|gotags?|inject_tags?):\s*[^\n]+`)

// Processor swagger后处理器
type Processor struct {
	verbose bool
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

	tagMap := make(map[string]map[string]FieldTag)
	for _, protoFile := range protoFiles {
		tags, err := ParseProtoFile(protoFile)
		if err != nil {
			return fmt.Errorf("解析 proto 文件失败 %s: %w", protoFile, err)
		}
		for _, t := range tags {
			if tagMap[t.MessageName] == nil {
				tagMap[t.MessageName] = make(map[string]FieldTag)
			}
			tagMap[t.MessageName][t.FieldName] = t.Tags
			if p.verbose {
				fmt.Printf("  发现 %s.%s → validate:%q\n", t.MessageName, t.FieldName, t.Tags.Validate)
			}
		}
	}

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

		msgTags, ok := tagMap[defName]
		if !ok {
			if p.stripInjectTagFromSchema(schema.Properties) {
				modified = true
			}
			continue
		}

		for _, entry := range *schema.Properties {
			propName := entry.Key
			prop := entry.Value
			if prop == nil {
				continue
			}

			oldDesc := prop.Description
			prop.Description = injectTagInDescRegex.ReplaceAllString(prop.Description, "")
			prop.Description = strings.TrimSpace(prop.Description)
			if prop.Description != oldDesc {
				modified = true
			}
			oldTitle := prop.Title
			prop.Title = injectTagInDescRegex.ReplaceAllString(prop.Title, "")
			prop.Title = strings.TrimSpace(prop.Title)
			if prop.Title != oldTitle {
				modified = true
			}

			if ft, found := msgTags[propName]; found && ft.Validate != "" {
				constraints := ParseValidateToSwagger(ft.Validate)
				if constraints != nil {
					p.applyConstraints(prop, constraints)
					if constraints.Required {
						foundReq := false
						for _, r := range schema.Required {
							if r == propName {
								foundReq = true
								break
							}
						}
						if !foundReq {
							schema.Required = append(schema.Required, propName)
						}
					}
					modified = true
					if p.verbose {
						fmt.Printf("  ✅ %s.%s → 已注入约束\n", defName, propName)
					}
				}
			}
		}
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

// applyConstraints 将SwaggerConstraints应用到swagger属性上
func (p *Processor) applyConstraints(prop *swaggerProperty, c *SwaggerConstraints) {
	if c.MinLength != nil {
		prop.MinLength = c.MinLength
	}
	if c.MaxLength != nil {
		prop.MaxLength = c.MaxLength
	}
	if c.Min != nil {
		prop.Minimum = c.Min
	}
	if c.Max != nil {
		prop.Maximum = c.Max
	}
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
}
