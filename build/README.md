# 构建输出目录

本目录不纳入 Git，所有本地编译产物放在这里，不要写到项目根目录。

| 文件 | 命令 | 说明 |
|------|------|------|
| `banana-mw` | `make build` | Linux amd64，用于服务器 / Docker 手动部署 |
| `banana-mw.local` | `make build-local` | 当前系统架构，本地直接运行 |

清理：`make clean`
