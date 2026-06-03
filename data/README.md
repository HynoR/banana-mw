# 运行时数据目录

本目录不纳入 Git，用于存放实际配置及后续可能增加的持久化文件。

## 首次使用

在项目根目录执行：

```sh
cp config.example.yaml data/config.yaml
```

然后编辑 `data/config.yaml`（至少将 `upstream` 改为真实上游地址）。

若已有私有部署约定（例如与外部面板共用的 Redis key 前缀），可参考同目录下的 `private.defaults.yaml`，把其中字段合并进 `config.yaml`。该文件与 `config.yaml` 一样不会提交到 Git。

Docker Compose 会将整个 `data/` 只读挂载到容器内 `/app/data/`。
