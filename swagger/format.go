/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:28:57
 * @FilePath: \protoc-go-inject-tag\swagger\format.go
 * @Description: Swagger文件序列化辅助，参照grpc-gateway format.go实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format swagger文件格式
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

// DetectFormat 根据文件扩展名检测格式
func DetectFormat(filename string) Format {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".yaml" || ext == ".yml" {
		return FormatYAML
	}
	return FormatJSON
}

// ReadSwaggerFile 读取并解析swagger文件
func ReadSwaggerFile(filename string) (*swaggerDoc, Format, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, "", fmt.Errorf("读取 swagger 文件失败: %w", err)
	}

	var doc swaggerDoc
	format := DetectFormat(filename)

	if format == FormatYAML {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, "", fmt.Errorf("解析 swagger YAML 失败: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, "", fmt.Errorf("解析 swagger JSON 失败: %w", err)
		}
	}

	return &doc, format, nil
}

// WriteSwaggerFile 将swagger文档序列化并写入文件
func WriteSwaggerFile(filename string, doc *swaggerDoc, format Format) error {
	var output []byte
	var err error

	if format == FormatYAML {
		var buf strings.Builder
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err = enc.Encode(doc); err != nil {
			enc.Close()
			return fmt.Errorf("序列化 swagger YAML 失败: %w", err)
		}
		enc.Close()
		output = []byte(buf.String())
	} else {
		output, err = json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化 swagger JSON 失败: %w", err)
		}
	}

	if err := os.WriteFile(filename, output, 0644); err != nil {
		return fmt.Errorf("写入 swagger 文件失败: %w", err)
	}

	return nil
}
