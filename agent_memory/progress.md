# 当前任务：清除讲解员接待测试数据

## 本轮已完成
用户授权清除8027讲解员接待模块测试数据。通过DBX现场核实REVENUESTATISTICSSYSTEM账号、SZGCTRIP实例，完整备份guide的1512条原型测试日报及恢复SQL后，以达梦事务块限定guide、seed来源/主键、版本1删除1512条并保留清理审计。
清理后guide记录0、全站12603条；其他8模块数量/金额/数量合计/接待量全部与清理前一致。
ChromeAiNewTabOnly=true，9223普通任务标签11，cleanup=20261022-guide；日/月/年营收及接待量0，趋势全0，列表空，2图正常，控制台无异常。
备份artifacts/guide-cleanup-20261022，报告docs/guide-test-cleanup-20261022.md及同名checks.json。代码和运行版本保持，无重启；最后同步GitHub文档并核对提交。

## 上轮全站交互检查（已完成）

## 本轮范围与成功标准
遍历8027的14个菜单入口、个人中心与旧入口跳转；检查周期、搜索、列表、编辑、关闭重开、设置、接口和系统管理弹窗；检查导出与密码修改链接。实际请求、控制台和界面共同验收。

## 已完成（2026-10-22）
修复5项：系统管理网络英文错误、损坏JSON导致内容消失、导出网络英文错误、搜索建议Tab移焦消失、个人中心密码链接错误。
查询和导出共用GET读取恢复，写请求单次提交并提示核对结果；搜索按click激活并保留内部焦点；密码入口改为/account/password.html。
14菜单、九业态周期及新增/列表/编辑弹窗、其他管理弹窗、搜索、旧入口、日志筛选分页、13项实际导出均通过；发布后62项检查全通过，记录174次实际API请求均正常、控制台无错误/警告。
密码入口实际点击到达正确账号和3个密码字段，未提交。未修改真实业务数据、角色状态及密码。原始偶发断连外部原因仍未复现。

## 发布与验证
前端typecheck/build、7组请求异常测试通过；已发布20261022-site-audit，489文件manifest及包SHA256核验通过。
Go二进制相同，原子切换前端，PID2873590保持；/api/health正常，临时HTTP传输服务已关闭。
备份指针backups/20261022-before-site-audit/previous-release.txt，前版20261022-period-modal。
报告docs/site-interaction-audit-20261022.md及site-interaction-audit-checks.json。Chrome9223任务标签4，audit=20261022-all；其余标签保留。
GitHub同步作为交付最后步骤，以HEAD及origin/main一致为准。
