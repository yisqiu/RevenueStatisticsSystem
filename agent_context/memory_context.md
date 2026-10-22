# 项目上下文

## 当前目标
原型复刻营收统计系统，Vben Admin 5.7/Vue3 + Go + 达梦 SZGCTRIP/REVENUESTATISTICSSYSTEM，统一认证，8027 发布。

## 已核实
源码 D:\网站备份\寿县\14营收统计系统，GitHub yisqiu/RevenueStatisticsSystem（公开）。
114 SSH EasyTier 可用；外部8027、后端127.0.0.1:8127。
DBX已创建业务账号/Schema/六类表，初始14112条原型测试日报及3条验收日报。2026-10-22按用户要求清除guide模块1512条原型测试数据；当前全站12603条日报，guide为0，其余8模块保留。完整备份及恢复SQL在artifacts/guide-cleanup-20261022，记录见docs/guide-test-cleanup-20261022.md。
统一认证独立应用 app-revenuestatistics；RevenueStatisticsSystem.Access / RevenueStatisticsSystem.SuperAdmin。
当前服务发布 /data/RevenueStatisticsSystem/releases/20261022-site-audit，Go PID2873590保持。全站检查再修复5项：系统管理/导出网络错误、损坏JSON内容消失、搜索建议键盘焦点、密码链接。统一GET读取重试一次，写请求单次；见docs/site-interaction-audit-20261022.md。上轮日报编辑层2001继续验证通过。
已有/system三标签与本地多角色；权限为Revenue.Read、Revenue.Write、Integration.Manage，中央超级用户管理系统与基础设置。sx02绑定editor保留旧权限，admin/sx01为中央超级账号。系统管理验收见docs/system-management-20261022.md。
DBX已增量建立SYSTEM_USERS、SYSTEM_ROLES、SYSTEM_USER_ROLES、SYSTEM_LOGS，AUDIT_LOG扩展模块和详情；管理验收临时数据已清理，审计保留。最新业务日报数量见上段。

## 约定
Go仅本机构建；SSH使用MCPSSH；DB操作使用DBX；境外依赖/GitHub走7897代理；凭据不进入Git。
当前清理Chrome9223普通任务标签pageId=11，标记cleanup=20261022-guide；其他标签保留。密码入口已实际验证为http://cd.libeasy.cn:8012/account/password.html，仅检查页面字段，未提交密码。
