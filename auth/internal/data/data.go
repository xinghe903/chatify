package data

import (
	"context"
	"fmt"
	"time"

	"auth/internal/biz"
	"auth/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormLogAdapter 是一个适配器，将Kratos的logger转换为GORM可用的logger
// 实现了gorm.io/gorm/logger.Writer接口
type gormLogAdapter struct {
	logger log.Logger
}

// Printf 实现gorm.io/gorm/logger.Writer接口的Printf方法
func (l *gormLogAdapter) Printf(format string, args ...interface{}) {
	l.logger.Log(log.LevelDebug, fmt.Sprintf(format, args...))
}

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewUserRepo, NewCacheRepo)

// Data 数据层主结构
type Data struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewData 创建数据层实例
func NewData(cb *conf.Bootstrap, logger log.Logger) (*Data, func(), error) {
	// 初始化MySQL客户端
	db, err := initMySQLClient(cb.Data, logger)
	if err != nil {
		return nil, nil, err
	}

	// 初始化Redis客户端
	redisClient, err := initRedisClient(cb.Data, logger)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.NewHelper(logger).Errorf("Failed to close redis client: %v", err)
			}
		}
		log.NewHelper(logger).Info("closing the data resources")
	}

	return &Data{
			db:    db,
			redis: redisClient,
		},
		cleanup,
		nil
}

// initMySQLClient 初始化MySQL客户端
func initMySQLClient(c *conf.Data, logg log.Logger) (*gorm.DB, error) {
	// 使用配置文件中的数据库连接字符串
	dsn := ""
	mysqlConfig := c.Database
	if mysqlConfig != nil && mysqlConfig.Source != "" {
		dsn = mysqlConfig.Source
	} else {
		// 默认配置（作为备选）
		dsn = "root:password@tcp(localhost:3306)/auth?charset=utf8mb4&parseTime=True&loc=Local"
	}

	// 使用默认的GORM日志级别（Info级别）

	// 创建GORM日志适配器
	logAdapter := &gormLogAdapter{logger: logg}

	// 创建GORM配置
	dbLogger := logger.New(
		logAdapter,
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// 连接MySQL
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: dbLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mysql: %w", err)
	}

	// 获取原始连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get db instance: %w", err)
	}

	// 配置连接池（使用默认值）
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(time.Minute * 30)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	log.NewHelper(logg).Info("MySQL client initialized successfully")

	return db, nil
}

// initRedisClient 初始化Redis客户端
func initRedisClient(c *conf.Data, logg log.Logger) (*redis.Client, error) {
	// 默认配置
	network := "tcp"
	addr := "localhost:6379"
	readTimeout := 3 * time.Second
	writeTimeout := 3 * time.Second

	// 从配置中读取Redis参数
	redisConfig := c.Redis
	if redisConfig != nil {
		if redisConfig.Network != "" {
			network = redisConfig.Network
		}
		if redisConfig.Addr != "" {
			addr = redisConfig.Addr
		}
		if redisConfig.ReadTimeout != nil {
			readTimeout = redisConfig.ReadTimeout.AsDuration()
		}
		if redisConfig.WriteTimeout != nil {
			writeTimeout = redisConfig.WriteTimeout.AsDuration()
		}
	}

	client := redis.NewClient(&redis.Options{
		Network:      network,
		Addr:         addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Password:     "", // 默认无密码，实际环境中应从配置读取
		DB:           0,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	log.NewHelper(logg).Info("Redis client initialized successfully")

	return client, nil
}

// NewCacheRepo 创建缓存仓库
func NewCacheRepo(data *Data) biz.CacheRepo {
	return &cacheRepo{data: data}
}

// cacheRepo 缓存仓库实现
type cacheRepo struct {
	data *Data
}

// GetClient 获取Redis客户端
func (c *cacheRepo) GetClient() *redis.Client {
	return c.data.redis
}
