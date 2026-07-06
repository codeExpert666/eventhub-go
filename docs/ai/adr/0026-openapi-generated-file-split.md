# ADR-0026 OpenAPI Generated File Split

## 标题
OpenAPI generated code 按 models 与 server wrapper 拆分为两个文件

## 状态
- accepted

## 背景
Go 版 EventHub 已经采用 spec-first OpenAPI，并通过 `oapi-codegen v2.5.0` 生成 `models`、`chi-server` 和 `strict-server`。

strict-server 迁移后，单个 `api/openapi/gen/eventhub.gen.go` 同时包含 schema model、request/response model、chi wrapper、strict server interface、strict request/response object 和 generated response writer。随着 API 数量增长，这个单文件已经接近两千行，可读性下降。

本次目标是改善 generated code 的阅读边界，但不能改变 API 契约、runtime route source-of-truth、handler -> service -> repository -> sqlc/database 分层，也不能把 generated model 下沉到 service/domain/repository。

## 决策
选择继续使用 `oapi-codegen v2.5.0`，但把生成配置拆为两个文件：

- `api/openapi/oapi-codegen.models.yaml`
  - 只启用 `generate.models`
  - 输出 `api/openapi/gen/models.gen.go`
- `api/openapi/oapi-codegen.server.yaml`
  - 启用 `generate.chi-server` 和 `generate.strict-server`
  - 输出 `api/openapi/gen/server.gen.go`

两个生成文件保持同一个 Go package `gen`。业务代码继续 import `eventhub-go/api/openapi/gen`，不感知物理文件名变化。

`make openapi-generate` 串行执行两次 `oapi-codegen`，只生成当前的 `models.gen.go` 与 `server.gen.go`。`make openapi-check` 继续保持 validate、generate 和 generated diff 的职责，diff 范围只覆盖这两个当前 generated 输出文件；旧 `eventhub.gen.go` 从仓库中删除，不再作为 Makefile 配置项保留。

新增 OpenAPI policy test 固化生成文件布局：

- `models.gen.go` 必须存在。
- `server.gen.go` 必须存在。
- `eventhub.gen.go` 不得残留。

## 备选方案
- 方案 1：继续单文件输出，只在 README 或阅读指南中说明阅读顺序。
- 方案 2：使用 include/exclude tags 按 Auth/User/Admin/System 等业务模块多次生成。
- 方案 3：拆分 `api/openapi/eventhub.yaml`，再通过 import mapping / self mapping 组合 shared schemas。
- 方案 4：使用自定义 oapi-codegen templates 输出更细粒度文件。

## 决策理由
- 两文件拆分直接解决当前最明显的阅读问题：schema model 与 runtime server glue 混在一个文件里。
- 同 package 双文件不会改变 import path、类型名、handler 使用方式或 runtime router 行为。
- `oapi-codegen v2.5.0` 已支持按生成类别分别输出；本方案不依赖未成熟的按 tag 自动拆 strict interface 能力。
- 当前 spec 规模还不需要拆分源 YAML；过早拆 spec 会增加 Redocly lint、kin-openapi validate、oasdiff breaking check 和文档维护复杂度。
- 自定义 templates 会引入生成器升级负担，当前收益不足。

## 影响
- 好处
  - `models.gen.go` 和 `server.gen.go` 的职责更清楚。
  - 新读者可以先看 generated model，再看 generated router/strict wrapper。
  - 旧单文件残留会被 policy test 捕获。
- 代价
  - `make openapi-generate` 需要运行两次 oapi-codegen。
  - `server.gen.go` 仍会因为 strict request/response object 较多而偏长。
  - 文档和 parity matrix 中关于 generated file 的索引需要从单文件更新为 generated package / 双文件。
- 后续可能需要调整的地方
  - 如果未来 API 数量显著增长，可以重新评估按 spec domain 拆分。
  - 如果 oapi-codegen 后续原生支持稳定的按 tag 多文件输出，可以再评估迁移。
