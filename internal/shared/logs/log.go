package logs

import (
	"io"
	"os"
	"strings"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"ThreeKingdoms/internal/shared/config"
)

var logger *zap.Logger = zap.NewNop()

func Init(appName string, cfg config.LogConfig) error {
	// 1) 解析日志级别：默认是 info
	//    cfg.Level 支持 "debug/info/warn/error/..."（大小写不敏感）
	lvl := zapcore.InfoLevel
	if err := lvl.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		// 解析失败则回退到 info
		lvl = zapcore.InfoLevel
	}
	// 使用 AtomicLevel 方便未来动态调整日志级别（例如热更新）
	atomicLevel := zap.NewAtomicLevelAt(lvl)

	// 2) console 和 file 共用的编码器配置（字段名、时间格式、caller 等）
	//    2026-01-28T10:00:00 INFO  gate-server  server start  login_main.go:12
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",                           // 时间字段 key
		LevelKey:       "level",                        // 日志级别字段 key
		NameKey:        "logger",                       // logger 名称字段 key（一般用不到也可以留着）
		CallerKey:      "caller",                       // 调用位置字段 key
		MessageKey:     "msg",                          // 日志消息字段 key
		StacktraceKey:  "stack",                        // 堆栈字段 key
		LineEnding:     zapcore.DefaultLineEnding,      // 行尾（默认 \n）
		EncodeTime:     zapcore.ISO8601TimeEncoder,     // 时间格式：ISO8601
		EncodeDuration: zapcore.SecondsDurationEncoder, // duration：以秒输出
		EncodeCaller:   zapcore.ShortCallerEncoder,     // caller：短路径（xx.go:123）
	}

	// 3) 控制台编码器（Console Encoder）：为了可读性，
	//    CapitalColorLevelEncoder 不同颜色输出 INFO/WARN/ERROR
	consoleCfg := encoderCfg
	consoleCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(consoleCfg)

	// 4) 文件编码器（File Encoder）：JSON 结构化输出
	//    CapitalLevelEncoder：不带颜色输出 INFO/WARN/ERROR
	fileCfg := encoderCfg
	fileCfg.EncodeLevel = zapcore.CapitalLevelEncoder
	jsonEncoder := zapcore.NewJSONEncoder(fileCfg)

	// 5) 设置文件输出（带切割）：使用 lumberjack
	//    如果没有配置文件路径，则丢弃文件输出（只输出到控制台）
	var fileWriter io.Writer
	if cfg.FileDir != "" {
		fileWriter = &lumberjack.Logger{
			Filename:   cfg.FileDir,
			MaxSize:    max(1, cfg.MaxSize),    // 单个文件最大大小（MB），至少 1
			MaxBackups: max(0, cfg.MaxBackups), // 最多保留多少个旧文件
			MaxAge:     max(0, cfg.MaxAge),     // 最多保留多少天的旧文件
			Compress:   cfg.Compress,           // 是否压缩旧文件
		}
	} else {
		fileWriter = io.Discard
	}

	// 6) 组合输出目的地：控制台 + 文件
	//    console 用 os.Stderr；并用 Lock 保证并发写安全
	consoleSyncer := zapcore.Lock(os.Stderr)
	//    fileSyncer：把 io.Writer 包装成 zap 的 WriteSyncer
	fileSyncer := zapcore.AddSync(fileWriter)
	//    multiSyncer：把两路写入合并（同一条日志写到两个地方）
	multiSyncer := zapcore.NewMultiWriteSyncer(consoleSyncer, fileSyncer)

	// 7) 构建 core：
	//    - 如果不写文件：用 consoleEncoder + multiSyncer（实际上 file 是 discard）
	//    - 如果写文件：用 NewTee 分成两路 core：
	//        * 控制台：consoleEncoder + consoleSyncer（彩色）
	//        * 文件：jsonEncoder + fileSyncer（JSON）
	//
	//    这样做的好处是：不会把带颜色的 ANSI 转义写进日志文件。
	core := zapcore.NewCore(consoleEncoder, multiSyncer, atomicLevel)
	if cfg.FileDir != "" {
		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, consoleSyncer, atomicLevel),
			zapcore.NewCore(jsonEncoder, fileSyncer, atomicLevel),
		)
	}

	// 8) zap 选项：
	//    - AddCaller：每条日志带调用文件:行号
	//    - 开发模式：更适合开发调试；并让 warn 及以上自动带堆栈（方便定位）
	opts := []zap.Option{zap.AddCaller()}
	if cfg.Dev {
		opts = append(opts, zap.Development(), zap.AddStacktrace(zapcore.WarnLevel))
	}

	// 9) 创建 logger
	l := zap.New(core, opts...).Named(appName)

	// 10) 替换全局 logger：如果之前初始化过，先 Sync 刷盘
	if l != nil {
		_ = l.Sync()
	}
	logger = l
	return nil
}

// 常用日志级别的辅助函数（便捷封装）。
// 这些函数只是对底层 logger 的同名方法做了一层包装，方便业务侧调用。
// 当 logger 还没初始化（logger == nil）时，这些函数会直接什么都不做（no-op），避免空指针 panic。

// Debug：输出 Debug 级别日志。
// fields 用 zap.String / zap.Int 等构造结构化字段。
func Debug(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Debug(msg, fields...)
	}
}

// Info：输出 Info 级别日志。
// 建议使用 zap.String、zap.Int、zap.Duration 等构造强类型字段。
// 示例：Info("user logged in", zap.String("userID", id))
func Info(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Info(msg, fields...)
	}
}

// Warn：输出 Warn 级别日志。
func Warn(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Warn(msg, fields...)
	}
}

// Error：输出 Error 级别日志。
func Error(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Error(msg, fields...)
	}
}

// DPanic：输出 DPanic 级别日志。
// 在开发模式（Development=true / zap.Development()）下，DPanic 会触发 panic（更容易暴露问题）。
func DPanic(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.DPanic(msg, fields...)
	}
}

// Panic：输出 Panic 级别日志，然后直接 panic。
func Panic(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Panic(msg, fields...)
	}
}

// Fatal：输出 Fatal 级别日志，然后退出程序（os.Exit(1)）。
func Fatal(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Fatal(msg, fields...)
	}
}
