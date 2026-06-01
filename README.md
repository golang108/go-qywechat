<!-- Created by Yanjunhui -->

# WeCom OpenFalcon Chat Sender

Forward OpenFalcon alarm messages to a WeCom application.

[中文文档](README_zh.md)

## Features

- Accepts the OpenFalcon sender payload: `tos` and `content`.
- Sends WeCom text messages through the application message API.
- Loads credentials from `config.conf` or environment variables.
- Requires a request token before messages can be sent.
- Uses HTTP timeouts and explicit upstream error handling.

## WeCom Setup

1. Register a WeCom account.
2. Get the `CorpID` from **My Company -> Company Info**.
3. Create an application from **Apps -> Add App**.
4. Copy the application `AgentId` and `Secret`.
5. Make sure the receiver follows the WeCom account, otherwise messages may only be visible inside the WeCom app.

## Configuration

Create a local config file:

```sh
cp config.example.conf config.conf
```

Edit `config.conf`:

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

`config.conf` is ignored by Git and should not be committed.

Environment variables override the file values:

| Variable | Description |
| --- | --- |
| `CHAT_CONFIG` | Config file path. Defaults to `config.conf`. |
| `CHAT_HTTP_ADDRESS` | HTTP bind address. |
| `CHAT_HTTP_PORT` | HTTP bind port. |
| `CHAT_AUTH_TOKEN` | Request token for `/send`. |
| `WEIXIN_CORP_ID` | WeCom CorpID. |
| `WEIXIN_AGENT_ID` | WeCom application AgentId. |
| `WEIXIN_SECRET` | WeCom application Secret. |

## Build and Run

```sh
go build -o main .
./control.sh start
./control.sh status
./control.sh stop
```

For foreground testing:

```sh
go run .
```

## Send API

Endpoint:

```text
GET /send?token=<auth_token>&tos=<wecom_user>&content=<message>
POST /send
```

Parameters:

| Field | Required | Description |
| --- | --- | --- |
| `token` | Yes | Request token. It can also be sent as `X-Chat-Token` or `Authorization: Bearer <token>`. |
| `tos` | Yes | WeCom user ID. |
| `content` | Yes | Text message content. |

Example:

```sh
curl 'http://127.0.0.1:4567/send?token=your-token&tos=zhangsan&content=test'
```

JSON `POST` is also accepted:

```sh
curl -X POST 'http://127.0.0.1:4567/send?token=your-token' \
  -H 'Content-Type: application/json' \
  -d '{"tos":"zhangsan","content":"test"}'
```

## OpenFalcon+ Configuration

Set the IM endpoint to this service. The token can be embedded in the URL:

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

OpenFalcon should send:

```text
tos     WeCom user ID
content Alarm content
```

## Verification

```sh
go test ./...
go build ./...
```
