# 配置引用草稿 v2 契约与回退

`schemaVersion: 2` 的 `references` 是业务引用的唯一持久化输入。`buffers` 仍保存独立 JSON、策略组非派生参数、DNS、路由等用户输入；订阅生成节点、关联入站对象、策略组生成成员和托管 sniff 规则只出现在服务端候选中。`editorState` 仅保存 `hidden`、图标等展示信息。`bindings` 仍保存原有规则集绑定。

```json
{
  "schemaVersion": 2,
  "references": {
    "version": 2,
    "outbounds": [{
      "id": "group-a", "tag": "选择", "type": "selector", "include": "HK", "exclude": "",
      "references": [{ "kind": "subscription", "subscriptionId": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "tag": "订阅名称" }]
    }],
    "inbounds": [{ "tag": "local-in", "resourceId": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "sourceTag": "源名称", "position": 0 }]
  }
}
```

`outbounds` 数组顺序决定策略组与引用顺序，`inbounds.position` 保留关联项与独立项的混排。保存 v2 时必须提交 `schemaVersion: 2` 和完整 `references`，并携带原有 `If-Match`。缺少版本的旧客户端向 v2 档案写入会收到 409 `schema_version_unsupported`；打开旧 v1 档案仍只在内存中适配，用户保存或复制时才迁移。源资源读取失败时不回退到旧展开副本。候选中的大整数通过 JSON 字符串回传，主动转独立采用同修订的签名候选或明确恢复迁移前备份。

首次将 v1 档案写成 v2 前，服务端把旧草稿保存到 `data/drafts/migration-backups/<id>-rev<revision>-v1.json`。不要在运行中的服务或正式 `Release/` 数据上试回退。回退演练已使用隔离目录验证：运行 `python3 scripts/restore-profile-v1.py <工作目录> <档案ID>` 后，旧草稿修订与索引匹配，已应用标记和预期运行状态被清除；脚本先归档迁移后的草稿、索引、核心状态和正式配置到同目录下的 `rollback-...` 文件夹。迁移后的用户编辑保存在归档中，不能靠切换旧二进制自动合并；回退后需核对归档、重新检测和应用，再启动核心。正式回退应同时使用与 v1 数据兼容的程序版本。

新格式的 `references` 包含来源 ID、过滤、别名、顺序和组引用；摘要继续使用类型前缀与源内容哈希。规则文件在检查后仍遵守现有锁边界，回退脚本不恢复运行中的核心进程。
