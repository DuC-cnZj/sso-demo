FROM node:lts-alpine as build-stage
WORKDIR /app
COPY web ./
RUN yarn config set registry http://registry.npm.taobao.org/ \
    && yarn install
COPY web .
RUN yarn run build:prod

FROM golang:1.14-alpine AS builder

WORKDIR /app
COPY . .
COPY --from=build-stage /app/dist /app/resources/views/admin/

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
  apk add --no-cache ca-certificates tzdata

RUN go env -w GOPROXY=https://goproxy.io && \
    go env -w GO111MODULE=on && \
    go mod tidy -v

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /bin/app cmd/serve.go

#FROM alpine
FROM scratch

COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /bin/app /app/app
COPY --from=builder /app/resources /app/resources/

ENTRYPOINT ["/app/app", "-root", "/app/"]