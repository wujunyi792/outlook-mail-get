## 0.1.2 - 2026-03-20

### 文档
- 重做 README 首页展示，补充徽章、快速开始、命令总览和更完整的使用说明

## 0.1.1 - 2026-03-20

### 修复
- 将本地配置和 token cache 统一存放到 `~/.outlook-mail-get`
- 在执行命令时自动把旧版本项目目录中的 `./data/` 状态迁移到用户数据目录

## 0.1.0 - 2026-03-20

### 新功能
- 支持通过 `go install github.com/wujunyi792/outlook-mail-get@latest` 直接安装

### 重构
- 将 CLI 重构为标准的 Cobra 子命令结构，拆分 `fetch`、`accounts` 和 `auth` 流程
