package data

import (
	"access/internal/biz"
	"access/internal/biz/bo"
	"context"
	"encoding/json"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	SessionKeyPrefix = "chatify:session:"
)

var _ biz.SessionRepo = (*sessionRepo)(nil)

type sessionRepo struct {
	data *Data
	log  *log.Helper
}

func NewSessionRepo(data *Data, logger log.Logger) biz.SessionRepo {
	return &sessionRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (s *sessionRepo) SetSession(ctx context.Context, session *bo.Session) error {
	if session == nil {
		return nil
	}
	sessionJson, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.data.redisClient.Set(ctx, SessionKeyPrefix+session.Uid, sessionJson, 0).Err()
}

func (s *sessionRepo) GetSession(ctx context.Context, uid string) (*bo.Session, error) {
	sessionJson, err := s.data.redisClient.Get(ctx, SessionKeyPrefix+uid).Result()
	if err != nil {
		return nil, err
	}
	var session bo.Session
	err = json.Unmarshal([]byte(sessionJson), &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *sessionRepo) ClearSession(ctx context.Context, uid string) error {
	return s.data.redisClient.Del(ctx, SessionKeyPrefix+uid).Err()
}

func (s *sessionRepo) BatchClearSession(ctx context.Context, uids []string) error {
	keys := make([]string, len(uids))
	for i, uid := range uids {
		keys[i] = SessionKeyPrefix + uid
	}
	return s.data.redisClient.Del(ctx, keys...).Err()
}
