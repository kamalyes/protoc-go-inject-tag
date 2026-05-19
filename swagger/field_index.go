package swagger

import "strings"

// protoFieldIndex 全量 proto 字段索引（含类型），用于嵌套 query 参数校验
type protoFieldIndex struct {
	byMessage map[string]map[string]ProtoFieldTags
}

// BuildProtoFieldIndex 从 proto 文件构建字段索引
func BuildProtoFieldIndex(protoFiles []string) (*protoFieldIndex, error) {
	idx := &protoFieldIndex{byMessage: make(map[string]map[string]ProtoFieldTags)}
	for _, pf := range protoFiles {
		fields, err := ParseProtoAllFields(pf)
		if err != nil {
			return nil, err
		}
		idx.merge(fields)
	}
	return idx, nil
}

func (idx *protoFieldIndex) merge(fields []ProtoFieldTags) {
	for _, f := range fields {
		if idx.byMessage[f.MessageName] == nil {
			idx.byMessage[f.MessageName] = make(map[string]ProtoFieldTags)
		}
		idx.byMessage[f.MessageName][f.FieldName] = f
	}
}

func (idx *protoFieldIndex) lookup(messageName, fieldName string) (ProtoFieldTags, bool) {
	if idx == nil || messageName == "" || fieldName == "" {
		return ProtoFieldTags{}, false
	}
	fields, ok := idx.byMessage[messageName]
	if !ok {
		return ProtoFieldTags{}, false
	}
	for _, name := range parameterNameCandidates(fieldName) {
		if f, found := fields[name]; found {
			return f, true
		}
	}
	return ProtoFieldTags{}, false
}

// childOmitsNestedRequired 嵌套 query 子字段是否声明 swagger:"omitrequired"（跳过父级 required 传播）
func (idx *protoFieldIndex) childOmitsNestedRequired(requestType, parent, child string) bool {
	if idx == nil || parent == "" || child == "" {
		return false
	}
	parentField, ok := idx.lookup(requestType, parent)
	if !ok {
		return false
	}
	nestedMsg := messageNameFromFieldType(parentField.FieldType)
	if nestedMsg == "" {
		return false
	}
	childField, ok := idx.lookup(nestedMsg, child)
	if !ok {
		return false
	}
	return childField.Tags.SwaggerOmitRequired
}

// nestedChildValidate 返回嵌套子字段自身的 validate（不含父级传播）
func (idx *protoFieldIndex) nestedChildValidate(requestType, parent, child string) string {
	if idx == nil {
		return ""
	}
	parentField, ok := idx.lookup(requestType, parent)
	if !ok {
		return ""
	}
	nestedMsg := messageNameFromFieldType(parentField.FieldType)
	if nestedMsg == "" {
		return ""
	}
	childField, ok := idx.lookup(nestedMsg, child)
	if !ok {
		return ""
	}
	return childField.Tags.Validate
}

func messageNameFromFieldType(fieldType string) string {
	return shortTypeName(strings.TrimSpace(fieldType))
}
