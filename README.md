# mail-code-get

一个面向 Hotmail/Outlook.com 的 Go CLI，使用微软官方推荐的 `OAuth2 + Microsoft Graph` 读取最近几条邮件，默认覆盖：

- `Inbox`
- `Junk Email`
- `Deleted Items`

工具会把这些系统文件夹里的最近邮件合并后按时间倒序输出。

## 为什么不用邮箱密码

Outlook.com 个人邮箱现在要求新式身份验证。Basic Auth/普通密码直连 IMAP 已经被逐步拦截，因此这个项目改为：

- 用户首次运行时通过设备码登录
- 本地缓存 OAuth token
- 后续通过 Microsoft Graph 读取邮件

## 使用前准备

你需要先在 Microsoft Entra 注册一个应用，并确保：

1. `Supported account types` 包含个人 Microsoft 账号
2. `Allow public client flows` 已开启
3. 已添加委托权限 `Mail.Read`

拿到应用的 `Application (client) ID` 后即可运行。

## 用法

推荐先设置环境变量：

```bash
export MAIL_CODE_GET_CLIENT_ID="your-app-client-id"
export MAIL_CODE_GET_TENANT_ID="consumers"
```

然后运行：

```bash
go run . fetch --limit 5
```

也可以直接传参数：

```bash
go run . fetch --client-id "your-app-client-id" --tenant-id "consumers" --limit 5
```

第一次成功运行后，CLI 也会把这两个值写入当前项目的 `data/config.json`，后续可直接运行：

```bash
go run . fetch --limit 5
```

首次授权成功后，`data/config.json` 里还会补充当前登录邮箱和账号标识，方便你确认现在绑的是哪个 Hotmail/Outlook 账号。

## 多账号

当前项目支持缓存多个 Hotmail/Outlook 账号。

- 查看当前项目已缓存账号：

```bash
go run . accounts list
```

- 指定某个邮箱读取邮件：

```bash
go run . fetch --email someone@hotmail.com --limit 5
```

- 如果指定的邮箱本地还没有缓存，CLI 会引导你做一次设备码登录，并把该账号加入当前项目的账号列表。

工具会把最近一次成功使用的邮箱记为默认账号；后续不传 `--email` 时优先使用默认账号。

输出 JSON：

```bash
go run . fetch --client-id "your-app-client-id" --json --limit 5
```

如果需要强制重新登录：

```bash
go run . auth reset
go run . fetch --client-id "your-app-client-id"
```

如果你希望在本次抓取前顺手强制重新登录，也可以直接：

```bash
go run . fetch --client-id "your-app-client-id" --reset-auth
```

## 默认配置

- 默认 tenant：`consumers`
- 默认返回条数：`10`
- 登录方式：`Device Code Flow`
- 邮件接口：`Microsoft Graph v1.0`

## 输出字段

JSON 模式下会返回：

- `folder`
- `id`
- `subject`
- `from`
- `to`
- `date`
- `message_id`
- `unread`

## 本地认证数据

工具会在当前项目的 `data/` 目录下保存项目级配置和认证状态：

- `data/config.json`
- `data/token-cache.bin`

其中：

- `config.json` 保存 `client_id`、`tenant_id`、默认邮箱，以及多个账号的邮箱标识、账号 ID
- `token-cache.bin` 保存本项目专用的本地 OAuth token cache

当前版本不再使用 macOS Keychain / `azidentity/cache`。

## 参考文档

- [Build Go apps with Microsoft Graph](https://learn.microsoft.com/en-us/graph/tutorials/go-authentication)
- [Use the Microsoft Graph API to get Outlook mail](https://learn.microsoft.com/en-us/graph/tutorials/go-email)
- [Outlook.com POP, IMAP, and SMTP settings](https://support.microsoft.com/en-gb/office/pop-imap-and-smtp-settings-for-outlook-com-d088b986-291d-42b8-9564-9c414e2aa040)
