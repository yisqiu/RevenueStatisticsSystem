# 项目上下文

## 当前目标
原型复刻营收统计系统，Vben Admin 5.7/Vue3 + Go + 达梦 SZGCTRIP/REVENUESTATISTICSSYSTEM，统一认证，8027 发布。

## 已核实
源码 D:\网站备份\寿县\14营收统计系统，GitHub yisqiu/RevenueStatisticsSystem（公开）。
114 SSH EasyTier 可用；外部8027、后端127.0.0.1:8127。
DBX已创建业务账号/Schema/六类表和14112条原型测试日报，另有3条验收日报。
统一认证独立应用 app-revenuestatistics；RevenueStatisticsSystem.Access / RevenueStatisticsSystem.SuperAdmin。
当前服务发布 /data/RevenueStatisticsSystem/releases/20261022-accepted；2026-10-22复核修复后端与界面，最终前端更新保留Go PID2706491。具体证据见docs/review-20261022.md。

## 约定
Go仅本机构建；SSH使用MCPSSH；DB操作使用DBX；境外依赖/GitHub走7897代理；凭据不进入Git。
本轮复核Chrome9223普通标签pageId=14；原任务pageId=9保留。
