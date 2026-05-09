/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-09 17:29:26
 * @FilePath: \protoc-go-inject-tag\swagger\validate_mapper_test.go
 * @Description: validate标签到Swagger约束映射器单元测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package swagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseValidateToSwagger(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		assert.Nil(t, ParseValidateToSwagger(""))
	})

	t.Run("required", func(t *testing.T) {
		c := ParseValidateToSwagger("required")
		assert.True(t, c.Required)
	})

	t.Run("omitempty ignored", func(t *testing.T) {
		c := ParseValidateToSwagger("omitempty")
		assert.False(t, c.Required)
	})

	t.Run("min max", func(t *testing.T) {
		c := ParseValidateToSwagger("min=3,max=50")
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(3), *c.Min)
		assert.NotNil(t, c.Max)
		assert.Equal(t, float64(50), *c.Max)
	})

	t.Run("len", func(t *testing.T) {
		c := ParseValidateToSwagger("len=6")
		assert.NotNil(t, c.MinLength)
		assert.Equal(t, int64(6), *c.MinLength)
		assert.NotNil(t, c.MaxLength)
		assert.Equal(t, int64(6), *c.MaxLength)
	})

	t.Run("len invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("len=abc")
		assert.Nil(t, c.MinLength)
		assert.Nil(t, c.MaxLength)
	})

	t.Run("gte lte", func(t *testing.T) {
		c := ParseValidateToSwagger("gte=1,lte=100")
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(1), *c.Min)
		assert.NotNil(t, c.Max)
		assert.Equal(t, float64(100), *c.Max)
	})

	t.Run("gte invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("gte=abc")
		assert.Nil(t, c.Min)
	})

	t.Run("lte invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("lte=xyz")
		assert.Nil(t, c.Max)
	})

	t.Run("gt lt", func(t *testing.T) {
		c := ParseValidateToSwagger("gt=0,lt=10")
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(0), *c.Min)
		assert.True(t, c.ExclusiveMinimum)
		assert.NotNil(t, c.Max)
		assert.Equal(t, float64(10), *c.Max)
		assert.True(t, c.ExclusiveMaximum)
	})

	t.Run("gt invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("gt=abc")
		assert.Nil(t, c.Min)
	})

	t.Run("lt invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("lt=xyz")
		assert.Nil(t, c.Max)
	})

	t.Run("email", func(t *testing.T) {
		c := ParseValidateToSwagger("email")
		assert.Equal(t, "email", c.Format)
	})

	t.Run("url", func(t *testing.T) {
		c := ParseValidateToSwagger("url")
		assert.Equal(t, "uri", c.Format)
	})

	t.Run("uri", func(t *testing.T) {
		c := ParseValidateToSwagger("uri")
		assert.Equal(t, "uri", c.Format)
	})

	t.Run("uuid", func(t *testing.T) {
		c := ParseValidateToSwagger("uuid")
		assert.Equal(t, "uuid", c.Format)
	})

	t.Run("oneof", func(t *testing.T) {
		c := ParseValidateToSwagger("oneof=ACTIVE INACTIVE")
		assert.Equal(t, []string{"ACTIVE", "INACTIVE"}, c.Enum)
	})

	t.Run("oneof empty", func(t *testing.T) {
		c := ParseValidateToSwagger("oneof=")
		assert.Empty(t, c.Enum)
	})

	t.Run("numeric", func(t *testing.T) {
		c := ParseValidateToSwagger("numeric")
		assert.Equal(t, "^[0-9]+$", c.Pattern)
	})

	t.Run("alpha", func(t *testing.T) {
		c := ParseValidateToSwagger("alpha")
		assert.Equal(t, "^[a-zA-Z]+$", c.Pattern)
	})

	t.Run("alphanum", func(t *testing.T) {
		c := ParseValidateToSwagger("alphanum")
		assert.Equal(t, "^[a-zA-Z0-9]+$", c.Pattern)
	})

	t.Run("dive ignored", func(t *testing.T) {
		c := ParseValidateToSwagger("dive")
		assert.False(t, c.Required)
		assert.Nil(t, c.Min)
		assert.Nil(t, c.Max)
	})

	t.Run("combined required,min,max", func(t *testing.T) {
		c := ParseValidateToSwagger("required,min=2,max=50")
		assert.True(t, c.Required)
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(2), *c.Min)
		assert.NotNil(t, c.Max)
		assert.Equal(t, float64(50), *c.Max)
	})

	t.Run("invalid value ignored", func(t *testing.T) {
		c := ParseValidateToSwagger("min=abc,max=xyz")
		assert.Nil(t, c.Min)
		assert.Nil(t, c.Max)
	})

	t.Run("unknown rule ignored", func(t *testing.T) {
		c := ParseValidateToSwagger("unknown_rule")
		assert.False(t, c.Required)
		assert.Nil(t, c.Min)
		assert.Nil(t, c.Max)
		assert.Empty(t, c.Format)
		assert.Empty(t, c.Pattern)
	})

	t.Run("rule with spaces trimmed", func(t *testing.T) {
		c := ParseValidateToSwagger("required, min=2")
		assert.True(t, c.Required)
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(2), *c.Min)
	})

	t.Run("full example from Address.proto", func(t *testing.T) {
		c := ParseValidateToSwagger("required,min=2,max=50")
		assert.True(t, c.Required)
		assert.NotNil(t, c.Min)
		assert.Equal(t, float64(2), *c.Min)
		assert.NotNil(t, c.Max)
		assert.Equal(t, float64(50), *c.Max)
	})

	t.Run("postal_code validate", func(t *testing.T) {
		c := ParseValidateToSwagger("omitempty,len=6,numeric")
		assert.False(t, c.Required)
		assert.NotNil(t, c.MinLength)
		assert.Equal(t, int64(6), *c.MinLength)
		assert.NotNil(t, c.MaxLength)
		assert.Equal(t, int64(6), *c.MaxLength)
		assert.Equal(t, "^[0-9]+$", c.Pattern)
	})

	t.Run("User id validate", func(t *testing.T) {
		c := ParseValidateToSwagger("required,uuid")
		assert.True(t, c.Required)
		assert.Equal(t, "uuid", c.Format)
	})

	t.Run("User phone validate", func(t *testing.T) {
		c := ParseValidateToSwagger("omitempty,len=11,numeric")
		assert.False(t, c.Required)
		assert.NotNil(t, c.MinLength)
		assert.Equal(t, int64(11), *c.MinLength)
		assert.Equal(t, "^[0-9]+$", c.Pattern)
	})

	t.Run("minitems", func(t *testing.T) {
		c := ParseValidateToSwagger("minitems=1")
		assert.NotNil(t, c.MinItems)
		assert.Equal(t, int64(1), *c.MinItems)
		assert.Nil(t, c.MaxItems)
	})

	t.Run("maxitems", func(t *testing.T) {
		c := ParseValidateToSwagger("maxitems=10")
		assert.Nil(t, c.MinItems)
		assert.NotNil(t, c.MaxItems)
		assert.Equal(t, int64(10), *c.MaxItems)
	})

	t.Run("minitems maxitems combined", func(t *testing.T) {
		c := ParseValidateToSwagger("required,minitems=1,maxitems=5")
		assert.True(t, c.Required)
		assert.NotNil(t, c.MinItems)
		assert.Equal(t, int64(1), *c.MinItems)
		assert.NotNil(t, c.MaxItems)
		assert.Equal(t, int64(5), *c.MaxItems)
	})

	t.Run("minitems invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("minitems=abc")
		assert.Nil(t, c.MinItems)
	})

	t.Run("maxitems invalid", func(t *testing.T) {
		c := ParseValidateToSwagger("maxitems=xyz")
		assert.Nil(t, c.MaxItems)
	})

	t.Run("dive ignored", func(t *testing.T) {
		c := ParseValidateToSwagger("dive")
		assert.False(t, c.Required)
		assert.Nil(t, c.Min)
		assert.Nil(t, c.Max)
	})
}

