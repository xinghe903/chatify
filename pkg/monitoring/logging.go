package monitoring

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string // 日志级别: debug, info, warn, error
	Format string // 日志格式: console, json
	Output string // 输出目标: stdout, stderr, 文件路径
}

// NewDefaultLoggingConfig 创建默认日志配置
func NewDefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}
}

// InitLogger 初始化zap日志器
func InitLogger(conf *LoggingConfig) log.Logger {
	// 设置日志级别
	level := zap.InfoLevel
	switch strings.ToLower(conf.Level) {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn", "warning":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	case "dpanic":
		level = zap.DPanicLevel
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	default:
		panic(fmt.Errorf("invalid log level: %s", conf.Level))
	}

	// 创建编码器
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	var encoder zapcore.Encoder
	if conf.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 创建输出
	var writer zapcore.WriteSyncer
	if conf.Output == "stdout" {
		writer = zapcore.AddSync(os.Stdout)
	} else if conf.Output == "stderr" {
		writer = zapcore.AddSync(os.Stderr)
	} else {
		// 文件输出
		file, err := os.OpenFile(conf.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("failed to open log file: %w", err))
		}
		writer = zapcore.AddSync(file)
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writer, level)

	// 创建日志器
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(3),
	)
	klog := NewLogger(logger)

	return log.With(klog, "traceId", tracing.TraceID(), "spanId", tracing.SpanID())
}

type Logger struct {
	logger *zap.Logger
}

func NewLogger(logger *zap.Logger) *Logger {
	return &Logger{
		logger: logger,
	}
}

func (l *Logger) Log(level log.Level, keyvals ...any) error {
	// 将keyvals转换为zap.Field
	fields := make([]zap.Field, 0, len(keyvals)/2)
	msg := ""

	for i := 0; i < len(keyvals); i += 2 {
		if i+1 >= len(keyvals) {
			// 奇数个参数，最后一个作为消息
			msg = fmt.Sprintf("%v", keyvals[i])
			break
		}

		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}

		value := keyvals[i+1]

		// 特殊处理消息字段
		if key == "msg" || key == "message" {
			if msgVal, ok := value.(string); ok {
				msg = msgVal
			}
		} else {
			// 添加为zap字段
			fields = append(fields, zap.Any(key, value))
		}
	}

	// 如果没有显式消息但有奇数个参数，使用最后一个参数作为消息
	if msg == "" && len(keyvals)%2 == 1 {
		msg = fmt.Sprintf("%v", keyvals[len(keyvals)-1])
	}

	switch level {
	case log.LevelDebug:
		l.logger.Debug(msg, fields...)
	case log.LevelInfo:
		l.logger.Info(msg, fields...)
	case log.LevelWarn:
		l.logger.Warn(msg, fields...)
	case log.LevelError:
		l.logger.Error(msg, fields...)
	case log.LevelFatal:
		l.logger.Fatal(msg, fields...)
	default:
		l.logger.Info(msg, fields...)
	}
	return nil
}
