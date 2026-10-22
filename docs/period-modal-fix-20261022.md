# 周期查询与日报弹窗修复

日期：2026-10-22；基线dedfc4d。

## 日报编辑层级

已复现：刷新文创页面，先打开新增日报再关闭，然后打开日报列表并点击编辑。两个Modal层级均为2000，列表后挂载，点击编辑输入框所在坐标时命中日报列表。

新增/编辑日报窗设置z-index=2001，列表保持2000，消息层2010继续在其上。按同一操作顺序复测，坐标命中编辑窗，输入框可获得焦点，取消后原备注保留。模拟保存接口返回成功时仅提交一次，编辑窗关闭、日报列表保留；本轮未向数据库写入日报。

本机前后截图：artifacts/modal-behind-before.png、artifacts/modal-front-after.png。

## 周期查询网络错误

当前真实浏览器日/年/月连续切换正常，汇总及设置请求均200，四图正常。服务健康正常，所查Nginx错误日志未发现对应错误；用户Edge同时间段访问日志也包含200请求。原始断连原因尚未复现。

对GET网络TypeError增加一次250ms后的自动重试，覆盖请求建立阶段及响应正文读取阶段；持续失败使用中文连接提示及原重新加载按钮。HTTP业务错误与写请求沿用原处理。

故障注入验证：首次请求失败、200响应后读取正文失败均请求两次后恢复；连续两次失败显示中文提示；连接恢复后重新加载按钮可恢复四图；HTTP500业务错误仅请求一次。正常日、年、月金额分别为0.0、6374.0、949.0万元。

明细见 [回归检查](period-modal-fix-checks.json)。网络故障与保存成功响应均为本任务标签内的模拟，网络恢复后的请求使用真实线上接口。

## 发布

前端typecheck及生产build通过。新版本/data/RevenueStatisticsSystem/releases/20261022-period-modal，备份指针位于backups/20261022-before-period-modal/previous-release.txt。
发布包SHA256：dcbbd3e65a79f9177dc4d6d90b31406cd2d6c7a53955cd26f7180abf4e068b02；450个文件manifest校验通过。
Go二进制与上一版一致（c9538c59944c4e6935f5ddd86ffeb3453ca3e332ebe9cedf1e328aceef9782f2），仅原子切换静态版本，Go PID2873590保持。
健康接口确认SZGCTRIP/REVENUESTATISTICSSYSTEM正常。浏览器刷新后加载新版本。
