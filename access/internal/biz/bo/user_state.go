package bo

// UserState 用户状态枚举
type UserState string

const (
	UserStateOnline  UserState = "online"  // 上线
	UserStateOffline UserState = "offline" // 下线
)

// UserStateMessage 用户状态消息结构体
type UserStateMessage struct {
	Id             string    `json:"id"`
	UserID         string    `json:"user_id"`         // 用户ID
	UserName       string    `json:"user_name"`       // 用户名称
	State          UserState `json:"state"`           // 状态：online/offline
	ConnectionTime int64     `json:"connection_time"` // 连接时间戳
	ConnectionId   string    `json:"connection_id"`   // 连接ID
}
