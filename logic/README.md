


1. 用户会话信息（Hash）
存储用户 1001 的连接信息，包括 service_id、status 和 connected_at

bash
深色版本
HSET user:session:1001 service_id "access-01" status "online" connected_at "1723456789"
✅ 说明：使用 HSET 设置一个 Hash，后续可通过 HGETALL user:session:1001 查看。

2. 在线用户集合（Set）
将用户 1001 添加到全局在线用户集合中

bash
深色版本
SADD online_users 1001
✅ 说明：online_users 是一个 Set，支持快速判断用户是否在线：SISMEMBER online_users 1001

3. 每个 access 服务上的用户（Set）
将用户 1001 添加到 access-01 服务的用户集合中

bash
深色版本
SADD service:users:access-01 1001
✅ 说明：可用于查询 access-01 当前有哪些用户连接，用 SMEMBERS service:users:access-01 查看。

🔁 可选：设置过期时间（TTL）
建议为用户会话设置自动过期（如 60 秒），避免服务宕机后状态残留：

bash
深色版本
EXPIRE user:session:1001 60
客户端定期发送心跳时，刷新这个 TTL。

这些命令组合起来，就能完整支持：

用户上线注册
查询用户是否在线
查询用户连接到了哪个 access 服务
支持自动下线（通过 TTL）
非常适合用于消息推送系统的用户状态管理。



127.0.0.1:6379> HSET chatify:user:session:1001 service_id "access-01" status "online" connected_at "1723456789"
(integer) 3
127.0.0.1:6379> HSET chatify:user:session:1002 service_id "access-01" status "online" connected_at "1723456789"
(integer) 3
127.0.0.1:6379> scan 0 match chatify* count 100
