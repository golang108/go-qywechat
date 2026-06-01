<!-- Created by Yanjunhui -->

# 企业微信 OpenFalcon 消息转发

将 OpenFalcon 告警消息转发到企业微信应用。

[English README](README.md)

## 功能

- 接收 OpenFalcon sender 格式：`tos` 和 `content`。
- 调用企业微信应用消息接口发送文本消息。
- 从 `config.conf` 或环境变量读取配置。
- 发送消息前必须通过请求 token 鉴权。
- HTTP 调用带超时，并显式处理上游错误。

## 企业微信准备

1. 申请企业微信号。
2. 在 **我的企业 -> 企业信息** 获取 `CorpID`。
3. 在 **应用管理 -> 添加应用** 创建应用。
4. 获取应用的 `AgentId` 和 `Secret`。
5. 接收人需要关注企业号，否则可能只能在企业微信 App 内收到消息。

## 配置

创建本地配置文件：

```sh
cp config.example.conf config.conf
```

编辑 `config.conf`：

```ini
[http]
address = 0.0.0.0
port = 4567

[server]
auth_token = replace-with-a-long-random-token

[weixin]
CorpID = replace-with-your-corpid
AgentId = 1000001
Secret = replace-with-your-secret
```

`config.conf` 已加入 `.gitignore`，不要提交真实密钥。

环境变量会覆盖配置文件：

| 变量 | 说明 |
| --- | --- |
| `CHAT_CONFIG` | 配置文件路径，默认 `config.conf`。 |
| `CHAT_HTTP_ADDRESS` | HTTP 监听地址。 |
| `CHAT_HTTP_PORT` | HTTP 监听端口。 |
| `CHAT_AUTH_TOKEN` | `/send` 请求 token。 |
| `WEIXIN_CORP_ID` | 企业微信 CorpID。 |
| `WEIXIN_AGENT_ID` | 企业微信应用 AgentId。 |
| `WEIXIN_SECRET` | 企业微信应用 Secret。 |

## 构建与运行

```sh
go build -o main .
./control.sh start
./control.sh status
./control.sh stop
```

前台调试：

```sh
go run .
```

## 发送接口

接口：

```text
GET /send?token=<auth_token>&tos=<wecom_user>&content=<message>
POST /send
```

参数：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `token` | 是 | 请求 token，也可以通过 `X-Chat-Token` 或 `Authorization: Bearer <token>` 传入。 |
| `tos` | 是 | 企业微信用户 ID。 |
| `content` | 是 | 文本消息内容。 |

示例：

```sh
curl 'http://127.0.0.1:4567/send?token=your-token&tos=zhangsan&content=test'
```

也支持 JSON `POST`：

```sh
curl -X POST 'http://127.0.0.1:4567/send?token=your-token' \
  -H 'Content-Type: application/json' \
  -d '{"tos":"zhangsan","content":"test"}'
```

## OpenFalcon+ 配置

在 OpenFalcon+ 的 IM 地址里配置本服务，token 可以直接放到 URL：

```json
{
  "api": {
    "im": "http://127.0.0.1:4567/send?token=your-token",
    "sms": "http://127.0.0.1:10086/sms",
    "mail": "http://127.0.0.1:10086/mail",
    "dashboard": "http://127.0.0.1:8081",
    "plus_api": "http://127.0.0.1:8080",
    "plus_api_token": "used-by-alarm-in-server-side-and-disabled-by-set-to-blank"
  }
}
```

OpenFalcon 发送格式：

```text
tos     企业微信用户 ID
content 告警内容
```

## 验证

```sh
go test ./...
go build ./...
```
