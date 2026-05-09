# protoc-go-inject-tag

从 proto 文件的 `@gotags`或者`@inject_tags` 注释中提取标签，注入到生成的 `.pb.go` 文件中,支持 json、validate、gorm、bson 等所有 Go 标签类型

> **注意：** 此工具会修改 protoc 生成的 `.pb.go` 文件（带有 `DO NOT EDIT` 警告）；这是为了在 protobuf 生成的代码中添加自定义标签，用于验证、ORM 等场景；每次重新生成 `.pb.go` 文件后需要重新运行此工具

## 特性

- ✅ 自动清理多余的 `@gotags` `@inject_tags` 注释
- ✅ 自动格式化生成的代码
- ✅ 支持批量处理和递归匹配（`**`）
- ✅ 试运行模式（dry-run）
- ✅ Windows 路径兼容
- ✅ Swagger 文档后处理：从 proto `@inject_tag`/`@gotags` 注入 validate 约束到 swagger schema

## 安装

```bash
git clone https://github.com/kamalyes/protoc-go-inject-tag.git
cd protoc-go-inject-tag
go build -o protoc-go-inject-tag.exe main.go  # Windows
go build -o protoc-go-inject-tag main.go      # Linux/Mac

# 或安装到 GOPATH/bin
go install
```

## 快速开始

```bash
# 1. 生成 protobuf 代码
protoc --go_out=. --go_opt=paths=source_relative example.proto

# 2. 注入标签（修改生成的 .pb.go 文件）
protoc-go-inject-tag -i example.pb.go -v

# 推荐
$ts = Get-Date -Format "yyyyMMddHHmmss"; go build -ldflags "-X github.com/kamalyes/protoc-go-inject-tag/bootstrap.Version=$ts" -o protoc-go-inject-tag.exe; .\protoc-go-inject-tag.exe -i="examples/*.pb.go" -v
```

> **工作流提示：** 建议将标签注入步骤集成到构建脚本中，这样每次运行 `protoc` 后自动注入标签

## Proto 标签示例

```protobuf
// 地址信息
// [EN] Address information
message Address {
  string province = 1;  // 省份 | [EN] Province // @gotags: json:"province" validate:"required,min=2,max=50" gorm:"type:varchar(50);not null"
  string city = 2;      // 城市 | [EN] City // @gotags: json:"city" validate:"required,min=2,max=50" gorm:"type:varchar(50);not null"
  string street = 3;    // 街道地址 | [EN] Street address // @gotags: json:"street" validate:"required,min=5,max=200" gorm:"type:varchar(200);not null"
  string postal_code = 4; // 邮政编码 | [EN] Postal code // @gotags: json:"postal_code" validate:"omitempty,len=6,numeric" gorm:"type:varchar(10)"
  bool is_default = 5;  // 是否默认地址 | [EN] Is default address // @gotags: json:"is_default" gorm:"type:boolean;default:false"
}
```

**生成后：**

```go
// 地址信息
// [EN] Address information
type Address struct {
 state         protoimpl.MessageState `protogen:"open.v1"`
 Province      string                 `protobuf:"bytes,1,opt,name=province,proto3" json:"province" validate:"required,min=2,max=50" gorm:"type:varchar(50);not null"`                // 省份 | [EN] Province
 City          string                 `protobuf:"bytes,2,opt,name=city,proto3" json:"city" validate:"required,min=2,max=50" gorm:"type:varchar(50);not null"`                        // 城市 | [EN] City
 Street        string                 `protobuf:"bytes,3,opt,name=street,proto3" json:"street" validate:"required,min=5,max=200" gorm:"type:varchar(200);not null"`                  // 街道地址 | [EN] Street address
 PostalCode    string                 `protobuf:"bytes,4,opt,name=postal_code,json=postalCode,proto3" json:"postal_code" validate:"omitempty,len=6,numeric" gorm:"type:varchar(10)"` // 邮政编码 | [EN] Postal code
 IsDefault     bool                   `protobuf:"varint,5,opt,name=is_default,json=isDefault,proto3" json:"is_default" gorm:"type:boolean;default:false"`                            // 是否默认地址 | [EN] Is default address
 unknownFields protoimpl.UnknownFields
 sizeCache     protoimpl.SizeCache
}
```

完整示例见 [examples/example.proto](examples/example.proto)

## 使用方法

### 命令示例

```bash
# 单个文件
protoc-go-inject-tag -i example.pb.go

# 目录中所有文件
protoc-go-inject-tag -i pb/*.pb.go

# 递归处理所有子目录（推荐）
protoc-go-inject-tag -i pb/**/*.pb.go

# 详细输出
protoc-go-inject-tag -i pb/*.pb.go -v

# 试运行（不修改文件）
protoc-go-inject-tag -i pb/*.pb.go -d
```

