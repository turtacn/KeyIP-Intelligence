# KeyIP-Intelligence 全页面测试数据与输入指南

> 面向 `http://192.168.99.100` (docker-machine) 环境的端到端功能验证。
> 所有数据通过 nginx API stubs 提供，无需依赖后端 Go 服务。

## 已验证数据统计

| 数据类别 | 数量 |
|---------|------|
| 分子 (Molecules) | 15 种 OLED 材料 |
| 专利 (Patents) | 5 件 (CN×3, US×1, EP×1) |
| 投资组合 (Portfolios) | 2 组 |
| 侵权警报 (Infringement) | 5 条 |
| 截止日期 (Deadlines) | 3 条 |
| 合作伙伴 (Partners) | 3 家 |
| FTO 风险 | 2 项 |
| 知识图谱节点/边 | 7 节点 / 4 边 |
| 引用网络 (per patent) | 前引 4 + 后引 2 = 6 |

---

## 各页面测试指南

### 1. 🏠 高管仪表盘

**URL**: `http://192.168.99.100/dashboard`

| 项目 | 说明 |
|------|------|
| 输入 | 无需输入 — 自动加载 KPI |
| 验证点 | 卡片显示 15 专利、12 活跃、76 分健康度 |
| 图表 | 趋势图 6 月数据、管辖区饼图 (CN 8 / US 3 / EP 2)、竞争对手雷达图 |
| 数据来源 | `GET /api/v1/dashboard/metrics` |

### 2. 🔍 全局搜索

**URL**: `http://192.168.99.100/search`

| 项目 | 说明 |
|------|------|
| 输入 | 搜索框输入 `蓝光`、`OLED`、`CBP` 等关键词 |
| 操作 | 回车搜索，结果来自 `POST /api/v1/patents/search` |
| 范围切换 | 下拉可选 `全部字段`、`标题`、`摘要` |
| 历史 | 搜索后自动记录，点击历史项可回搜 |
| 注意 | 关键词匹配 "match" 模型（中文 `蓝光` 也生效） |

### 3. ⛏️ 专利挖掘

**URL**: `http://192.168.99.100/patent-mining`

| 项目 | 说明 |
|------|------|
| 文本搜索 | 输入 `OLED`、`蓝光`、`host material` → 点击搜索 |
| 结构搜索 | 切换到 Structure Search 标签，输入 SMILES |
| 可用 SMILES | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` (CBP)、`c1ccc2c(c1)n(-c1cccc(-n3c4ccccc4c4ccccc43)c1)c2ccccc2` (mCP) |
| 数据来源 | `POST /api/v1/patents/search` |

### 4. 📡 知识图谱

**URL**: `http://192.168.99.100/knowledge-graph`

| 项目 | 说明 |
|------|------|
| Mock 模式 (默认) | 显示 7 个节点 (2 专利 + 5 分子) 的关系图 |
| 图表搜索 | 左侧搜索框输入 `CBP` 或 `CN115650927B` → 高亮节点 |
| 专利引用网络 | 右侧输入 **专利号** + 点击 Load 按钮 |
| 可用专利号 | `CN115650927B`、`US11678901B2` |
| 引用展示 | 显示前引 (forward) + 后引 (backward) 有向网络图 |
| 数据来源 | `GET /api/v1/knowledge-graph`、`GET /api/v1/patents/:id/citations` |

### 5. ⚖️ FTO 搜索

**URL**: `http://192.168.99.100/fto`

| 项目 | 说明 |
|------|------|
| 输入 | SMILES 结构式或专利号 |
| 可用 SMILES | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` (CBP 风险检测) |
| 结果 | 2 条风险记录 (HIGH: US9876543B2, MEDIUM: EP3456789A1) |
| 显示字段 | 相似度分数、权利项编号 |
| 数据来源 | `POST /api/v1/fto/search` |

### 6. 🛡️ 侵权监控

**URL**: `http://192.168.99.100/infringement-watch`

| 项目 | 说明 |
|------|------|
| 数据 | 自动加载 5 条侵权警报 |
| 风险等级 | 2 HIGH + 2 MEDIUM + 1 LOW |
| 关键关联 | Ir(ppy)3 → HIGH (相似度 0.87)、CBP → MEDIUM (0.72)、DMAC-TRZ → MEDIUM (0.68) |
| 数据来源 | `GET /api/v1/infringement/alerts`、`GET /api/v1/infringement/watch` |

### 7. 📅 生命周期控制台

**URL**: `http://192.168.99.100/lifecycle`

| 项目 | 说明 |
|------|------|
| 截止日期 | 3 条 (2026-06-15、2026-07-22、2026-08-10) |
| 生命周期事件 | 3 条 (CN115650927B: filing → publication → grant) |
| 筛选 | 可按日期范围、事件类型筛选 |
| 数据来源 | `GET /api/v1/lifecycle/deadlines`、`GET /api/v1/lifecycle/events` |

### 8. 📊 组合优化器

**URL**: `http://192.168.99.100/portfolio-optimizer`

| Tab | 内容 |
|-----|------|
| 组合全景 | 总体概览，2 组 portfolio |
| Constellation | 3 个点的散点图 (H10K 50/00 群集) |
| 竞争缺口 | 白空间分析 |
| 价值评分 | 技术 82 / 法律 71 / 商业 74 / 综合 76 |
| 预算优化 | 推荐列表 |
| 情景模拟 | 模拟面板 |
| 数据来源 | `GET /api/v1/portfolios`、`GET /api/v1/portfolios/summary`、`GET /api/v1/portfolios/scores`、`GET /api/v1/portfolios/:id/constellation`、`GET /api/v1/portfolios/:id/analysis` |

