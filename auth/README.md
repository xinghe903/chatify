# Auth Service

Auth Service 是一个基于 Kratos 框架实现的认证微服务，提供用户注册、登录、登出、账户注销和令牌刷新等核心认证功能。

## 目录结构

Auth 服务遵循标准的 Kratos 项目结构，采用分层架构设计：

```
├── api/                # API 定义目录
│   └── auth/v1/        # Protobuf 协议定义
├── cmd/                # 命令行入口
│   └── auth/           # 服务主入口
├── configs/            # 配置文件目录
├── internal/           # 内部实现目录
│   ├── biz/            # 业务逻辑层
│   │   └── bo/         # 业务对象
│   ├── conf/           # 配置定义
│   ├── data/           # 数据访问层
│   ├── server/         # 服务器实现
│   └── service/        # 服务实现层
├── third_party/        # 第三方依赖
├── go.mod              # Go 模块定义
├── Makefile            # 构建脚本
└── README.md           # 项目文档
```

## 主要功能

Auth 服务实现了以下核心功能：

1. **用户注册（Register）**：创建新用户账户
2. **用户登录（Login）**：支持通过用户名、邮箱或手机号登录
3. **用户登出（Logout）**：退出当前会话，使令牌失效
4. **用户注销（RevokeUser）**：永久删除用户账户
5. **刷新令牌（RefreshToken）**：使用 refresh token 续签 access token

## 环境要求

- Go 1.23.0 或更高版本
- Redis（用于用户数据存储和令牌管理）

## 快速开始

### 1. 安装依赖

```bash
cd auth
make deps
```

### 2. 构建服务

```bash
make build
```

### 3. 运行服务

```bash
make run
```

## 配置说明

服务配置文件位于 `configs/config.yaml`，主要配置项包括：

- **Server**：HTTP 和 gRPC 服务器配置
- **Data**：数据库和 Redis 配置
- **Auth**：认证相关配置（JWT密钥、令牌过期时间等）

示例配置：

```yaml
server:
  http:
    network: tcp
    addr: 0.0.0.0:8080
    timeout: 1s
  grpc:
    network: tcp
    addr: 0.0.0.0:9090
    timeout: 1s

data:
  database:
    driver: mysql
    source: root:password@tcp(localhost:3306)/auth?parseTime=true
  redis:
    network: tcp
    addr: localhost:6379
    read_timeout: 1s
    write_timeout: 1s

auth:
  jwt_secret: your-secret-key-here
  access_token_ttl: 3600        # 1小时
  refresh_token_ttl: 604800     # 7天
```

## API 文档

Auth 服务基于 Protobuf 定义了以下 API 接口：

### 1. Register

注册新用户。

**请求参数**：
- `username`：用户名（必填）
- `email`：邮箱（必填）
- `password`：密码（必填）
- `phone`：手机号（可选）

**返回结果**：
- `user_id`：注册成功的用户ID

### 2. Login

用户登录。

**请求参数**：
- `username`/`email`/`phone`：登录标识符（三选一）
- `password`：密码

**返回结果**：
- `user_id`：用户ID
- `access_token`：访问令牌
- `refresh_token`：刷新令牌
- `access_expires_in`：访问令牌过期时间（秒）
- `refresh_expires_in`：刷新令牌过期时间（秒）

### 3. Logout

用户登出。

**请求参数**：
- `user_id`：用户ID
- `refresh_token`：刷新令牌

### 4. RevokeUser

注销用户账户。

**请求参数**：
- `user_id`：用户ID
- `password`：密码（用于验证）
- `token`：当前有效的令牌（用于身份确认）

### 5. RefreshToken

刷新访问令牌。

**请求参数**：
- `refresh_token`：刷新令牌

**返回结果**：
- `access_token`：新的访问令牌
- `access_expires_in`：新的访问令牌过期时间（秒）

## 开发指南

### 生成 Wire 代码

依赖注入代码使用 Wire 工具生成：

```bash
make gen-wire
```

### 生成 Protobuf 代码

```bash
make gen-proto
```

### 格式化代码

```bash
make fmt
```

### 运行测试

```bash
make test
```

## 构建与部署

### Docker 构建

Auth 服务包含 Dockerfile 支持容器化部署：

```bash
docker build -t auth-service .
docker run -p 9090:9090 -p 8080:8080 auth-service
```

### Docker Compose 部署

项目根目录提供了 docker-compose.yaml 文件，支持与其他服务一起部署：

```bash
docker-compose up -d
```

## 注意事项

1. 生产环境中，请确保使用强密钥和适当的令牌过期时间
2. 密码存储应使用加密算法（当前实现为简化版本）
3. 服务依赖 Redis 进行数据存储，请确保 Redis 服务可用