package logger

import (
	"fabric/internal/config"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger(cfg *config.Config) *zap.Logger {
	// 初始化编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	// 修改时间编码器为人类可读的 ISO8601 格式
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 配置 lumberjack 实现日志切割
	logPath := filepath.Join(cfg.WorkDir, "logs", "gateway.log")
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath,            // 日志文件路径
		MaxSize:    cfg.Log.MaxSize,    // 单个文件最大尺寸（MB）
		MaxBackups: cfg.Log.MaxBackups, // 最多保留的旧文件个数
		MaxAge:     cfg.Log.MaxAge,     // 最多保留的天数
		Compress:   cfg.Log.Compress,   // 是否开启 gzip 压缩，默认 false
	}

	// 配置文件输出与终端输出
	writeSyncer := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
		zapcore.AddSync(lumberjackLogger),
	)

	// 设定最低日志级别
	levelEnabler := zap.InfoLevel

	// 组装 Core
	core := zapcore.NewCore(encoder, writeSyncer, levelEnabler)

	// 生成 Logger，并注入调用者信息选项
	logger := zap.New(core, zap.AddCaller())
	return logger
}
