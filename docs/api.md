# 外部系统接入

超级用户进入“接口对接”，新增系统名称并绑定经营业态。新建时显示一次专属密钥，保存到来源系统。服务端仅存密钥哈希。编辑配置可启用或停用接口。

`POST /api/ingest`，请求头 `Content-Type: application/json` 和 `Authorization: Bearer <接口密钥>`。

```json
{
  "sourceId": "ticket-order-20261021-001",
  "record": {
    "date": "2026-10-21",
    "category": "成人门票",
    "quantity": 3,
    "amount": "90.00",
    "visitors": 3,
    "note": "来源系统流水"
  }
}
```

amount 使用以元为单位的十进制字符串，最多两位小数，数据库按整数分保存。业态取自接口配置。相同接口与 sourceId 重复提交按幂等方式处理，来源编号应在对应系统中唯一。停用接口的密钥返回 401。

页面“测试接口展示数据”调用登录保护的 `GET /api/integration-test`，展示接口契约及测试请求示例。真实来源系统可使用上面的契约接入。当前已有验收用接口配置，完成去重测试后已停用。

业务查询与维护：`GET /api/summary?range=day|month|year&date=YYYY-MM-DD`、`GET/POST /api/records`、`GET/POST /api/settings`、`GET/POST /api/integrations`。日报编辑携带 id 和 version，版本冲突需刷新后重试。Excel 导入字段以“下载导入模板”为准，整批通过校验后保存。导出使用 `/api/export`，支持 xlsx、csv 和模板。
