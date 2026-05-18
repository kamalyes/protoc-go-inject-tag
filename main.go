/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-18 23:30:00
 * @FilePath: \protoc-go-inject-tag\main.go
 * @Description: protoc-go-inject-tag 命令行入口
 *
 * 功能：将 proto 文件中的 @gotags/@inject_tag 注解注入到生成的 Go 结构体字段标签中，
 * 并支持将 validate 约束映射到 Swagger/OpenAPI schema。
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package main

import (
	"fmt"
	"os"

	"github.com/kamalyes/protoc-go-inject-tag/bootstrap"
)

func main() {
	if err := bootstrap.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
