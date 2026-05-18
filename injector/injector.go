/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-18 23:32:17
 * @FilePath: \protoc-go-inject-tag\injector\injector.go
 * @Description: Go结构体标签注入器，将proto @gotags注解注入到.pb.go文件的结构体字段中
 *
 * 处理流程：
 * 1. 读取 .pb.go 文件内容
 * 2. 使用正则匹配结构体字段行尾的 @gotags/@inject_tags 注释
 * 3. 解析现有标签和新标签，合并后生成新的字段定义
 * 4. 可选移除 @gotags 注释、格式化代码
 * 5. 写回修改后的文件
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package injector

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/structtag"
)

// Options 注入器选项
type Options struct {
	Verbose        bool // 详细输出模式
	RemoveComments bool // 移除 @gotags 注释
	FormatCode     bool // 格式化代码
	DryRun         bool // 试运行模式（不实际修改文件）
}

// Injector 标签注入器，负责将 @gotags 注解注入到 Go 结构体字段标签中
type Injector struct {
	opts Options
}

// New 创建新的标签注入器
func New(opts Options) *Injector {
	return &Injector{opts: opts}
}

// ProcessFile 处理单个 .pb.go 文件：注入标签 + 可选清理注释 + 可选格式化
func (inj *Injector) ProcessFile(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	modified, changed, err := inj.injectTags(content)
	if err != nil {
		return fmt.Errorf("注入标签失败: %w", err)
	}

	if !changed {
		inj.logf("  文件未修改: %s\n", filename)
		return nil
	}

	return inj.postProcessModified(filename, modified)
}

// postProcessModified 对已修改的内容执行后处理（清理注释、格式化、写入）
func (inj *Injector) postProcessModified(filename string, modified []byte) error {
	if inj.opts.RemoveComments {
		modified = inj.removeGotagsComments(modified)
	}

	modified, err := inj.formatModified(modified)
	if err != nil {
		return err
	}

	if inj.opts.DryRun {
		inj.logf("  [DRY RUN] 将修改文件: %s\n", filename)
		return nil
	}

	if err := os.WriteFile(filename, modified, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	inj.logf("  ✓ 已更新: %s\n", filename)
	return nil
}

// formatModified 格式化修改后的代码，格式化失败时保留原始内容
func (inj *Injector) formatModified(content []byte) ([]byte, error) {
	if !inj.opts.FormatCode {
		return content, nil
	}
	formatted, err := format.Source(content)
	if err != nil {
		inj.logf("  警告: 格式化失败，使用未格式化的代码: %v\n", err)
		return content, nil
	}
	return formatted, nil
}

// logf 在 verbose 模式下输出格式化日志
func (inj *Injector) logf(format string, args ...interface{}) {
	if inj.opts.Verbose {
		fmt.Printf(format, args...)
	}
}

// injectTags 注入标签到结构体字段
func (inj *Injector) injectTags(content []byte) ([]byte, bool, error) {
	gotagsRegex := regexp.MustCompile(`(?m)^(\s*)(\w+)\s+([^\s]+)\s+` + "`" + `([^` + "`" + `]*)` + "`" + `([^\n]*?)(?://\s*)?@(?:gotags?|inject_tags?):\s*([^\n]+)$`)

	changed := false
	result := gotagsRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		matches := gotagsRegex.FindSubmatch(match)
		if len(matches) < 7 {
			return match
		}
		merged, ok := inj.mergeFieldTags(matches)
		if !ok {
			return match
		}
		changed = true
		return []byte(merged)
	})

	return result, changed, nil
}

// mergeFieldTags 合并字段的新旧标签，返回新的字段定义行
func (inj *Injector) mergeFieldTags(matches [][]byte) (string, bool) {
	indent := string(matches[1])
	fieldName := string(matches[2])
	fieldType := string(matches[3])
	existingTags := string(matches[4])
	middleComment := string(matches[5])
	newTagsStr := strings.TrimSpace(string(matches[6]))

	inj.logf("  发现字段: %s, 标签: %s\n", fieldName, newTagsStr)

	tags := inj.parseExistingTags(existingTags, fieldName)
	newTags, err := structtag.Parse(newTagsStr)
	if err != nil {
		inj.logf("  警告: 解析新标签失败 %s: %v\n", fieldName, err)
		return "", false
	}

	for _, tag := range newTags.Tags() {
		_ = tags.Set(tag)
	}

	return fmt.Sprintf("%s%s %s `%s`%s", indent, fieldName, fieldType, tags.String(), middleComment), true
}

// parseExistingTags 解析现有标签，解析失败时返回空标签集
func (inj *Injector) parseExistingTags(existingTags, fieldName string) *structtag.Tags {
	tags, err := structtag.Parse(existingTags)
	if err != nil {
		inj.logf("  警告: 解析现有标签失败 %s: %v\n", fieldName, err)
		return &structtag.Tags{}
	}
	return tags
}

// removeGotagsComments 移除 @gotags 和 @inject_tags 注释
func (inj *Injector) removeGotagsComments(content []byte) []byte {
	// 移除同一行中的 @gotags/@inject_tags 部分，但保留前面的注释
	// 匹配: // 前面的注释 @gotags: 后面的内容
	gotagsInCommentRegex := regexp.MustCompile(`(?m)^(\s*//[^@]*?)\s*@(?:gotags?|inject_tags?):[^\n]*`)
	result := gotagsInCommentRegex.ReplaceAll(content, []byte("$1"))

	// 移除单独的 @gotags/@inject_tags 注释行
	gotagsLineRegex := regexp.MustCompile(`(?m)^\s*//\s*@(?:gotags?|inject_tags?):[^\n]*\n`)
	result = gotagsLineRegex.ReplaceAll(result, []byte(""))

	// 移除多余的空白行（连续超过2个空行）
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAll(result, []byte("\n\n"))

	// 移除行尾多余空格
	trailingSpaces := regexp.MustCompile(`[ \t]+\n`)
	result = trailingSpaces.ReplaceAll(result, []byte("\n"))

	return bytes.TrimSpace(result)
}
