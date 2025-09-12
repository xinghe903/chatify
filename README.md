# chatify
这是一个消息通知系统

## 含义
“Chatify” 是由 “Chat”（聊天）和后缀 “-ify”（意为“使…化”或“实现…”）组合而成的合成词。整体含义是“让沟通变得简单、即时”，既体现了系统核心功能——单聊（Chat），也隐含了消息通知（Notify）的即时性。“-ify” 后缀常用于科技产品命名（如 Spotify、Amplify），赋予名称现代感和技术感，简洁易记，适合一个轻量、专注的聊天系统。


消息推送系统
API Gateway 对外统一入口、认证、限流、路由转发   Nginx, JWT  无状态水平扩展、负载均衡、熔断降级
Access Service (接入层)    维护海量用户长连接、协议解析、上下行消息传递  Golang (利用其高并发特性), WebSocket, TCP, 自定义二进制协议 水平扩展、智能心跳、连接打散[bucket]
Session Service (会话服务)  管理用户状态、路由信息（用户与Access Service的映射）   Redis (Cluster模式)   中央存储/分布式缓存、快速读写、集群化
Logic Service (逻辑层) 消息路由、推送逻辑（单推、群推、广播）、调用业务处理  Golang, gRPC, Kafka 无状态设计、异步处理、消息队列削峰
Push Service (推送服务) 接收Logic指令，向特定Access Service发送消息 Golang, gRPC    无状态设计、连接池、高效RPC
Offline Message Service (离线消息)  存储用户离线时的消息  MongoDB/Redis   定义消息过期时间、支持多种存储引擎
Job Service (任务服务)  处理延迟/定时推送、批量推送等异步任务 Golang, Kafka   依赖消息队列、弹性扩缩容
Monitor & Admin Service (监控管理)  收集指标、日志、性能监控、灰度发布   Prometheus, Grafana, ELK



## 开始

启动参数
> docker-compose up -d