### 9. 🧪 分子详情

**URL**: `http://192.168.99.100/molecules`（列表） → 点击进入详情

| 分子 | SMILES | URL |
|------|--------|-----|
| CBP | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` | `/molecules/c0000001-0000-0000-0000-000000000001` |
| CDBP | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` | `/molecules/c0000001-0000-0000-0000-000000000002` |
| mCP | `c1ccc2c(c1)n(-c1cccc(-n3c4ccccc4c4ccccc43)c1)c2ccccc2` | `/molecules/c0000001-0000-0000-0000-000000000003` |
| TAPC | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000004` |
| TCTA | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000005` |
| α-NPD | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` | `/molecules/c0000001-0000-0000-0000-000000000006` |
| DMAC-TRZ | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000007` |
| 2CzPN | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000008` |
| Ir(ppy)3 | `c1ccc2c(c1)n(-c1cccc(-n3c4ccccc4c4ccccc43)c1)c2ccccc2` | `/molecules/c0000001-0000-0000-0000-000000000009` |
| FIrpic | `c1ccc2c(c1)n(-c1cccc(-n3c4ccccc4c4ccccc43)c1)c2ccccc2` | `/molecules/c0000001-0000-0000-0000-000000000010` |
| PO-01 | `c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1` | `/molecules/c0000001-0000-0000-0000-000000000011` |
| DCzDCN | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000012` |
| TRZ-1 | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000013` |
| TRZ-2 | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000014` |
| CBPO | `c1ccc(-n2c3ccccc3c3ccccc32)cc1` | `/molecules/c0000001-0000-0000-0000-000000000015` |
| 数据来源 | `GET /api/v1/molecules`、`GET /api/v1/molecules/search`、`GET /api/v1/molecules/:id` |

### 10. 📜 专利详情

**URL**: `http://192.168.99.100/patents/CN115650927B`

| 项目 | 说明 |
|------|------|
| 基本信息 | 标题、摘要、IPC 编码 (H10K 50/00)、权利要求 |
| 专利族 | 显示 CN/US/EP 三地同族 |
| 前引 (forward) | 4 条引用本专利的后续专利 |
| 后引 (backward) | 2 条本专利引用的先前专利 |
| 可用专利号 | `CN115650927B`、`CN114539266B`、`CN113004315B`、`US11678901B2`、`EP4089437A1` |
| 数据来源 | `GET /api/v1/patents/:id`、`GET /api/v1/patents/:id/family`、`GET /api/v1/patents/:id/citations` |

### 11. 🤝 合作伙伴

**URL**: `http://192.168.99.100/partners`

| 项目 | 说明 |
|------|------|
| 数据 | 3 个合作伙伴 |
| 类型 | 2 supplier + 1 collaborator |
| 公司 | TCI (JP)、Sigma-Aldrich (US)、Samsung Display (KR) |
| 数据来源 | `GET /api/v1/partners` |

### 12. ❤️ 系统健康

**URL**: `http://192.168.99.100/health`

| 项目 | 说明 |
|------|------|
| 服务数 | 8 个中间件: PostgreSQL、Redis、Neo4j、OpenSearch、Milvus、Kafka、MinIO、Keycloak |
| 状态 | 全部 healthy |
| 响应时间 | 各服务 ms 级 |
| 自动刷新 | 可选 10s / 30s / 60s / 5m / 关闭 |
| 数据来源 | `GET /api/v1/healthz/detail` |

### 13. ⚙️ 设置

**URL**: `http://192.168.99.100/settings`

| 项目 | 说明 |
|------|------|
| 功能 | theme (light/dark)、language (en/zh)、notifications 开关 |
| 数据来源 | `GET /api/v1/settings` |

### 14. 🔐 登录

**URL**: `http://192.168.99.100/login`

| 项目 | 说明 |
|------|------|
| 邮箱 | `turta@keyip.io` |
| 密码 | `turta123!` |
| 验证点 | 登录后侧边栏头像显示 `TU` (Turta 用户)，邮箱显示 turta@keyip.io |
| 登出 | 头像下拉菜单 → Sign Out |
| 数据来源 | `POST /api/v1/auth/signin` → JWT → `GET /api/v1/auth/me` |

---

## API Mode 说明

| Mode | 说明 | 数据来源 |
|------|------|---------|
| `mock` | 前端 MSW 内存 mock | MSW handlers (内存) |
| `proxy` **(默认)** | nginx 反向代理 | nginx stubs → apiserver |
| `live` | 生产直连 | `https://api.keyip.io` |

在 TopBar 的 API Mode 下拉中切换，切换后自动刷新页面。

---

## 已知限制

1. **分子详情 SMILES**: nginx return 指令中 `$mol_id` 不展开，详情页返回 `$mol_id` 字面值而非真实 SMILES
2. **知识图谱引用加载**: 需手动输入专利号并点击 Load，不自动加载当前节点引用
3. **Mock 模式**: 数据写死在 MSW handlers 中，数量有限

---

## 相关文档

- [开发环境搭建 SOP](../harness/01-dev-env-setup.md)
- [CDP Debug Chrome SOP](../harness/02-cdp-debug.md)
- [测试数据导入指南](../scripts/import/README.md)
