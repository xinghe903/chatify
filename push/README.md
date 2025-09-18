


# 用户会话信息（主数据）
user:session:1001 -> Hash: {service_id: "access-01", status: "online", connected_at: "1723456789"}

# 在线用户集合（用于快速查询）
online_users -> Set: {1001, 1002, 1003}

# 每个 access 服务上的用户（可选）
service:users:access-01 -> Set: {1001, 1002}


