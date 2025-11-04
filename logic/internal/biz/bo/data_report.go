package bo

type DataReport struct {
	UserId    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
	Data      string `json:"data"`
}
