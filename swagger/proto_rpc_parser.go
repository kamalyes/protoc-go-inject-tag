/*
 * @Author: Lion-3625 lsheng758@gmail.com
 * @Date: 2026-05-18 21:30:27
 * @LastEditors: Lion-3625 lsheng758@gmail.com
 * @LastEditTime: 2026-05-18 21:51:30
 * @FilePath: \protoc-go-inject-tag\swagger\proto_rpc_parser.go
 * @Description: proto rpc 解析器实现
 * Copyright (c) 2026 by Lion-3625, All Rights Reserved.
 */
package swagger

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// ProtoRPC 单个 RPC 方法的元信息
 type ProtoRPC struct {
	 PackageName  string
	 ServiceName  string
	 MethodName   string
	 RequestType  string
	 ResponseType string
	 OperationID  string
 }
 
 var (
	 serviceLineRegexp = regexp.MustCompile(`^service\s+(\w+)\s*\{`)
	 rpcLineRegexp     = regexp.MustCompile(`^rpc\s+(\w+)\s*\(\s*([\w.]+)\s*\)\s*returns\s*\(\s*([\w.]+)\s*\)`)
 )
 
 type protoParseState struct {
	 result       []ProtoRPC
	 packageName  string
	 serviceName  string
	 serviceDepth int
 }
 
 // ParseProtoRPCs 解析 proto 文件中的 service/rpc 定义
 func ParseProtoRPCs(filename string) ([]ProtoRPC, error) {
	 f, err := os.Open(filename)
	 if err != nil {
		 return nil, err
	 }
	 defer f.Close()
 
	 state := &protoParseState{}
	 scanner := bufio.NewScanner(f)
	 for scanner.Scan() {
		 state.processLine(scanner.Text())
	 }
	 return state.result, scanner.Err()
 }
 
 func (s *protoParseState) processLine(line string) {
	 trimmed := strings.TrimSpace(line)
	 if isSkippableProtoLine(trimmed) {
		 return
	 }
	 if s.tryPackage(trimmed) || s.tryServiceStart(trimmed) {
		 return
	 }
	 s.updateServiceDepth(trimmed)
	 if s.serviceName == "" {
		 return
	 }
	 s.tryAppendRPC(trimmed)
 }
 
 func isSkippableProtoLine(trimmed string) bool {
	 return trimmed == "" || strings.HasPrefix(trimmed, "//")
 }
 
 func (s *protoParseState) tryPackage(trimmed string) bool {
	 if !strings.HasPrefix(trimmed, "package ") {
		 return false
	 }
	 s.packageName = extractPackageName(trimmed)
	 return true
 }
 
 func (s *protoParseState) tryServiceStart(trimmed string) bool {
	 m := serviceLineRegexp.FindStringSubmatch(trimmed)
	 if len(m) != 2 {
		 return false
	 }
	 s.serviceName = m[1]
	 s.serviceDepth = 1
	 return true
 }
 
 func (s *protoParseState) updateServiceDepth(trimmed string) {
	 if s.serviceDepth <= 0 {
		 return
	 }
	 s.serviceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
	 if s.serviceDepth <= 0 {
		 s.serviceName = ""
		 s.serviceDepth = 0
	 }
 }
 
 func (s *protoParseState) tryAppendRPC(trimmed string) {
	 m := rpcLineRegexp.FindStringSubmatch(trimmed)
	 if len(m) != 4 {
		 return
	 }
	 s.result = append(s.result, ProtoRPC{
		 PackageName:  s.packageName,
		 ServiceName:  s.serviceName,
		 MethodName:   m[1],
		 RequestType:  shortTypeName(m[2]),
		 ResponseType: shortTypeName(m[3]),
		 OperationID:  s.serviceName + "_" + m[1],
	 })
 }
 
 func shortTypeName(typeName string) string {
	 if idx := strings.LastIndex(typeName, "."); idx >= 0 {
		 return typeName[idx+1:]
	 }
	 return typeName
 }
 
 // BuildOperationRequestIndex 合并多个 proto 文件的 RPC 映射
 func BuildOperationRequestIndex(protoFiles []string) (map[string]ProtoRPC, error) {
	 index := make(map[string]ProtoRPC)
	 for _, protoFile := range protoFiles {
		 rpcs, err := ParseProtoRPCs(protoFile)
		 if err != nil {
			 return nil, err
		 }
		 for _, rpc := range rpcs {
			 index[rpc.OperationID] = rpc
		 }
	 }
	 return index, nil
 }
 