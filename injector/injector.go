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
	Verbose        bool // 详细输出
	RemoveComments bool // 移除 @gotags 注释
	FormatCode     bool // 格式化代码
	DryRun         bool // 试运行
}

// Injector 标签注入器
type Injector struct {
	opts Options
}

// New 创建新的注入器
func New(opts Options) *Injector {
	return &Injector{opts: opts}
}

// ProcessFile 处理单个文件
func (inj *Injector) ProcessFile(filename string) error {
	// 读取文件
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	// 注入标签
	modified, changed, err := inj.injectTags(content)
	if err != nil {
		return fmt.Errorf("注入标签失败: %w", err)
	}

	if !changed {
		if inj.opts.Verbose {
			fmt.Printf("  文件未修改: %s\n", filename)
		}
		return nil
	}

	// 移除多余注释
	if inj.opts.RemoveComments {
		modified = inj.removeGotagsComments(modified)
	}

	// 格式化代码
	if inj.opts.FormatCode {
		formatted, err := format.Source(modified)
		if err != nil {
			if inj.opts.Verbose {
				fmt.Printf("  警告: 格式化失败，使用未格式化的代码: %v\n", err)
			}
		} else {
			modified = formatted
		}
	}

	// 试运行模式
	if inj.opts.DryRun {
		if inj.opts.Verbose {
			fmt.Printf("  [DRY RUN] 将修改文件: %s\n", filename)
		}
		return nil
	}

	// 写入文件
	if err := os.WriteFile(filename, modified, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	if inj.opts.Verbose {
		fmt.Printf("  ✓ 已更新: %s\n", filename)
	}

	return nil
}

// injectTags 注入标签到结构体字段
func (inj *Injector) injectTags(content []byte) ([]byte, bool, error) {
	// 匹配行尾的 @gotags 注释
	// 格式: FieldName Type `existing tags` // comment // @gotags: new tags
	gotagsRegex := regexp.MustCompile(`(?m)^(\s*)(\w+)\s+([^\s]+)\s+` + "`" + `([^` + "`" + `]*)` + "`" + `([^\n]*?)//\s*@(?:gotags?|inject_tags?):\s*([^\n]+)$`)

	changed := false
	result := gotagsRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		matches := gotagsRegex.FindSubmatch(match)
		if len(matches) < 7 {
			return match
		}

		indent := string(matches[1])
		fieldName := string(matches[2])
		fieldType := string(matches[3])
		existingTags := string(matches[4])
		middleComment := string(matches[5])
		newTagsStr := strings.TrimSpace(string(matches[6]))

		if inj.opts.Verbose {
			fmt.Printf("  发现字段: %s, 标签: %s\n", fieldName, newTagsStr)
		}

		// 解析现有标签
		tags, err := structtag.Parse(existingTags)
		if err != nil {
			if inj.opts.Verbose {
				fmt.Printf("  警告: 解析现有标签失败 %s: %v\n", fieldName, err)
			}
			tags = &structtag.Tags{}
		}

		// 解析新标签
		newTags, err := structtag.Parse(newTagsStr)
		if err != nil {
			if inj.opts.Verbose {
				fmt.Printf("  警告: 解析新标签失败 %s: %v\n", fieldName, err)
			}
			return match
		}

		// 合并标签（新标签覆盖旧标签）
		for _, tag := range newTags.Tags() {
			_ = tags.Set(tag)
		}

		// 生成新的字段定义（保留中间的注释）
		newLine := fmt.Sprintf("%s%s %s `%s`%s", indent, fieldName, fieldType, tags.String(), middleComment)

		changed = true
		return []byte(newLine)
	})

	return result, changed, nil
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
