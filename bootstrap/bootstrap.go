/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 13:39:52
 * @FilePath: \protoc-go-inject-tag\bootstrap\bootstrap.go
 * @Description: 命令行入口与参数解析模块，负责匹配目标文件并调度标签注入流程
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/kamalyes/protoc-go-inject-tag/injector"
	"github.com/kamalyes/protoc-go-inject-tag/swagger"
	"github.com/spf13/cobra"
)

var (
	inputPattern   string
	showVersion    bool
	verbose        bool
	removeComments bool
	formatCode     bool
	dryRun         bool

	swaggerInput string
	swaggerFile  string
	swaggerProto string

	checkProtoDir string
	checkInput    string
)

// Version can be injected at build time, e.g.:
// go build -ldflags="-X 'github.com/kamalyes/protoc-go-inject-tag/bootstrap.Version=v1.2.3'"
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "protoc-go-inject-tag",
	Short: "注入自定义标签到 protobuf 生成的 Go 代码中",
	Long: `protoc-go-inject-tag 是一个用于在 protobuf 生成的 Go 代码中注入自定义标签的工具

支持的功能：
  - 从 proto 文件的 @gotags 注释中提取标签
  - 注入到生成的 .pb.go 文件的结构体字段中
  - 自动清理多余的注释
  - 格式化生成的代码
  - 支持批量处理

示例：
  protoc-go-inject-tag -input="./pb/*.pb.go"
  protoc-go-inject-tag -input="./pb/**/*.pb.go" -verbose
  protoc-go-inject-tag -input="./pb/*.pb.go" -remove-comments -format`,
	RunE: run,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&inputPattern, "input", "i", "", "输入文件模式 (必需，例如: ./pb/*.pb.go)")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "显示版本号")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细输出")
	rootCmd.Flags().BoolVarP(&removeComments, "remove-comments", "r", true, "移除 @gotags 注释 (默认: true)")
	rootCmd.Flags().BoolVarP(&formatCode, "format", "f", true, "格式化代码 (默认: true)")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "试运行，不实际修改文件")

	swaggerCmd := &cobra.Command{
		Use:   "swagger",
		Short: "处理 swagger 文件：从 proto @inject_tag 注入约束 + 剥离注释",
		Long: `swagger 子命令用于后处理 protoc-gen-openapiv2 生成的 swagger 文件

功能：
  - 从 proto 文件的 @inject_tag/@gotags 注解中解析 validate 标签
  - 将 validate 约束转换为 swagger schema（required/minLength/maxLength/minimum/maximum/pattern/enum/format）
  - 从 swagger description 中剥离 @inject_tag 文本

示例：
  protoc-go-inject-tag swagger --input=./proto/**/*.proto --swagger=./proto/**/*.swagger.yaml
  protoc-go-inject-tag swagger --input=./proto/ -s=./proto/access_control.swagger.yaml`,
		RunE: runSwagger,
	}
	swaggerCmd.Flags().StringVarP(&swaggerInput, "input", "i", "", "proto 文件模式 (必需)")
	swaggerCmd.Flags().StringVarP(&swaggerFile, "swagger", "s", "", "swagger 文件路径（指定单个文件时使用）")
	swaggerCmd.Flags().StringVar(&swaggerProto, "proto-dir", "", "proto 目录（递归扫描 .proto 文件）")
	swaggerCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细输出")
	rootCmd.AddCommand(swaggerCmd)

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "校验 swagger 与 proto @inject_tag 的 required 是否一致",
		Long: `check 子命令用于检测 swagger query 参数是否存在 validate 穿透误标

功能：
  - 按 operationId 绑定 RPC request message，避免跨 message 字段名误匹配
  - 检测 query 参数 required 与 proto validate:"required" 是否一致
  - 忽略 path 参数（OpenAPI 固有必填）

示例：
  protoc-go-inject-tag check --proto-dir=./proto/system
  protoc-go-inject-tag check --proto-dir=./proto`,
		RunE: runCheck,
	}
	checkCmd.Flags().StringVarP(&checkInput, "input", "i", "", "proto 目录或 glob（与 --proto-dir 二选一）")
	checkCmd.Flags().StringVar(&checkProtoDir, "proto-dir", "", "proto 目录（递归扫描 .proto 与 swagger）")
	checkCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细输出")
	rootCmd.AddCommand(checkCmd)
}

func run(cmd *cobra.Command, args []string) error {
	if showVersion {
		fmt.Printf("%s\n", versionText())
		return nil
	}

	if inputPattern == "" {
		return fmt.Errorf("必须指定 -input 参数")
	}

	// 查找匹配的文件
	files, err := findFiles(inputPattern)
	if err != nil {
		return fmt.Errorf("查找文件失败: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("没有找到匹配的文件: %s", inputPattern)
	}

	if verbose {
		fmt.Printf("找到 %d 个文件\n", len(files))
	}

	// 创建注入器
	inj := injector.New(injector.Options{
		Verbose:        verbose,
		RemoveComments: removeComments,
		FormatCode:     formatCode,
		DryRun:         dryRun,
	})

	// 处理每个文件
	successCount := 0
	errorCount := 0

	for _, file := range files {
		if verbose {
			fmt.Printf("处理文件: %s\n", file)
		}

		if err := inj.ProcessFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "处理文件失败 %s: %v\n", file, err)
			errorCount++
		} else {
			successCount++
		}
	}

	// 输出统计
	fmt.Printf("\n处理完成:\n")
	fmt.Printf("  成功: %d\n", successCount)
	if errorCount > 0 {
		fmt.Printf("  失败: %d\n", errorCount)
	}

	if errorCount > 0 {
		return fmt.Errorf("有 %d 个文件处理失败", errorCount)
	}

	return nil
}

