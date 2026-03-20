## 0.1.0 - 2026-03-20

### 新功能
- 支持通过 `go install github.com/wujunyi792/outlook-mail-get@latest` 直接安装

### 重构
- 将 CLI 重构为标准的 Cobra 子命令结构，拆分 `fetch`、`accounts` 和 `auth` 流程
