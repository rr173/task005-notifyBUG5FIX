# syntax=docker/dockerfile:1
ARG GO_BASE=docker.m.daocloud.io/library/golang:1.26.3-bookworm
ARG ALPINE_BASE=docker.m.daocloud.io/library/alpine:3.20

FROM ${GO_BASE} AS builder
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/task005-notify .

FROM ${ALPINE_BASE}
WORKDIR /app
COPY --from=builder /out/task005-notify /app/task005-notify
EXPOSE 8080
ENTRYPOINT ["/app/task005-notify"]
CMD ["server", "--addr", ":8080"]
