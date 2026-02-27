# 前后端接口契约一致性校验报告

## 1. 契约层解析结果 (Proto)
*   **来源**: `api/proto/v1/molecule.proto`, `api/proto/v1/patent.proto`
*   **核心定义**:
    *   `MoleculeService`: `GetMolecule`, `ListMolecules`, `SimilaritySearch`, `AssessPatentability`.
    *   `PatentService`: `GetPatent`, `ListPatents`, `SearchPatents`, `AssessInfringementRisk`.
    *   **规范**: gRPC 风格，使用 snake_case 字段，enum 定义明确（如 `MoleculeStatus`, `PatentOffice`）。

## 2. 后端一致性校验结果
*   **代码位置**: `internal/interfaces/http/handlers/`
*   **一致性问题**:
    *   **路由前缀**: 后端路由注册为 `/api/v1/molecules` 和 `/api/v1/patents`。符合一般 REST 规范，但与 Proto 的 gRPC 服务名不直接对应（预期行为）。
    *   **字段命名**:
        *   `SimilaritySearchRequest` (HTTP Handler) 使用 `Threshold` (json: `threshold`)，而 Proto 定义为 `similarity_threshold`。
        *   `SearchPatentsRequest` (HTTP Handler) 缺少 `filters` 嵌套结构，而是直接打平或部分缺失（如 Proto 支持 `ListPatentsRequest filters`，Handler 仅支持 `Query`, `Page` 等）。
    *   **功能缺失**:
        *   `PatentHandler` 中 `AnalyzeClaims`, `GetFamily`, `CheckFTO` 为 placeholder 实现 (`http.StatusNotImplemented`)。
        *   Proto 定义的 `AssessInfringementRisk` 在 HTTP Handler 中未找到直接对应的方法注册。

## 3. 前端一致性校验结果
*   **代码位置**: `web/src/services/`
*   **一致性问题**:
    *   **URL 前缀缺失 (高风险)**:
        *   `molecule.service.ts`: 请求 `/molecules`。
        *   `patent.service.ts`: 请求 `/patents`。
        *   **后端实际路由**: `/api/v1/molecules`, `/api/v1/patents`.
        *   **后果**: 接口调用将返回 404。
    *   **参数不匹配 (中风险)**:
        *   `patentService.getPatents` 发送 `searchType` 参数。
        *   后端 `SearchPatents` (HTTP) 或 `ListPatents` 均不接收 `searchType` 参数。后端 `SearchByStructure` (Molecule) 接收 `search_type`，可能是混淆。

## 4. 最终校验报告与建议

| 接口/模块 | 问题级别 | 问题描述 | 修复建议 |
| :--- | :--- | :--- | :--- |
| **全局/API Base** | **高风险** | 前端请求 URL 缺少 `/api/v1` 前缀 | 修改 `web/src/services/adapter.ts` 或各 service 文件，统一添加 `/api/v1` 前缀。 |
| **Patent/Search** | **中风险** | 前端传递多余参数 `searchType` | 检查前端业务逻辑，若无需该参数则移除；若需要，后端 `SearchPatentsRequest` 需添加对应字段。 |
| **Molecule/Similarity** | **低风险** | 字段名不一致 (`threshold` vs `similarity_threshold`) | 建议后端 JSON tag 对齐 Proto 定义，或前端适配后端现有 JSON 字段。 |
| **Patent/Infringement** | **高风险** | HTTP 接口缺失 | 在 `PatentHandler` 中实现 `AssessInfringementRisk` 并注册路由，对齐 Proto 能力。 |

**优化建议**:
1.  **自动生成**: 使用 `grpc-gateway` 或类似工具从 Proto 自动生成 HTTP Handler 代码，确保字段定义 100% 一致。
2.  **前端类型生成**: 使用 `protobuf-ts` 等工具从 Proto 生成前端 TypeScript 类型定义，避免手动维护 `types/domain.ts` 导致的偏差。
