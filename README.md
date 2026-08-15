# task005-notify

轻量通知服务，使用进程内存保存数据，不依赖数据库、第三方包和外部服务。

## 本地运行

```bash
go run . server --addr :8080
go run . --smoke-test
```

主要接口：`POST /api/notifications`、`GET /api/notifications`、`GET /api/notifications/{id}`、`POST /api/notifications/{id}/send`、`POST /api/notifications/{id}/read`、`DELETE /api/notifications/{id}`、`GET /healthz`。

## Docker

镜像使用国内 DaoCloud Go 1.26.3 Bookworm builder 和 Alpine 3.20 runtime；支持 `linux/amd64` 与 `linux/arm64`。
