# GitHub Copilot Custom Instructions - Commit Message Skill

## Git Commit Message 生成规范

当用户请求生成 commit message 或分析 git diff 时，遵循以下规范：

### 格式规范
- **格式**: `emoji 类型(可选scope): 简短描述`
- **语言**: 简体中文
- **长度**: 标题不超过 50 字

### Emoji 映射

| 改动类型 | Emoji | 示例 |
|---------|-------|------|
| 新功能/重要更新 | 🔥 | `🔥 feat: 降低扫描未分配工单频率` |
| Bug 修复 | 🐛 | `🐛 fix: 修复客服状态日志问题` |
| 依赖更新 | 📦 | `📦 chore: 更新 go-wsc v1.0.37` |
| 代码重构 | ♻️ | `♻️ refactor: 重构 IM 服务架构` |
| 国际化 | 🌍 | `🌍 feat(i18n): 新增9国语言支持` |
| 性能优化 | ⚡ | `⚡ perf: 优化消息查询性能` |
| 文档更新 | 📝 | `📝 docs: 更新 README 文档` |
| CI/CD | 👷 | `👷 ci: 完善部署流程` |

### 生成要求

1. **分析改动**: 识别核心改动类型和影响范围
2. **生成两个版本**:
   - **简洁版**: 一行描述（小改动）
   - **详细版**: 多行列表（大改动，包含 ✨🎨🔧📝🗃️⚙️📦 等子项）

### 历史风格参考

```
🔥 feat: 发送欢迎语跟结束语带上ticket.Language
🔥 feat: 降低扫描未分配工单频率（后续改成发布订阅）
🐛 fix: 修复客服状态日志持续时间跟StartTime无法从pbmo直接获取问题
📦 chore: 更新LarkGame/im-share-proto版本
♻️ refactor: 重构IM服务核心架构，优化模块结构
```

### 触发词

当用户输入以下任意词时，自动应用此规范：
- "生成 commit"
- "commit message"
- "分析改动"
- "git diff"
- "@commit"

## 示例对话

**用户**: `@commit 分析这个改动`
```diff
modified: locales/vi.json
+ 85 new lines (Vietnamese translations)
```

**Copilot**:
```
简洁版：
🌍 feat(i18n): 新增越南语翻译文件

详细版：
🌍 feat(i18n): 新增越南语国际化支持

- ✨ 新增 locales/vi.json 越南语翻译文件
- 📝 完善所有错误提示和系统消息的越南语版本
- 🔧 支持根据 ticket.Language 自动选择越南语
```