func versionText() string {
	if Version != "" && Version != "dev" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision == "" {
		return Version
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified {
		return revision + "-dirty"
	}
	return revision
}

func runSwagger(cmd *cobra.Command, args []string) error {
	if swaggerInput == "" && swaggerProto == "" {
		return fmt.Errorf("必须指定 --input 或 --proto-dir 参数")
	}

	var protoFiles []string
	var err error

	if swaggerInput != "" {
		protoFiles, err = findFiles(swaggerInput)
		if err != nil {
			return fmt.Errorf("查找 proto 文件失败: %w", err)
		}
	} else {
		err = filepath.Walk(swaggerProto, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".proto") {
				protoFiles = append(protoFiles, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("遍历 proto 目录失败: %w", err)
		}
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("没有找到匹配的 proto 文件")
	}

	if verbose {
		fmt.Printf("找到 %d 个 proto 文件\n", len(protoFiles))
	}

	var swaggerFiles []string
	if swaggerFile != "" {
		swaggerFiles = []string{swaggerFile}
	} else {
		if swaggerProto != "" {
			err = filepath.Walk(swaggerProto, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (strings.HasSuffix(path, ".swagger.json") || strings.HasSuffix(path, ".swagger.yaml") || strings.HasSuffix(path, ".swagger.yml")) {
					swaggerFiles = append(swaggerFiles, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("遍历 swagger 目录失败: %w", err)
			}
		} else {
			swaggerFiles, err = findFiles(swaggerInput)
			if err != nil {
				return fmt.Errorf("查找 swagger 文件失败: %w", err)
			}
		}
	}

	if len(swaggerFiles) == 0 {
		return fmt.Errorf("没有找到 swagger 文件")
	}

	if verbose {
		fmt.Printf("找到 %d 个 swagger 文件\n", len(swaggerFiles))
	}

	proc := swagger.NewProcessor(verbose)

	errorCount := 0
	for _, sf := range swaggerFiles {
		if !strings.HasSuffix(sf, ".swagger.json") && !strings.HasSuffix(sf, ".swagger.yaml") && !strings.HasSuffix(sf, ".swagger.yml") {
			continue
		}
		if err := proc.ProcessFile(sf, protoFiles); err != nil {
			fmt.Fprintf(os.Stderr, "处理 swagger 文件失败 %s: %v\n", sf, err)
			errorCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("有 %d 个 swagger 文件处理失败", errorCount)
	}

	fmt.Printf("\n✅ swagger 注入处理完成\n")
	return nil
}

func runCheck(cmd *cobra.Command, args []string) error {
	if checkInput == "" && checkProtoDir == "" {
		return fmt.Errorf("必须指定 --input 或 --proto-dir 参数")
	}

	scanRoot := checkProtoDir
	if scanRoot == "" {
		scanRoot = checkInput
	}

	var protoFiles, swaggerFiles []string
	var err error

	if info, statErr := os.Stat(scanRoot); statErr == nil && info.IsDir() {
		base, absErr := filepath.Abs(scanRoot)
		if absErr != nil {
			return absErr
		}
		protoFiles, swaggerFiles, err = swagger.CollectProtoSwaggerFiles(base)
		if err != nil {
			return fmt.Errorf("遍历目录失败: %w", err)
		}
	} else {
		protoFiles, err = findFiles(checkInput)
		if err != nil {
			return fmt.Errorf("查找 proto 文件失败: %w", err)
		}
		if len(protoFiles) == 0 {
			return fmt.Errorf("没有找到匹配的 proto 文件: %s", checkInput)
		}
		repoRoot := findCheckBaseDir(filepath.Dir(protoFiles[0]))
		protoFiles, swaggerFiles, err = swagger.CollectProtoSwaggerFiles(filepath.Join(repoRoot, "proto"))
		if err != nil {
			return fmt.Errorf("遍历 proto 目录失败: %w", err)
		}
		scanRoot = repoRoot
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("没有找到 proto 文件")
	}
	if len(swaggerFiles) == 0 {
		return fmt.Errorf("没有找到 swagger 文件")
	}

	if verbose {
		fmt.Printf("检查 %d 个 proto 与 %d 个 swagger 文件\n", len(protoFiles), len(swaggerFiles))
	}

	report, err := swagger.RunCheck(swagger.CheckOptions{
		ProtoFiles:   protoFiles,
		SwaggerFiles: swaggerFiles,
		Verbose:      verbose,
	})
	if err != nil {
		return err
	}

	baseDir := findCheckBaseDir(scanRoot)
	swagger.PrintCheckReport(report, baseDir)

	if report.HasErrors() {
		return fmt.Errorf("发现 %d 处 query 穿透误标", len(report.QueryPenetration))
	}
	return nil
}

func findCheckBaseDir(start string) string {
	dir := start
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "proto")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// findFiles 查找匹配的文件，支持 ** 递归匹配
func findFiles(pattern string) ([]string, error) {
	// 处理 ** 递归匹配
	if strings.Contains(pattern, "**") {
		return findFilesRecursive(pattern)
	}

	// 使用标准 glob 匹配
	return filepath.Glob(pattern)
}

// findFilesRecursive 递归查找文件
func findFilesRecursive(pattern string) ([]string, error) {
	var matches []string

	// 分割路径和模式
	parts := strings.Split(filepath.ToSlash(pattern), "**")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的 ** 模式: %s", pattern)
	}

	baseDir := parts[0]
	if baseDir == "" {
		baseDir = "."
	} else {
		baseDir = strings.TrimSuffix(baseDir, "/")
	}

	filePattern := strings.TrimPrefix(parts[1], "/")

	// 递归遍历目录
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 检查文件是否匹配模式
		matched, err := filepath.Match(filePattern, filepath.Base(path))
		if err != nil {
			return err
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}
