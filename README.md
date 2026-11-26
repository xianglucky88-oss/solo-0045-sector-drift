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

## 显存堆模式

请求在 `Policy` 上提供 `"Heap": {"Capacity": 128}` 即启用单块连续显存堆；容量为 0 时回退用 `Budget`，不提供 `Heap` 的请求保持原有字节预算行为。资源字段可声明：

- `Align`：2 的幂对齐，缺省为 1
- `FixedOffset`：永久固定地址，不可搬移、卸载不回收
- `FirstOffset`：仅约束首帧那次装入的偏移，资源必须在帧 0 必需闭包内

堆模式产物逐帧给出带地址的 `load`（`To`）、`move`（`From`/`To`）、`evict`（`From`）动作和帧末 `Layout`；搬移始终先复制到完整、空闲、对齐且与源不相交的区间再释放原址。必需资源无法在某帧合法交接时，编译报出首个失败帧并拒绝生成成品。计划在发布前会从空堆完整重放校验。

## 核验

`go build ./...` 构建全部 Go 包。安装 Python 3.9+ 后运行 `python3 -B tools/verify.py all`：启动临时回环服务，用内置样例验证实际产物与拒绝边界、并发去重、重启复取和损坏拒绝，随后关闭实例。完整测试套件暂缓。详见 [PROJECT_SPEC.md](PROJECT_SPEC.md)。
