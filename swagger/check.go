/*
 * @Author: Lion-3625 lsheng758@gmail.com
 * @Date: 2026-05-18 21:39:03
 * @LastEditors: Lion-3625 lsheng758@gmail.com
 * @LastEditTime: 2026-05-18 21:46:35
 * @FilePath: \protoc-go-inject-tag\swagger\check.go
 * @Description: swagger 与 proto validate一致性检查实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckOptions swagger 与 proto validate 一致性检查配置
type CheckOptions struct {
	ProtoFiles   []string
	SwaggerFiles []string
	// BaseDir 用于输出相对路径，为空则使用绝对路径
	BaseDir string
	Verbose bool
}

// SkippedSwaggerEntry 无法审计的 swagger 文件
type SkippedSwaggerEntry struct {
	File  string
	Error string
}

// CheckReport 检查结果汇总
type CheckReport struct {
	SwaggerFileCount int
	QueryPenetration []ParameterRequiredIssue
	QueryMissing     []ParameterRequiredIssue
	PathIgnored      int
	Definitions      []DefinitionRequiredIssue
	SkippedSwagger   []SkippedSwaggerEntry
}

// HasErrors 是否存在需失败的检查项（query 穿透）
func (r *CheckReport) HasErrors() bool {
	return len(r.QueryPenetration) > 0
}

func (r *CheckReport) isClean() bool {
	return !r.HasErrors() && len(r.QueryMissing) == 0 && len(r.Definitions) == 0
}

// RunCheck 批量检查 swagger 与 proto @inject_tag 的 required 一致性
func RunCheck(opts CheckOptions) (*CheckReport, error) {
	if err := validateCheckOptions(opts); err != nil {
		return nil, err
	}

	messages, operations, err := buildCheckIndexes(opts.ProtoFiles)
	if err != nil {
		return nil, err
	}

	report := &CheckReport{SwaggerFileCount: len(opts.SwaggerFiles)}
	for _, sf := range opts.SwaggerFiles {
		audit, err := AuditSwaggerFile(sf, messages, operations)
		if err != nil {
			report.SkippedSwagger = append(report.SkippedSwagger, SkippedSwaggerEntry{File: sf, Error: err.Error()})
			continue
		}
		mergeAuditIntoReport(report, audit)
	}
	return report, nil
}

func validateCheckOptions(opts CheckOptions) error {
	if len(opts.ProtoFiles) == 0 {
		return fmt.Errorf("未指定 proto 文件")
	}
	if len(opts.SwaggerFiles) == 0 {
		return fmt.Errorf("未指定 swagger 文件")
	}
	return nil
}

func buildCheckIndexes(protoFiles []string) (map[string]map[string]string, map[string]ProtoRPC, error) {
	messages, err := BuildMessageValidateIndex(protoFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 proto 失败: %w", err)
	}
	operations, err := BuildOperationRequestIndex(protoFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("解析 RPC 失败: %w", err)
	}
	return messages, operations, nil
}

func mergeAuditIntoReport(report *CheckReport, audit AuditIssue) {
	for _, it := range audit.Parameters {
		classifyParameterIssue(report, it)
	}
	report.Definitions = append(report.Definitions, audit.Definitions...)
}

func classifyParameterIssue(report *CheckReport, it ParameterRequiredIssue) {
	switch {
	case isIgnoredPathParameter(it):
		report.PathIgnored++
	case isQueryParameter(it):
		recordQueryParameterIssue(report, it)
	}
}

func isIgnoredPathParameter(it ParameterRequiredIssue) bool {
	return it.ParamIn == "path" && it.SwaggerRequired && !it.ProtoExpected
}

func isQueryParameter(it ParameterRequiredIssue) bool {
	return it.ParamIn == "query"
}

func recordQueryParameterIssue(report *CheckReport, it ParameterRequiredIssue) {
	if it.SwaggerRequired && !it.ProtoExpected {
		report.QueryPenetration = append(report.QueryPenetration, it)
	}
	if !it.SwaggerRequired && it.ProtoExpected {
		report.QueryMissing = append(report.QueryMissing, it)
	}
}

// PrintCheckReport 输出检查报告
func PrintCheckReport(report *CheckReport, baseDir string) {
	printSkippedSwagger(report.SkippedSwagger, baseDir)
	if report.isClean() {
		printCheckSuccess(report)
		return
	}
	printQueryPenetration(report.QueryPenetration, baseDir)
	printQueryMissing(report.QueryMissing, baseDir)
	printDefinitionIssues(report.Definitions, baseDir)
}

func printSkippedSwagger(skipped []SkippedSwaggerEntry, baseDir string) {
	for _, skip := range skipped {
		fmt.Fprintf(os.Stderr, "⚠️  跳过 %s: %s\n", relPath(skip.File, baseDir), skip.Error)
	}
}

func printCheckSuccess(report *CheckReport) {
	fmt.Printf("✅ 已检查 %d 个 swagger 文件，未发现 query 穿透误标 required\n", report.SwaggerFileCount)
	if report.PathIgnored > 0 {
		fmt.Printf("ℹ️  %d 处 path 参数为 OpenAPI 固有必填（proto 未标 @inject_tag），已忽略\n", report.PathIgnored)
	}
}

func printQueryPenetration(issues []ParameterRequiredIssue, baseDir string) {
	if len(issues) == 0 {
		return
	}
	fmt.Printf("❌ query 穿透误标 required: %d 处\n\n", len(issues))
	for _, it := range issues {
		fmt.Printf("- %s | %s | %s\n", relPath(it.SwaggerFile, baseDir), it.OperationID, it.ParamName)
		fmt.Printf("  request=%s（该 message 下此字段无 validate:\"required\"）\n", it.RequestType)
	}
	fmt.Println()
}

func printQueryMissing(issues []ParameterRequiredIssue, baseDir string) {
	if len(issues) == 0 {
		return
	}
	fmt.Printf("⚠️  query 漏标 required（proto 有 validate 但 swagger 未标）: %d 处\n\n", len(issues))
	for _, it := range issues {
		fmt.Printf("- %s | %s | %s validate=%q\n", relPath(it.SwaggerFile, baseDir), it.OperationID, it.ParamName, it.ProtoValidate)
	}
	fmt.Println()
}

func printDefinitionIssues(issues []DefinitionRequiredIssue, baseDir string) {
	if len(issues) == 0 {
		return
	}
	fmt.Printf("⚠️  definition.required 不一致: %d 处\n\n", len(issues))
	for _, it := range issues {
		printDefinitionIssue(it, baseDir)
	}
}

func printDefinitionIssue(it DefinitionRequiredIssue, baseDir string) {
	fmt.Printf("- %s | %s.%s | swagger.required列表=%v proto期望=%v",
		relPath(it.SwaggerFile, baseDir), it.DefinitionName, it.FieldName, it.InRequiredList, it.ProtoExpected)
	if it.ProtoValidate != "" {
		fmt.Printf(" validate=%q", it.ProtoValidate)
	}
	fmt.Println()
}

func relPath(path, baseDir string) string {
	if baseDir == "" {
		return path
	}
	if rel, err := filepath.Rel(baseDir, path); err == nil {
		return rel
	}
	return path
}

// CollectProtoSwaggerFiles 从目录收集 .proto 与 swagger 文件
func CollectProtoSwaggerFiles(root string) (protoFiles, swaggerFiles []string, err error) {
	collected := &protoSwaggerFiles{}
	err = filepath.Walk(root, collected.walk)
	return collected.protoFiles, collected.swaggerFiles, err
}

type protoSwaggerFiles struct {
	protoFiles   []string
	swaggerFiles []string
}

func (c *protoSwaggerFiles) walk(path string, info os.FileInfo, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if info.IsDir() {
		return nil
	}
	if strings.HasSuffix(path, ".proto") {
		c.protoFiles = append(c.protoFiles, path)
		return nil
	}
	if isSwaggerFile(path) {
		c.swaggerFiles = append(c.swaggerFiles, path)
	}
	return nil
}

func isSwaggerFile(path string) bool {
	return strings.HasSuffix(path, ".swagger.yaml") ||
		strings.HasSuffix(path, ".swagger.yml") ||
		strings.HasSuffix(path, ".swagger.json")
}