func TestSplitRules(t *testing.T) {
	t.Run("basic split", func(t *testing.T) {
		assert.Equal(t, []string{"required", "min=2", "max=50"}, splitRules("required,min=2,max=50"))
	})

	t.Run("with leading space", func(t *testing.T) {
		assert.Equal(t, []string{"required", " min=2"}, splitRules("required, min=2"))
	})

	t.Run("empty string", func(t *testing.T) {
		assert.Empty(t, splitRules(""))
	})

	t.Run("single rule", func(t *testing.T) {
		assert.Equal(t, []string{"required"}, splitRules("required"))
	})

	t.Run("quoted comma", func(t *testing.T) {
		result := splitRules(`oneof=a,b,c`)
		assert.Equal(t, []string{"oneof=a", "b", "c"}, result)
	})

	t.Run("quoted value with comma", func(t *testing.T) {
		result := splitRules(`oneof="a,b,c"`)
		assert.Equal(t, []string{`oneof="a,b,c"`}, result)
	})

	t.Run("escape char", func(t *testing.T) {
		result := splitRules(`key="val\,ue"`)
		assert.Equal(t, []string{`key="val,ue"`}, result)
	})

	t.Run("escape without quote", func(t *testing.T) {
		result := splitRules(`required\,min`)
		assert.Equal(t, []string{"required,min"}, result)
	})

	t.Run("multiple quoted segments", func(t *testing.T) {
		result := splitRules(`a="x,y",b="z"`)
		assert.Equal(t, []string{`a="x,y"`, `b="z"`}, result)
	})

	t.Run("trailing comma", func(t *testing.T) {
		result := splitRules("required,")
		assert.Equal(t, []string{"required"}, result)
	})
}

func TestSplitRule(t *testing.T) {
	t.Run("key=value", func(t *testing.T) {
		k, v := splitRule("min=3")
		assert.Equal(t, "min", k)
		assert.Equal(t, "3", v)
	})

	t.Run("key only", func(t *testing.T) {
		k, v := splitRule("required")
		assert.Equal(t, "required", k)
		assert.Equal(t, "", v)
	})

	t.Run("key with empty value", func(t *testing.T) {
		k, v := splitRule("oneof=")
		assert.Equal(t, "oneof", k)
		assert.Equal(t, "", v)
	})

	t.Run("key with multiple equals", func(t *testing.T) {
		k, v := splitRule("key=val=ue")
		assert.Equal(t, "key", k)
		assert.Equal(t, "val=ue", v)
	})
}
