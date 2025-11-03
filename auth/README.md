## 一、概述

Auth Service 是 Chatify 项目中的认证服务，负责处理用户身份验证、会话管理和授权等核心功能。该服务基于 Golang 实现，采用 Kratos 微服务框架，提供安全可靠的身份认证能力，作为一个独立模块运行。

## 二、目录结构

```
auth/
├── api/             # API定义，包含protobuf文件
│   ├── auth/        # 认证相关接口
│   │   └── v1/      # API版本
├── cmd/             # 应用程序入口
│   └── auth/        # 认证服务入口
├── configs/         # 配置文件
├── internal/        # 内部实现
│   ├── biz/         # 业务逻辑层
│   ├── conf/        # 配置定义
│   └── data/        # 数据访问层
│       └── po/      # 持久化对象
├── go.mod           # Go模块定义
├── go.sum           # 依赖版本锁定
└── openapi.yaml     # OpenAPI文档
```

## 三、功能特性

### 3.1 用户管理
- **用户注册**：支持通过用户名、邮箱和可选手机号注册账户
- **用户登录**：支持通过用户名/邮箱/手机号登录
- **用户注销**：永久删除用户账户，作废所有关联令牌
- **用户登出**：退出当前会话，作废指定刷新令牌
- **用户状态管理**：支持用户账户的激活、锁定和注销状态

### 3.2 令牌管理
- **双Token机制**：实现Access Token和Refresh Token的分离管理
- **Token刷新**：支持使用Refresh Token续签Access Token，无需重新登录
- **Token验证**：提供令牌有效性校验服务
- **令牌作废**：支持单点登出和批量令牌作废

### 3.3 安全特性
- **密码加密**：使用bcrypt算法对密码进行加密加盐存储
- **输入验证**：对用户名、邮箱、手机号等进行格式和规则校验
- **重复账户检测**：防止用户名、邮箱、手机号重复注册
- **用户状态验证**：登录时验证用户账户状态

## 四、技术栈

### 4.1 核心技术
- **编程语言**：Golang 1.24.0
- **框架**：Kratos v2.9.1
- **API协议**：gRPC
- **HTTP接口**：通过gRPC-Gateway自动生成RESTful API

### 4.2 数据存储
- **关系型数据库**：MySQL (使用GORM v1.25.10作为ORM框架)
- **缓存**：Redis v9.15.0 (用于缓存用户信息和令牌)

### 4.3 认证与安全
- **密码加密**：bcrypt算法
- **ID生成**：Sonyflake (雪花算法变种)
- **双Token机制**：Access Token (10分钟)和Refresh Token (7天)
- **不带用户信息的Token**：Token不包含敏感用户信息，通过服务端验证

## 五、API接口

### 5.1 认证服务 (AuthService)

#### 5.1.1 注册用户
```
rpc Register(RegisterRequest) returns (RegisterResponse)
```
- **HTTP路径**：POST `/chatify/auth/v1/register`
- **请求参数**：用户名、邮箱、密码、手机号(可选)
- **返回结果**：用户ID

#### 5.1.2 用户登录
```
rpc Login(LoginRequest) returns (LoginResponse)
```
- **HTTP路径**：POST `/chatify/auth/v1/login`
- **请求参数**：用户名/邮箱/手机号、密码
- **返回结果**：用户ID、Access Token、Refresh Token、过期时间

#### 5.1.3 注销用户
```
rpc RevokeUser(RevokeUserRequest) returns (RevokeUserResponse)
```
- **HTTP路径**：POST `/chatify/auth/v1/revokeUser`
- **请求参数**：用户ID、密码(安全验证)、当前Token
- **功能**：永久删除用户账户，作废所有关联Token

#### 5.1.4 用户登出
```
rpc Logout(LogoutRequest) returns (LogoutResponse)
```
- **HTTP路径**：POST `/chatify/auth/v1/logout`
- **请求参数**：用户ID、Refresh Token
- **功能**：退出当前会话，作废指定Refresh Token

#### 5.1.5 刷新Token
```
rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse)
```
- **HTTP路径**：POST `/chatify/auth/v1/refreshToken`
- **请求参数**：Refresh Token
- **返回结果**：新的Access Token、可选新的Refresh Token、过期时间

#### 5.1.6 验证Token
```
rpc VerifyToken(VerifyTokenRequest) returns (VerifyTokenResponse)
```
- **HTTP路径**：GET `/chatify/auth/v1/verifyToken`
- **请求参数**：Access Token
- **返回结果**：用户名、用户ID、过期时间戳

## 六、调用流程与服务形式

### 6.1 整体架构
- **统一入口**：所有外部客户端请求统一通过nginx网关访问系统
- **服务隔离**：认证服务作为独立模块运行，不涉及下游调用
- **代理转发**：nginx负责请求的路由和代理转发

