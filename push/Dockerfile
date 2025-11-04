FROM registry.cn-hangzhou.aliyuncs.com/xinghe903/golang:1.24.7 AS builder

WORKDIR /src

# 1. 先只复制 go.mod 和 go.sum，用于缓存依赖
COPY go.mod go.sum ./

# 2. 下载依赖（可选：验证依赖完整性）
RUN GOPROXY=https://goproxy.cn go mod download
# 或者更严谨一点：
# RUN GOPROXY=https://goproxy.cn go mod verify && go mod download

# 3. 再复制全部源码
COPY . .

# 4. 构建
RUN GOPROXY=https://goproxy.cn make build

FROM registry.cn-hangzhou.aliyuncs.com/xinghe903/debian:stable-slim

# 直接写入新的 sources.list（使用清华镜像，适配 bookworm）
RUN echo "deb https://mirrors.tuna.tsinghua.edu.cn/debian/ bookworm main" > /etc/apt/sources.list && \
    echo "deb https://mirrors.tuna.tsinghua.edu.cn/debian/ bookworm-updates main" >> /etc/apt/sources.list && \
    echo "deb https://mirrors.tuna.tsinghua.edu.cn/debian-security bookworm-security main" >> /etc/apt/sources.list

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./server", "-conf", "/data/conf"]
