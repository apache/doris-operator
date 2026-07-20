
# `dorisctl` 使用指南

> 此文档由AI生成，仅供参考。

`dorisctl` 是 Doris Operator 自带的命令行工具，用于从集群前端（FE）拉取节点元数据，帮助运维人员快速排查集群状态。它通过 FE 暴露的 MySQL 协议执行 `SHOW FRONTENDS` / `SHOW BACKENDS` 等语句，因此使用前需要确保能够以有权限的账号连接到 FE。

> 当前版本聚焦于只读能力，核心命令为 `get`。未来扩展命令时，文档会随版本更新。

## 构建与安装

仓库根目录的 `Makefile` 已内置构建流程。

- **推荐方式**：在仓库根目录执行 `make build`，可同时生成 `bin/dorisctl`、`bin/dorisoperator` 等二进制。
- **单独构建**：执行 `go build -o bin/dorisctl ./cmd/dorisctl`。

构建完成后，将 `bin/` 目录加入 `PATH`，或直接使用绝对路径运行。

## 连接前的准备与全局参数

所有子命令共享同一组全局参数，用于描述 FE 连接信息：

| 参数 | 说明 | 备注 |
| --- | --- | --- |
| `--fe-host` | FE 对外访问地址 | **必填**。可以是域名或 IP。 |
| `--query-port` | FE MySQL 协议端口 | 默认 `9030`。如果 FE 使用自定义端口需要显式指定。 |
| `--user` | 登录用户名 | **必填**。必须具备执行 `SHOW FRONTENDS/BACKENDS` 的权限。 |
| `--password` | 登录密码 | 可以通过环境变量/交互方式置入，命令行将回显。 |
| `--ssl-ca` | CA 根证书路径 | 如果 FE 启用了 TLS，则需同时提供 `--ssl-cert` 和 `--ssl-key`。 |
| `--ssl-cert` | 客户端证书路径 | |
| `--ssl-key` | 客户端私钥路径 | |

这些参数会传递给内部的 Doris 客户端（`pkg/common/cmd/util/client.go`），后者基于 `mysql` 驱动建立连接。TLS 选项存在缺一不可的约束：只要指定了 `--ssl-ca`，就必须同时提供 `--ssl-cert` 与 `--ssl-key`。

### 认证与权限小贴士

- 建议为 `dorisctl` 准备只读账号，至少授予 `SHOW FRONTENDS`、`SHOW BACKENDS` 权限。
- 若连接失败或权限不足，`dorisctl` 会直接输出驱动返回的错误信息，可据此排查网络、防火墙或账号策略问题。

## 基本用法

命令模板：

```
dorisctl [全局参数] <子命令> <资源类型> <资源标识> [命令参数]
```

- 全局参数可以放在命令任意位置，最佳实践是在子命令前显式传入。
- 当前仅实现 `get` 子命令，资源类型支持 `node`；`computegroup` 为预留关键字，暂未实现（调用会无输出）。

### `get` ——查询节点元数据

`get` 子命令用于查看单个 FE 或 BE 节点的详细状态。执行流程如下：

1. 建立到 FE 的 MySQL 连接。
2. 顺序执行 `SHOW FRONTENDS` 和 `SHOW BACKENDS`。
3. 根据传入的 `资源标识`（节点 `Host` 字段）匹配到对应记录。
4. 将结果以 JSON 形式输出到标准输出。

语法：

```
dorisctl [全局参数] get node <host> [-o <输出选项>]
```

- `<host>` 必须与 FE/BE 在 `SHOW` 结果中的 `Host` 字段完全一致。
- 如果目标节点在两类列表均不存在，将不会返回内容（也不报错）。

#### 输出控制 `-o, --output`

- 默认输出为格式化 JSON。
- 当使用 `-o custom-columns=<字段路径>` 时，可以提取指定字段：
  - 常规字段采用 `gjson` 语法，例如：`-o custom-columns=role` 输出 FE 的角色。
  - 当字段位于 BE 的标签（`Tag`，JSON 字符串）内时，可使用 `tag.<子字段>`，工具会自动展开标签 JSON。例如：

    ```bash
    dorisctl --fe-host fe.example.com \
      --user monitor --password secret \
      get node be-1.example.com \
      -o custom-columns=tag.compute_group_name
    ```

    如果标签缺失或字段不存在，将返回空行。

> 目前 `yaml` 等选项并未单独实现，传入 `-o yaml` 时与默认输出相同。

#### 示例

- **查看 FE 节点详情**

  ```bash
  dorisctl --fe-host fe-1.prod.svc.cluster.local \
    --user monitor --password ***** \
    get node fe-1.prod.svc.cluster.local
  ```

- **查询 BE 标签中的计算组信息**

  ```bash
  dorisctl --fe-host fe-1.prod.svc.cluster.local \
    --user monitor --password ***** \
    get node be-3.prod.svc.cluster.local \
    -o custom-columns=tag.compute_group_name
  ```

- **启用 TLS 访问 FE**

  ```bash
  dorisctl --fe-host fe-ssl.prod.svc.cluster.local \
    --query-port 9430 \
    --user monitor --password ***** \
    --ssl-ca /etc/doris/ca.pem \
    --ssl-cert /etc/doris/client.crt \
    --ssl-key /etc/doris/client.key \
    get node be-3.prod.svc.cluster.local
  ```

## 常见问题排查

| 场景 | 现象 | 建议处理 |
| --- | --- | --- |
| 连接失败 | 输出类似 `dial tcp: lookup ...` 或 `i/o timeout` | 检查 FE 地址/端口、防火墙或 K8s Service 是否暴露 MySQL 端口。 |
| 认证失败 | 输出 `Access denied for user` | 确认用户/密码或账号权限；若使用 LDAP/外部认证，需在 FE 侧开启相应配置。 |
| 输出为空 | 命令执行正常但无内容 | 核实 `Host` 是否与 Doris 显示字段一致，必要时先登录 FE 手动执行 `SHOW FRONTENDS`/`SHOW BACKENDS`。 |
| `custom-columns` 返回空字符串 | 字段名称不匹配 | 使用 `dorisctl ... get node <host>` 默认输出查看真实 JSON 字段，确认路径后再组合 `custom-columns`。 |

## 后续规划

- `computegroup` 资源读取逻辑目前为空壳，如需此能力可关注后续版本或自行在 `pkg/common/cmd/get/get.go` 中实现。
- 如果需要批量查询/过滤，可考虑在外层脚本结合 `dorisctl` 与 `jq`/`gjson` 等工具。

如在使用过程中遇到新的问题，欢迎在仓库 Issue 中反馈。