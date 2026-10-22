# 部署与维护

## 当前部署
114服务器，目录/data/RevenueStatisticsSystem；网站http://cd.libeasy.cn:8027/，后端127.0.0.1:8127，服务revenuestatistics.service。
current指向releases/20261022-site-audit，Go PID2873590保持。最新检查见[全站交互检查](site-interaction-audit-20261022.md)，上轮修复见[周期与弹窗](period-modal-fix-20261022.md)，系统管理验收见[系统管理升级](system-management-20261022.md)。
发布包SHA256：c49c7c3587c263d1d3c00482095a9122e40c04c6e6be816b506bf4d0fa4b2f76。
Go二进制SHA256：c9538c59944c4e6935f5ddd86ffeb3453ca3e332ebe9cedf1e328aceef9782f2。
manifest.json保存逐文件校验值。20261022-accepted、20261021-final等历史版本保留。

## 凭据与统一认证
config/dm-dsn、config/oidc-client-id、config/oidc-client-secret权限600，目录700，属主revenuestatistics。服务通过*_FILE读取，源码不保存凭据。
Casdoor应用app-revenuestatistics，组织sx，回调http://cd.libeasy.cn:8027/api/auth/callback；中央准入及超级权限为RevenueStatisticsSystem.Access/SuperAdmin。
中央认证维护身份、应用许可与超级用户，本地保存角色、成员与启停。普通用户取得有效本地角色后操作；多角色权限取并集。Revenue.Read为查看及导出、Revenue.Write为日报录入编辑导入、Integration.Manage为接口管理。系统管理和景区基础信息编辑由中央超级用户操作。
当前sx02绑定editor，保留升级前权限。后续新增或变更用户权限通过系统管理提交。

## 数据库升级
所有数据库操作使用DBX并核对实例SZGCTRIP与Schema REVENUESTATISTICSSYSTEM。
既有数据库执行尚未应用的deploy/20261022-system-management.sql，新建4张管理表并为AUDIT_LOG增加MODULE/DETAIL；已应用后勿重复执行。
DDL使用已核实的管理员连接；业务连接保持最小权限。本次通过增量兼容绑定保留sx02资格，同步真实最近登录与角色分开管理。
首次初始化顺序：deploy/schema.sql、deploy/seed.sql、deploy/20261022-system-management.sql，再核对有效中央账号并配置本地角色。

## 发布
Go仅在本机测试及交叉编译；前端完成typecheck/build，再运行scripts/package-release.ps1。
经MCPSSH上传至新的releases子目录并验证SHA256。首次安装使用install-release.sh；日常后端更新使用activate-release.sh，复用当前配置，仅重启revenuestatistics，健康失败自动切回前版本。
纯前端更新在核实二进制一致后原子切换current，保持业务PID及会话。保留旧散列静态资源，支持既有标签继续加载。
后端重启后用户经统一认证重新进入；验证/api/health、实际登录页面和必要业务保存。

## 回退
本轮纯前端发布保留20261022-period-modal，备份指针位于backups/20261022-before-site-audit/previous-release.txt；原子切回该版本即可，无须重启业务。
本轮发布前备份/data/RevenueStatisticsSystem/backups/20261022-before-system-management，包含受限配置、服务配置、previous-release.txt及进程信息。源码基线4458e82，备份分支backup/20261022-before-system-management。
应用回退原子切换current至20261022-accepted并重启本服务。新增表、日志、身份与角色映射保留；AUDIT_LOG新增字段可与旧程序兼容。
回退前核对新增角色及停用状态，旧版普通用户按中央准入使用查看与日报录入规则。当前真实sx02的editor权限与旧规则一致。
文本扩容迁移migration-20261021-text-capacity.sql已应用，回退应用时保留扩容字段。
