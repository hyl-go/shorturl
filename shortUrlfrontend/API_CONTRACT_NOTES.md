# 前后端接口契约与对齐说明

## 后端真实路径（无前缀 `/api`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/convert` | 生成短链 |
| GET | `/stats` | 查询统计，`query`: shortURL, startDate, endDate |
| POST | `/analyze` | AI 分析报告，JSON body 同上 |
| GET | `/links` | 管理端列表，`query`: page, pageSize |
| GET | `/:shortURL` | 302 跳转（非 JSON） |

路由注册顺序已固定：`convert`、`stats`、`analyze`、`links` 在 `/:shortURL` 之前，避免被当成短链 slug。

## 前端调用方式（开发）

- `axios` 的 `baseURL` 默认为 **`/api`**（可用环境变量 `VITE_API_BASE` 覆盖）。
- Vite `server` / `preview` 均配置了代理：`/api` → `http://127.0.0.1:8888`，并 **rewrite 去掉 `/api` 前缀**，与后端真实路径一致。
- 参考：`shortUrlfrontend/.env.example`。

短链 **跳转** 不使用 axios 调 JSON，而是由浏览器访问「短链域名 + path」；前端提供 `openShortUrlDisplay(api返回的 shortURL 字段)` 在新标签试跳。

## 已发生问题（归档）

- **400 `field "customShortURL" is not set`**：后端已为可选字段加 `optional`；前端提交 convert 时固定带上四个字段（含空字符串）。

## Convert 请求规范

- 必选：`longURL`
- 可选：`customShortURL`、`expireAt`、`enableAI`（布尔）；提交时建议始终带上键，避免 strict 校验报错。
