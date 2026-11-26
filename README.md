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

## 选路编译

除直接提交 `Route`（逐帧 `Visit`）外，`POST /v1/build` 还接受选路请求：

- `Signposts`：带 `ID`、`Sector`、`Position` 的路标；
- `Moves`：允许的有向移动 `{"From","To"}`（可附 `Gate`），也接受邻接表等写法；
- `Start` / `End`：起终点路标 ID（或序号、内联坐标）；
- `MaxMoves`：最多移动次数。

每次移动生成一帧，起点单独准备第 0 帧。跨区移动必须能从出发路标的位置按既有 `At`/`Reach`/`OneWay` 规则匹配到门。计划从空载开始：每帧末装齐当前分区闭包与 `Pinned`，新装入仅限当前必需资源；仍有预算时可保留已加载资源及其依赖。编译器在移动上限内联合搜索路线与驻留，取加载总字节（首载与重新装入均计费、持续驻留的共享资源只计一次）最小者，并列时优先更少移动，再按路标 ID 字典序保证确定性。无可达交付路线时返回 422；显式 `Route` 请求仍走原有编译逻辑。

## 核验

`go build ./...` 构建全部 Go 包。安装 Python 3.9+ 后运行 `python3 -B tools/verify.py all`：启动临时回环服务，用内置样例验证实际产物与拒绝边界、并发去重、重启复取和损坏拒绝，随后关闭实例。完整测试套件暂缓。详见 [PROJECT_SPEC.md](PROJECT_SPEC.md)。
