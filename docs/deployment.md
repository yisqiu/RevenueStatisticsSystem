# 部署与维护

## 当前部署

114 服务器，目录 `/data/RevenueStatisticsSystem`。网站 `http://cd.libeasy.cn:8027/`，Go 监听 `127.0.0.1:8127`，systemd 服务 `revenuestatistics.service`。

当前版本链接 `current` 指向 `releases/20261021-final`。上一版本 `releases/20261021-verified` 和初始版本 `releases/20261021-initial` 已保留，配置备份与版本指针位于 `backups/`。

最终发布包 SHA256：`b5560c07b1aeab720572899728fa905efd671a52bb069e2571aba629fe972aa0`。包内 `manifest.json` 保存每个文件的 SHA256。最终页面更新时验证了 Go 二进制与上版相同，原子切换 current，后端 PID 2673694 保持不变。

## 凭据与统一认证

`config/dm-dsn`、`config/oidc-client-id`、`config/oidc-client-secret` 权限 600，配置目录权限 700，属主 revenuestatistics。服务通过 *_FILE 读取，源码不保存真实值。达梦连接串遵循当前 DM 驱动格式，口令以驱动支持的原始形式配置。

Casdoor 应用 `app-revenuestatistics`，组织 sx，回调 `http://cd.libeasy.cn:8027/api/auth/callback`；应用权限为 RevenueStatisticsSystem.Access 和 RevenueStatisticsSystem.SuperAdmin。普通授权用户可查询和录入日报，超级用户维护设置及接口配置。所有业务请求均执行身份和应用授权校验。

## 发布

本机完成 Go 测试、Linux amd64 交叉编译、前端类型检查与构建后运行 `scripts/package-release.ps1`。经 MCPSSH 上传发布包到新的 releases 子目录，核对压缩包 SHA256 和 manifest 后发布。

首次安装或后端更新使用 `scripts/install-release.sh <新版本绝对路径>`，该脚本保存原配置和版本指针、配置独立服务、执行 nginx -t、重启本系统服务并 reload Nginx。单纯前端更新可在确认二进制一致后原子切换 current，延续现有会话。

后端重启后内存会话重新建立，用户经统一身份认证再次进入。服务器业务服务按系统独立维护。

## 回退

通过 MCPSSH 将 current 原子切回 `backups/previous-release-*.txt` 记录的已验证版本。若二进制有变化，重启 revenuestatistics.service；前端单独回退保持后端运行。若变更 Nginx，恢复对应备份，先 nginx -t，再 reload nginx。

本次最终页面版本可以回退至 `/data/RevenueStatisticsSystem/releases/20261021-verified`。该回退只涉及应用文件，保留数据库日报。数据库结构变更应在变更前通过 DBX 另行备份并制定配套回退。

验证：检查 `/api/health` 的数据库与 Schema、登录后的 `/api/summary`、真实页面图表和业务保存结果。
