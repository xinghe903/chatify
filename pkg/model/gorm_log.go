package model

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm/logger"
)

// KratosGormLogger 是一个适配器，将 GORM 日志桥接到 Kratos 日志
type KratosGormLogger struct {
	log *log.Helper
	logger.Config
}

func NewKratosGormLogger(l *log.Helper) *KratosGormLogger {
	return &KratosGormLogger{
		log: l,
		Config: logger.Config{
			SlowThreshold: 200 * time.Millisecond, // 慢查询阈值
			LogLevel:      logger.Info,            // 日志级别
			Colorful:      true,
		},
	}
}

// LogMode 实现 logger.Interface
func (l *KratosGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 实现
func (l *KratosGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		l.log.WithContext(ctx).Infof(msg, data...)
	}
}

// Warn 实现
func (l *KratosGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		l.log.WithContext(ctx).Warnf(msg, data...)
	}
}

// Error 实现
func (l *KratosGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		l.log.WithContext(ctx).Errorf(msg, data...)
	}
}

// Trace 实现 SQL 执行追踪
func (l *KratosGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	switch {
	case err != nil && l.LogLevel >= logger.Error:
		l.log.WithContext(ctx).Errorf("SQL Error: %v, Elapsed: %s, SQL: %s, Rows: %d", err, elapsed, sql, rows)
	case elapsed >= l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		l.log.WithContext(ctx).Warnf("SLOW SQL >= %v, Elapsed: %s, SQL: %s, Rows: %d", l.SlowThreshold, elapsed, sql, rows)
	case l.LogLevel >= logger.Info:
		l.log.WithContext(ctx).Infof("SQL: %s, Elapsed: %s, Rows: %d", sql, elapsed, rows)
	}
}
