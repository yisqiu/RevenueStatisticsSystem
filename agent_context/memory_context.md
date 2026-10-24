# 项目上下文

## 当前目标
原型复刻营收统计系统，Vben Admin 5.7/Vue3 + Go + 达梦 SZGCTRIP/REVENUESTATISTICSSYSTEM，统一认证，8027 发布。

## 已核实
源码 D:\网站备份\寿县\14营收统计系统，GitHub yisqiu/RevenueStatisticsSystem（公开）。
114 SSH EasyTier 可用；外部8027、后端127.0.0.1:8127。
DBX已创建业务账号/Schema/六类表，初始14112条原型测试日报及3条验收日报。2026-10-22按用户要求清除guide模块1512条原型测试数据；清理后全站12603条日报、guide为0。随后中台正式推送入库，目前全站12605条、guide为2条1960元/12人，其余8模块保留。清理备份在artifacts/guide-cleanup-20261022，记录见docs/guide-test-cleanup-20261022.md。
统一认证独立应用 app-revenuestatistics；RevenueStatisticsSystem.Access / RevenueStatisticsSystem.SuperAdmin。
当前静态版本 /data/RevenueStatisticsSystem/releases/20261024-reception-types，Go PID2873590保持，guide接收PID300963。接待类型修复见本文件末段。修复新增/编辑表单保存跳过校验、保存中输入仍可改及Enter无效；37项面板和6项键盘/窄屏检查通过。独立临时日报真实新增/修改及清理完成，业务数量与金额恢复，见docs/panels-audit-20261023.md。侧栏按钮及前版交互修复保持。
已有/system三标签与本地多角色；权限为Revenue.Read、Revenue.Write、Integration.Manage，中央超级用户管理系统与基础设置。sx02绑定editor保留旧权限，admin/sx01为中央超级账号。系统管理验收见docs/system-management-20261022.md。
DBX已增量建立SYSTEM_USERS、SYSTEM_ROLES、SYSTEM_USER_ROLES、SYSTEM_LOGS，AUDIT_LOG扩展模块和详情；管理验收临时数据已清理，审计保留。最新业务日报数量见上段。

## 约定

中台项目10015已发布推送任务2/配置3/Quartz62，每小时07/22/37/52分推送现行非测试订单；23:05首次入库、23:07自动重复跳过验证通过。接口dmp-guide-orders仅绑定guide，专用接收服务revenuestatistics-guide-ingest监听127.0.0.1:8128，回环凭据代理8129，原营收PID2873590及Quartz PID2604保持。新GUIDE_IMPORT_STATE表处理版本、幂等、取消/退款撤回。部署与回退见docs/guide-push-20261022.md；凭据只在服务器受限配置。

Go仅本机构建；SSH使用MCPSSH；DB操作使用DBX；境外依赖/GitHub走7897代理；凭据不进入Git。
当前清理Chrome9223普通任务标签pageId=11，标记cleanup=20261022-guide；其他标签保留。密码入口已实际验证为http://cd.libeasy.cn:8012/account/password.html，仅检查页面字段，未提交密码。

## 2026-10-24 接待类型同步修复
已发布 20261024-reception-types；接收器按 receptionType 写入分类，页面按真实分类显示图表。两笔历史日报通过原接口修正为团队1800元/10人、游客160元/2人，合计1960元/12人保持；中台14条重复推送成功，其他八业态汇总保持。主服务PID2873590、Nginx4328、Quartz2604保持，接收PID300963。详情 docs/guide-reception-type-20261024.md，独立修复分支 fix/guide-reception-type-20261024。