**Windows 注意事项：**

- 使用反斜杠：`pb\**\*.pb.go`
- 不要用引号包裹路径
- 使用 `-i` 而不是 `--input`

### 命令行选项

| 选项                | 简写 | 默认值 | 说明                                 |
| ------------------- | ---- | ------ | ------------------------------------ |
| `--input`           | `-i` | (必需) | 输入文件模式，支持 glob 和 `**` 递归 |
| `--verbose`         | `-v` | false  | 显示详细输出                         |
| `--remove-comments` | `-r` | true   | 移除 @gotags 注释                    |
| `--format`          | `-f` | true   | 格式化代码                           |
| `--dry-run`         | `-d` | false  | 试运行                               |

## Swagger 后处理子命令

`swagger` 子命令用于后处理 `protoc-gen-openapiv2` 生成的 swagger 文件，将 proto 中的 `@inject_tag`/`@gotags` validate 标签转换为 OpenAPI 约束（如 `required`、`minLength`、`maxLength`、`minimum`、`maximum`、`pattern`、`enum`、`format`），并从 swagger 的 `description`/`title` 中剥离 `@inject_tag`/`@gotags`  文本。

**工作原理：**

1. 解析 `.proto` 文件中的 `@inject_tag`/`@gotags` 注解，提取 `validate` 标签
2. 读取 swagger YAML/JSON 文件
3. 遍历 `definitions`，将 validate 约束转换为对应的 swagger schema 字段
4. 从 `description` 和 `title` 中剥离 `@inject_tag`/`@gotags`  文本
5. 清理未在 `paths` 中引用的顶层 `tags`
6. 写回修改后的 swagger 文件（保持原始格式）

**模块结构：**

- `types.go` — Swagger/OpenAPI v2 类型定义，参照 grpc-gateway `protoc-gen-openapiv2` 的 `types.go` 结构，所有结构体均附带规范链接注释
- `format.go` — Swagger 文件序列化辅助（YAML/JSON 读写），2 空格缩进
- `processor.go` — 核心后处理逻辑
- `proto_parser.go` — Proto 文件解析器，提取 `@inject_tag`/`@gotags`  注解
- `validate_mapper.go` — validate 标签到 Swagger 约束的映射器

**有序序列化：**
参照 grpc-gateway 的实现，使用自定义有序集合类型（`SchemaProperties`、`swaggerPathsObject`、`swaggerPathItemObject`）确保 YAML/JSON 输出保持 proto 字段定义顺序和 HTTP 方法声明顺序，不会被 Go map 的字母排序打乱。

### 命令示例

```bash
# 处理单个 swagger 文件
protoc-go-inject-tag swagger --input=./proto/access_control/*.proto --swagger=./proto/access_control/auth_service.swagger.yaml

# 处理目录下的所有 swagger 文件（推荐）
protoc-go-inject-tag swagger --proto-dir=./proto/access_control

# 详细输出
protoc-go-inject-tag swagger --proto-dir=./proto/ -v
```

### 命令行选项

| 选项          | 简写 | 默认值 | 说明                              |
| ------------- | ---- | ------ | --------------------------------- |
| `--input`     | `-i` |        | proto 文件模式（指定 proto 文件） |
| `--swagger`   | `-s` |        | 指定单个 swagger 文件路径         |
| `--proto-dir` |      |        | proto 目录（递归扫描）            |

## 代码中使用验证

```bash
go get github.com/go-playground/validator/v10
```

```go
import "github.com/go-playground/validator/v10"

validate := validator.New()
user := &pb.User{Email: "invalid"}

if err := validate.Struct(user); err != nil {
    fmt.Printf("验证失败: %v\n", err)
}
```

## 常见问题

**Q: 标签没有生效？**

检查：`@gotags:` 格式正确（双斜杠+空格）、使用双引号、在生成 protobuf 代码后运行工具使用 `-v` 查看详细日志

**Q: Windows 下找不到文件？**

```bash
# 正确 ✅
protoc-go-inject-tag -i pb\**\*.pb.go

# 错误 ❌
protoc-go-inject-tag --input="pb\*.pb.go"
```

**Q: 支持哪些标签？**

支持所有 Go 结构体标签：`json`、`validate`、`gorm`、`bson`、`yaml`、`xml` 等

## 相关链接

- [原版 protoc-go-inject-tag](https://github.com/favadi/protoc-go-inject-tag)
- [go-playground/validator](https://github.com/go-playground/validator)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
- [GORM](https://gorm.io/)
