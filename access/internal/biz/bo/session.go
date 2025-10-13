package bo

const (
	SessionKeyPrefix = "chatify:session:"
)

type Session struct {
	Uid            string `json:"uid"`
	Username       string `json:"username"`
	ConnectionTime int64  `json:"connection_time"`
	ConnectionId   string `json:"connection_id"`
}
