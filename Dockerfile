# syntax=docker/dockerfile:1
# 一个镜像：server、tenant、devseed 三个命令 + 前端构建产物 + Golden 快照。
#
#   docker build --build-arg BUILD_SHA=$(git rev-parse HEAD) -t dw-pilot-v2 .
#   docker run --rm -e DATABASE_URL=... -p 8080:8080 dw-pilot-v2
#   docker run --rm -e DATABASE_URL=... -e TENANT_TEST_PASSWORD=... dw-pilot-v2 tenant reset -tenant dlpv1
#
# 三个命令都在 PATH 里，镜像只设 CMD 不设 ENTRYPOINT，所以 docker run 后面的第一个词就是命令名。
# 基础镜像按 tag 写，平台接入时按 digest 固定。

FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY daily/ daily/
COPY api/ api/
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags='-s -w' -o /out/ ./api/cmd/server ./api/cmd/tenant ./api/cmd/devseed

# distroless static：没有 shell，自带 CA 证书和 tzdata（二进制里也内嵌了 time/tzdata），nonroot 用户 65532。
FROM gcr.io/distroless/static-debian12:nonroot
ARG BUILD_SHA=dev
ENV BUILD_SHA=${BUILD_SHA} \
    ADDR=:8080 \
    WEB_DIR=/app/web \
    TENANT_SNAPSHOT=/app/golden
WORKDIR /app
COPY --from=go /out/server /out/tenant /out/devseed /usr/local/bin/
COPY --from=web /src/web/dist /app/web
COPY api/testdata/golden /app/golden
USER nonroot:nonroot
EXPOSE 8080
CMD ["server"]
