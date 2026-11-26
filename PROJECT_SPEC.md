# 边载：后端契约

## 领域流程

`Sectors` 为分区及资源根，`Assets` 为记录实际常驻字节的资源，`After` 指加载前必须存在的依赖，`Gates` 描述从 `From` 到 `To` 的门。`At`、`Reach` 和路线的 `Position` 都是 0–10000 的区内整数。双向门在两侧使用同一锚点；区别于同分区内移动，换区必须由上一步靠近该方向的门。

`Route` 每项是一帧。起始驻留为空。当前区及 `Pinned` 的依赖闭包必须装下；`Retain` 帧内最近离开的分区优先于 `Lookahead` 范围内的门候选。候选分区按完整闭包接受或延期；共享的资产只占一份预算。卸载按逆拓扑，加载按正拓扑。逐条重放时内存不能超限、依赖不能悬空。

产物 `stream-plan.json` 含预算、跨区记录和逐帧 `Required`、`Resident`、`Bytes`、`Operations`、`Decisions`；`Required` 仅为当前区闭包，固定常驻资源另列于 `Pinned`。`Usage` 汇总峰值和传输字节。区域拓扑输入不含真实网格二进制。

## HTTP 接口

| 入口 | 结果 |
| --- | --- |
| `GET /healthz` | 进程就绪 |
| `GET /v1/example` | 无外部依赖的合成请求 |
| `POST /v1/build` | 新成品返回 201；已存在的相同制品返回 200；均带 `digest`、`bytes`、`artifact` 与 `Location` |
| `GET /v1/artifacts/{digest}` | 原始成品下载；支持 ETag 与字节范围；未知摘要 404，摘要不符 409 |

提交仅接受 JSON，最多 3 MiB；未知字段、多余 JSON 值 400，类型错误 415，超体积 413，领域拒绝 422，同时处理中的构建最多 2 项，繁忙返回 503。不提供鉴权和多租户隔离，部署为本地工具服务，不应直接暴露到公网。应由使用者管理归档目录容量和保留期；服务不覆盖已有摘要。归档文件系统须支持硬链接。

## 观景区画质档规划（可选模式）

当请求中出现画质档表（如 `Quality.Tiers`/`Gears`/`Vista`，或顶层 `Tiers`/`Gears`，字段名兼容多种写法）与逐帧限额时，编译入口改走按帧选档规划；不含这些字段的普通请求仍走原分区流送逻辑，产物结构不变。

- 档位数组按**从高到低**排列，每项 `{"ID","Roots"}`；分区或配置中的 `Common`/`Shared`/各档通用根对所有档生效；固定常驻仍取 `Policy.Pinned` 的依赖闭包。规划只考虑当前分区（`Sector` 或 `Route[0].Sector`）与 pinned，不做邻区预载。
- 每帧限额可取 `Limits: [...]`、逐 `Route` 项的 `Limit`/`Quota`/`Budget`、或 `PostBytes: [...]` 配合 `Policy.Budget`（限额 = 总预算 − 后处理占用）。
- 第 0 帧从空载按 `Limits[0]` 正拓扑装载。第 i≥1 帧的换档：先在旧档常驻下按正拓扑装载新档缺失依赖（每条操作后驻留 ≤ `Limits[i-1]`），全部装好后到达交接点，再按逆拓扑卸载旧档独占资源；收尾驻留（= 该档闭包 ∪ pinned）必须 ≤ `Limits[i]`。共用与 pinned 资源已驻留，不会重复 load。
- 用反向可达 DP 只保留能走完整段的档位序列，再从第 0 帧起逐帧取可行的最高档（最早差异优先高档）。未来降额会因此提前降档；升档必须等到上一帧限额容得下新旧档共存的完整交接。整段无任何序列可行时返回 422。
- 产物仍为 `stream-plan.json`：每帧沿用有序可回放的 `Operations`（先 load 后 evict）、`Required`（该档闭包）、`Resident`、`Bytes`，并额外给出 `Tier`/`TierIndex`、`Closure`、`Switched`、数值型 `HandoffIndex`/`Handoff`（`Operations` 中 load 与 evict 的分界下标；未换档为 -1）、`Timeline`（含 `Action:"handoff"` 事件）、限额与共存峰值等同义字段；顶层另有 `Handoffs`、`Tiers` 档表与 `Quality`/`Schedule` 汇总。`Usage.PeakBytes` 按每条操作后的实际驻留计算，包含新旧档共存峰值。


## 单一职责

`cmd/server` 管理配置及监听，`internal/service` 负责请求和响应，`internal/domain` 拥有领域校验、核心计算和编码，`internal/vault` 负责归档和摘要完整性。中间结果只在一次构建中有效，成功成品按内容寻址，重启后仍可通过原摘要读取。
