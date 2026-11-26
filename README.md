# 边载 · 拓扑和驻留编译

面向开放世界流送工程师。输入分区拓扑、场景块资源依赖、路线与常驻预算，输出可回放的分区资源加载、卸载、驻留与超预算决策表。仅提供 HTTP API，不包含前端，不依赖第三方 Go 模块。

## 运行

需要 Go 1.22 或更高版本。在本仓根目录启动：

```sh
go run ./cmd/server
```

默认监听 `127.0.0.1:8751`，归档目录默认 `.local/`。可指定 `-port` 与 `-data`，例如 `go run ./cmd/server -port 8751 -data .local`。

```sh
curl -fsS http://127.0.0.1:8751/v1/example -o request.json
curl -fsS -H 'Content-Type: application/json' --data-binary @request.json http://127.0.0.1:8751/v1/build
```

取返回 JSON 里的 `artifact` 路径，向同一服务发送 GET，获得 `stream-plan.json`。输出摘要是交付原始字节的 SHA-256。

## 租约引用与运行时回放

过场可能在换区后继续引用上一分区的人物模型、骨骼等资源。构建请求可附带 `Leases`，每份租约声明唯一 `Credential`（也接受 `Token`/`ID` 等写法）、可取得的 `Frame`、资源根 `Roots` 与最晚持有帧 `Until`：

```json
"Leases": [
  {"Credential": "cue-bank-bridge", "Frame": 2, "Roots": ["bank-mesh"], "Until": 3}
]
```

带租约的成品会额外内嵌 `Leases`、规范化后的 `Assets`/`Sectors`/`Gates`/`Policy` 以及一份名义 `Replay`，因此仅凭下载的 `stream-plan.json` 即可回放；未声明租约的请求成品与旧格式逐字节一致。

联调时把实机逐帧的取得/结束事件发给回放入口（均为同一实现的别名）：

- `POST /v1/replay`，体：`{"Digest":"<digest>","Events":[{"Frame":2,"Credential":"cue-bank-bridge","Type":"acquire"}]}`
- `POST /v1/artifacts/{digest}/replay`（事件可直接放 `Events` 或按帧放入 `Frames`）
- 有状态逐帧会话：`POST /v1/replay/sessions` 建会话，再逐次 `POST /v1/replay/sessions/{id}/frames`

每帧先令 `Until < 当前帧` 的租约到期，备齐当前区闭包与 `Pinned`，再按接收顺序处理事件，最后用既有 `Retain`/`Lookahead` 优先级分配剩余容量。取得准入会假设新租约与全部已生效租约都持有到各自 `Until`，逐帧检查从取得帧到新租约 `Until`（含首尾）的预算；任一帧容不下即在该次取得返回 `Accepted:false`、`Rejected:true` 与最早 `ConflictFrame`，且不改变此前的引用与驻留。不同凭证共享资源按并集只计一份；同一凭证只成功取得一次；提前结束或到期后的重复结束都是幂等空操作，不会释放其他凭证；同帧“先取得新凭证、再结束旧凭证”不会卸载仍被持有的资源。响应逐项给出事件结果（`Results`），逐帧给出可执行 `Operations` 与实际 `Resident`/`Bytes`，每条动作后依赖与容量约束均成立。`GET /v1/example/leases` 提供带租约的样例请求。


## 核验

`go build ./...` 构建全部 Go 包。安装 Python 3.9+ 后运行 `python3 -B tools/verify.py all`：启动临时回环服务，用内置样例验证实际产物与拒绝边界、并发去重、重启复取和损坏拒绝，随后关闭实例。完整测试套件暂缓。详见 [PROJECT_SPEC.md](PROJECT_SPEC.md)。
