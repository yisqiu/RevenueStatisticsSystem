# 项目上下文

## 当前目标
原型复刻营收统计系统，Vben Admin 5.7/Vue3 + Go + 达梦 SZGCTRIP/REVENUESTATISTICSSYSTEM，统一认证，8027 发布。

## 已核实
源码 D:\网站备份\寿县\14营收统计系统，GitHub yisqiu/RevenueStatisticsSystem（公开）。
114 SSH EasyTier 可用；外部8027、后端127.0.0.1:8127。
DBX已创建业务账号/Schema/六类表和14112条原型测试日报，另有3条验收日报。
统一认证独立应用 app-revenuestatistics；RevenueStatisticsSystem.Access / RevenueStatisticsSystem.SuperAdmin。
当前服务发布 /data/RevenueStatisticsSystem/releases/20261022-system-management-verified，Go PID2873590。新增/system三标签与本地多角色；权限为Revenue.Read、Revenue.Write、Integration.Manage，中央超级用户管理系统与基础设置。sx02绑定editor保留旧权限，admin/sx01为中央超级账号。完整证据见docs/system-management-20261022.md。
DBX已增量建立SYSTEM_USERS、SYSTEM_ROLES、SYSTEM_USER_ROLES、SYSTEM_LOGS，AUDIT_LOG扩展模块和详情；原14115条日报保留，临时验收数据已清理，审计保留。

## 约定
Go仅本机构建；SSH使用MCPSSH；DB操作使用DBX；境外依赖/GitHub走7897代理；凭据不进入Git。
本轮系统管理Chrome9223普通任务标签pageId=17，标记system-management=1；其他用户标签保留。
