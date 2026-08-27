# BENZHI_README

基于 Go 实现的stage-rigging-release Web 项目，一款后端服务，面向剧场技术团队的舞台高空吊挂安全工作台，覆盖建档、器材核验、载荷评估、彩排整改、独立复核、证据冻结、放行凭据签发与验真。

## 项目说明
- 项目：benzhi-project-14fadb8f-d2f7-4352-808d-7689cf681e7a
- 项目用途：面向剧场技术团队的舞台高空吊挂安全工作台，覆盖建档、器材核验、载荷评估、彩排整改、独立复核、证据冻结、放行凭据签发与验真。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19091
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-14fadb8f-d2f7-4352-808d-7689cf681e7a-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-14fadb8f-d2f7-4352-808d-7689cf681e7a-arm64 linux/arm64
docker run -it benzhi-project-14fadb8f-d2f7-4352-808d-7689cf681e7a-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19091`
