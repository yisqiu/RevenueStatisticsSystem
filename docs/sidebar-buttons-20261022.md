# 恢复侧栏折叠与固定按钮（2026-10-22）

截图对应Vben原生侧栏底部折叠/展开、固定/取消固定开关。preferences.ts中fixedButton和collapsedButton在布局适配提交4458e82被设为false，线上缓存也记录了false。

## 修改

两项配置恢复true。现有偏好初始化明确以项目配置优先于缓存，浏览器刷新即可恢复，原有其他偏好保留。

原CSS使用first-of-type及nth-of-type定位logo、菜单和版本区；原生按钮以div插入后会错套高度。改为按logo结构、vben-scrollbar及相邻版本区定位。增加60px收起模式的32px标志、图标菜单与底部布局；展开底部预留84px，版本文字位于按钮上方。

## 验证

typecheck和生产build通过。Chrome9223普通任务标签17，标记sidebar-buttons=20261022，1440×1000视口实际鼠标点击验证：

| 状态 | 侧栏宽度 | 结果 |
| --- | --- | --- |
| 展开 | 238px | 两按钮24×24px，中心坐标命中按钮 |
| 点击收起 | 60px | 紧凑图标栏，展开按钮可见可点击 |
| 取消固定后移出 | 60px | 自动收起 |
| 移回悬停 | 238px | 自动展开，可点击固定按钮 |
| 固定后移出 | 238px | 保持展开 |
| 刷新 | 238px | 两按钮及固定状态保留 |

为稳定测量几何，检查过程在任务页面临时关闭过渡动画，结束后移除。正式样式保留框架原有动画。检查数据见sidebar-buttons-checks.json，本地截图artifacts/sidebar-buttons-expanded.png、sidebar-buttons-collapsed.png，控制台无错误/警告。

## 发布

仅打包web静态文件，不携带工作区其他接入功能的Go和Nginx变更。

- 当前版本：`/data/RevenueStatisticsSystem/releases/20261022-sidebar-buttons-v2`。
- 静态包SHA256：`78447647a21f98e7ed289d08e71ce2522201838e7bd5195962e32d68c8bcd9a7`，105个静态文件逐一核验。
- 原子切换前后营收服务PID2873590、讲解推送接收服务PID3181589保持，健康检查200。
- 初始前版20261022-site-audit，备份指针backups/20261022-before-sidebar-buttons/previous-release.txt；初次恢复后调整底部留白发布v2，前版20261022-sidebar-buttons亦保留。
- 回退：原子切换current至20261022-site-audit即可，业务服务保持运行。
