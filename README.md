# 寿县古城景区营收统计系统

已部署：[营收统计系统](http://cd.libeasy.cn:8027/)。

采用 Vben Admin 5.7 / Vue 3 原生后台框架、Go 服务端、达梦 SZGCTRIP 数据库和 Casdoor 统一身份认证。界面采用原型的暖纸色、棕色侧栏、砖红与金色配色、品牌图标、九大业态与图表布局。

功能包括综合看板、营收排行榜、九业态明细、日/月/年筛选、日报新增与编辑、Excel 导入导出、CSV 明细、系统设置、对接系统配置及测试接口；新增系统管理的用户管理、角色管理、操作日志三个标签。

- 本地源码：`D:\网站备份\寿县\14营收统计系统`
- 服务目录：`/data/RevenueStatisticsSystem`
- 外部端口：8027；后端：127.0.0.1:8127
- 数据库：SZGCTRIP；Schema：REVENUESTATISTICSSYSTEM
- 运行账号：RevenueStatisticsSystem；凭据存放在服务器受限配置文件中
- [部署与维护](docs/deployment.md)
- [接口接入](docs/api.md)
- [验收记录](docs/acceptance.md)
- [系统管理升级验收](docs/system-management-20261022.md)

本机验证与构建：

```powershell
Set-Location backend
go test ./...
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go build -trimpath -ldflags='-s -w' -o ../artifacts/revenuestatistics .
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
Set-Location ../frontend
pnpm --filter @vben/web-antd typecheck
pnpm --filter @vben/web-antd build
```

真实数据库集成测试使用 `DM_TEST_DSN` 环境变量，未配置时跳过对应测试。下载依赖时使用本机代理 `http://127.0.0.1:7897`。Go 在本机交叉编译后上传产物。

原型测试数据共 14,112 条，来源标记为“原型测试数据”，覆盖 2025-11 至 2026-10。另保留三条上线验收记录。首次初始化依次执行 `deploy/schema.sql`、`deploy/seed.sql`、`deploy/20261022-system-management.sql`。既有数据库仅执行尚未应用的增量迁移；部署前按当前中央准入清单制定本地角色兼容绑定。

`scripts/prepare-prototype.cjs` 默认只重新生成测试 SQL；`--bootstrap` 会重新抽取原型资源并生成初始路由，仅用于首次初始化。运行配置、令牌和构建产物已从 Git 跟踪中排除。
