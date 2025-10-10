package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"

	"github.com/sony/sonyflake"
)

// generateRandomString 生成随机字符串
// length: 返回字符串长度
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// 使用URL安全的base64编码，不添加填充字符
	token := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes)
	return token, nil
}

// Sonyflake 是基于Sonyflake算法的分布式ID生成器
// 特点：
//   - 分布式：支持在多个节点上生成唯一ID
//   - 趋势递增：生成的ID总体趋势是递增的
//   - 高性能：无锁设计，生成速度快
//   - 时间有序：ID中包含时间戳信息，可按时间排序
//
// ID结构：1位保留 + 39位时间戳 + 8位机器ID + 16位序列号
// 适用于分布式系统、数据库主键等场景
type Sonyflake struct {
	sf *sonyflake.Sonyflake
}

// NewSonyflake 创建并返回一个新的Sonyflake实例
// 使用默认配置初始化Sonyflake生成器
// 默认配置使用内网IP地址作为机器ID的生成依据
// 返回：初始化好的Sonyflake实例指针
func NewSonyflake() *Sonyflake {
	var st sonyflake.Settings
	return &Sonyflake{sf: sonyflake.NewSonyflake(st)}
}

// GenerateID 生成一个唯一的分布式ID
// 如果ID生成过程中发生错误，会触发panic
// 返回：64位的唯一ID
func (s *Sonyflake) GenerateID() (uint64, error) {
	return s.sf.NextID()
}

// 不保证递增
func (s *Sonyflake) GenerateIDString() (string, error) {
	id, err := s.GenerateID()
	if err != nil {
		return "", err
	}
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, id)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// 使用保持顺序的字符集
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// 生成的string递增
func (s *Sonyflake) GenerateBase62() (string, error) {
	id, err := s.GenerateID()
	if err != nil {
		return "", err
	}

	var result []byte
	for id > 0 {
		result = append([]byte{base62Chars[id%62]}, result...)
		id /= 62
	}
	return string(result), nil
}
