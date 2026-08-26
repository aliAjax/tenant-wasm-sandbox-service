# 27-wasm-sandbox-executor

多租户WASM沙箱执行服务。项目提供一个纯Go控制平面和可替换的安全模拟运行时，用于验证模块登记、摘要校验、不可变发布、资源预算、租户配额、公平队列、编译缓存隔离、执行取消、节点排空和审计结果保存。

模拟运行时只接受`examples/`中的JSON模块清单，不执行上传的本机代码。清单的`operation`可以是`echo`、`upper`、`reverse`、`hash`、`sum`或`fail`，每个模块还可以声明指令消耗、内存页、栈、延迟和输出重复次数，从而可重复地验证预算边界。

## 快速启动

```sh
go run ./cmd/server -config configs/config.yaml
```

服务默认监听HTTP `:8080`和gRPC `:9090`，Bearer token为`dev-secret`。配置支持YAML和`WASM_HTTP_ADDRESS`、`WASM_GRPC_ADDRESS`、`WASM_STATE_FILE`、`WASM_API_KEY`、`WASM_NODE_ID`、`WASM_WORKERS`、`WASM_ALLOW_UNSIGNED`环境变量覆盖。

浏览器访问 `http://localhost:8080/console` 可打开内嵌运维控制台，查看节点、队列、编译缓存和最近执行状态。控制台业务请求使用与 API 相同的 Bearer token。

```sh
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:8080/readyz
curl -fsS -H 'Authorization: Bearer dev-secret' http://localhost:8080/v1/cache
```

## 模块和执行流程

1. `POST /v1/modules`登记模块。`content`是base64编码的模拟清单；服务重新计算SHA-256摘要并拒绝不匹配的摘要。模块首先是`draft`。
2. `POST /v1/modules/{id}/publish`完成兼容性检查并切换到不可变的`published`状态。
3. `POST /v1/executions`提交调用。请求包含`tenant_id`、已发布的`module_id`、`invocation.kind`（json、binary或stream）、payload和可选预算。服务只在内存中短暂保存原始payload，执行记录仅保留请求摘要和输出摘要。
4. 公平队列按租户轮转，租约管理并发、CPU和内存配额。调用`POST /v1/executions/{id}/cancel`可取消排队或运行中的执行；超时、预算耗尽和运行时trap会返回不同`error_class`。
5. 编译缓存键为`tenant_id + runtime_version + module_digest`，撤回或运维调用`POST /v1/cache/recalculate`会删除租户分区。

主要HTTP端点：

- `POST/GET /v1/modules`、`GET /v1/modules/{id}`、`POST /v1/modules/{id}/publish`、`POST /v1/modules/{id}/withdraw`
- `POST/GET /v1/executions`、`GET /v1/executions/{id}`、`POST /v1/executions/{id}/cancel`
- `GET/PUT /v1/tenants/{id}/quota`
- `GET /v1/nodes`、`POST /v1/nodes/{id}/drain`、`POST /v1/nodes/{id}/isolate`
- `GET /healthz`、`GET /readyz`、`GET /metrics`、`GET /v1/sandbox/health`

gRPC的`sandbox.v1.ExecutionService/Execute`是双向流：客户端发送一个`google.protobuf.Struct`请求，服务端持续推送queued、running和terminal快照。契约见`api/execution.proto`；HTTP契约见`api/openapi.yaml`。

## 工程结构

`cmd/server`负责进程生命周期。`internal`按`module`、`runtime`、`execution`、`resource`、`queue`、`cache`、`tenant`、`artifact`、`callback`、`worker`领域拆分，每个领域保留domain/application/adapter/infrastructure边界。存储、签名验证、运行时、队列、配额计量、回调和时钟均通过接口构造注入。HTTP中间件提供请求ID、Bearer认证、恢复、超时、限流、请求体大小和统一错误体。

`store.Memory`是可持久化的JSON适配器，用于本地运行和重启恢复；`migrations/001_initial.sql`给出了切换关系数据库时的模块与执行表结构。`deploy/docker-compose.yaml`以非root、只读根文件系统、丢弃Linux capabilities和独立数据卷运行服务。

## 验证

```sh
./scripts/verify.sh
```

脚本执行gofmt、go test、go vet、go build并检查非测试Go源码不少于2300行。手工验收应覆盖：错误摘要、未发布模块、输出截断、低指令/内存预算、超时和取消、同一租户并发配额、不同租户缓存隔离、幂等键、排空后ready/提交行为、SIGTERM优雅停机和JSON状态文件重启恢复。

## 运行时安全边界

本项目的模拟运行时是控制面测试替身，不能证明真实WASM引擎的安全性。生产替换实现必须在独立进程或经过审计的WASM运行时中执行，并继续实现`internal/runtime/domain.Runtime`接口、资源预算、context取消和导入/环境白名单。
