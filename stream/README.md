# Stream 服务

## 项目介绍

Stream 服务是 Chatify 项目中的一个核心组件，主要提供基于 gRPC 的流式通信功能，支持实时消息传输和双向数据交换。

## 目录结构

```
stream/
├── README.md              # 项目说明文档
├── client/                # 客户端代码
│   ├── api/               # 客户端 API 定义
│   ├── cmd/               # 客户端命令行工具
│   ├── go.mod             # Go 模块定义
│   └── go.sum             # 依赖版本锁定
└── service/               # 服务端代码
    ├── api/               # 服务端 API 定义
    └── cmd/               # 服务端命令行入口
        └── service/       # 服务实现
            ├── api/       # API 接口定义
            ├── internal/  # 内部实现
            │   ├── biz/   # 业务逻辑层
            │   ├── data/  # 数据访问层
            │   └── service/ # 服务实现层
            ├── go.mod     # Go 模块定义
            └── go.sum     # 依赖版本锁定
```

## 主要功能

1. **双向流式通信**：支持客户端和服务端之间的实时双向数据传输
2. **消息接收与处理**：接收客户端发送的消息并进行处理
3. **连接管理**：建立和维护 gRPC 连接

## 技术栈

- Go 语言
- gRPC
- Protocol Buffers

## 安装与运行

### 前提条件

- Go 1.16+ 环境
- 已安装 protoc 和相关插件

### 安装依赖

```bash
cd stream/service
go mod tidy
```

### 运行服务端

```bash
cd stream/service/cmd/service
go run main.go
```

## API 接口说明

Stream 服务提供以下主要接口：

### 1. Chat 接口

**功能**：双向流式通信接口，用于实时消息传输

**请求格式**：
```protobuf
message ChatReq {
  string name = 1;  // 消息名称/内容
}
```

**响应格式**：
```protobuf
message ChatRsp {
  string message = 1;  // 响应消息
}
```

**调用方式**：双向流式 RPC
```protobuf
service Service {
  rpc Chat (stream ChatReq) returns (stream ChatRsp);
}
```

## 客户端调用示例

### 触发方式示例

```bash
curl --location --request GET 'http://localhost:8010/helloworld/gene' \
--header 'Accept: */*' \
--header 'Host: localhost:8010' \
--header 'Connection: keep-alive'
```

### gRPC 客户端示例代码

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    v1 "service/api/service/v1"
)

func main() {
    // 连接服务器
    conn, err := grpc.Dial("localhost:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("did not connect: %v", err)
    }
    defer conn.Close()
    
    // 创建客户端
    c := v1.NewServiceClient(conn)
    
    // 创建上下文
    ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
    defer cancel()
    
    // 建立流式连接
    stream, err := c.Chat(ctx)
    if err != nil {
        log.Fatalf("could not chat: %v", err)
    }
    
    // 发送消息
    for i := 0; i < 5; i++ {
        err := stream.Send(&v1.ChatReq{Name: fmt.Sprintf("Message %d", i)})
        if err != nil {
            log.Fatalf("could not send: %v", err)
        }
        time.Sleep(time.Second)
    }
    
    // 关闭发送流并接收响应
    res, err := stream.CloseAndRecv()
    if err != nil {
        log.Fatalf("could not receive: %v", err)
    }
    
    fmt.Printf("Response: %s\n", res.Message)
}
```

## 注意事项

1. 确保服务端和客户端使用相同版本的 Protocol Buffers 定义
2. 服务默认监听 9000 端口，可通过配置文件修改
3. 流式连接会保持打开状态，直到客户端或服务端主动关闭
4. 如需高可用性部署，建议结合负载均衡和服务发现机制

## License

[MIT](https://opensource.org/licenses/MIT)