### 6.2 认证流程
- **认证接口访问**：外部客户端请求认证接口（如登录、注册）时，nginx直接代理到认证服务
- **业务接口访问**：外部客户端请求其他业务接口时，nginx首先调用认证服务验证用户身份，获取用户信息，然后再将请求代理到相应的业务服务
- **令牌管理**：所有令牌的生成、验证和刷新请求均通过nginx网关转发至认证服务处理

## 七、配置说明

主要配置项位于 `configs/config.yaml` 文件中：

```yaml
server:
  http:            # HTTP服务配置
    network: tcp
    addr: 0.0.0.0:8080
    timeout: 1s
  grpc:            # gRPC服务配置
    network: tcp
    addr: 0.0.0.0:9090
    timeout: 1s

data:
  database:        # 数据库配置
    driver: mysql
    source: myuser:mypassword@tcp(localhost:13306)/chatify?parseTime=true
  redis:           # Redis配置
    network: tcp
    addr: localhost:16379
    read_timeout: 1s
    write_timeout: 1s

auth:
  access_token_ttl: 600               # Access Token有效期(秒)，默认10分钟
  refresh_token_ttl: 604800           # Refresh Token有效期(秒)，默认7天
```

## 八、系统架构

### 8.1 分层设计
- **API层**：定义接口契约，处理请求路由和参数解析
- **业务层(biz)**：实现核心业务逻辑，包含认证规则和流程
- **数据层(data)**：负责数据存取，包括MySQL和Redis操作
- **依赖注入**：使用Google Wire进行依赖注入

### 8.2 数据模型
用户表结构(chatify_auth_user)：
- ID: 用户唯一标识(使用Sonyflake生成)
- Username: 用户名(唯一)
- Email: 邮箱(唯一)
- Phone: 手机号(可选，唯一)
- Password: 加密后的密码
- Status: 用户状态(active, locked, revoked)
- RevokedAt: 注销时间
- CreatedAt: 创建时间
- UpdatedAt: 更新时间

### 8.3 缓存策略
- **用户信息缓存**：缓存用户基本信息，减少数据库查询
- **令牌映射缓存**：维护用户名/邮箱/手机号到用户ID的映射
- **令牌存储**：存储和管理AccessToken和RefreshToken

## 九、安全设计

### 9.1 密码安全
- 密码使用bcrypt算法加盐加密存储
- 禁止明文密码传输和存储
- 密码强度检查(前端实现)

### 9.2 令牌安全
- 采用双Token机制，Access Token有效期短，Refresh Token有效期长
- Token存储在Redis中，支持快速作废
- 刷新Token时可选轮换新的Refresh Token，提高安全性

### 9.3 输入验证
- 用户名规则：长度限制(最大20字符)，不允许为手机号，不允许包含@和空格
- 邮箱规则：符合标准邮箱格式，长度限制(最大50字符)
- 手机号规则：符合中国手机号格式

## 十、部署说明

### 10.1 环境要求
- Go 1.24.0或更高版本
- MySQL 5.7或更高版本
- Redis 6.0或更高版本

### 10.2 构建与运行

1. **安装依赖**
```bash
cd auth
go mod tidy
```

2. **编译服务**
```bash
make build
```

3. **运行服务**
```bash
./bin/auth -conf configs/config.yaml
```

### 10.3 Docker部署
项目包含Dockerfile，支持容器化部署：
```bash
docker build -t chatify-auth .
docker run -p 8080:8080 -p 9090:9090 --env CONFIG_PATH=/app/configs/config.yaml chatify-auth
```

## 十一、开发指南

### 11.1 项目初始化
1. 克隆仓库并进入auth目录
2. 安装Go依赖：`go mod tidy`
3. 初始化数据库架构
4. 配置本地开发环境

### 11.2 代码生成
使用Kratos工具生成代码：
```bash
kratos proto client api/auth/v1/auth.proto
```

### 11.3 测试
运行单元测试：
```bash
make test
```

## 十二、常见问题

### 12.1 用户注册失败
- 检查用户名、邮箱或手机号是否已被注册
- 确认输入信息符合格式要求
- 查看服务日志获取具体错误信息

### 12.2 登录认证失败
- 确认账号密码正确
- 检查用户账户状态是否正常(未被锁定或注销)
- 尝试重置密码

### 12.3 Token刷新失败
- 确认Refresh Token是否有效且未过期
- 检查用户账户状态
- 如多次刷新失败，可能需要重新登录

## 十三、维护与更新

### 13.1 版本控制
- 遵循语义化版本规范
- 详细记录每次版本变更

### 13.2 更新建议
- 定期更新依赖库版本
- 根据实际需求调整Token有效期
- 监控系统性能，优化数据库查询和缓存策略