package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	EmojiStartup  = "🚀"
	EmojiShutdown = "🛑"
	EmojiRequest  = "📥"
	EmojiResponse = "📤"
	EmojiSuccess  = "✅"
	EmojiError    = "❌"
	EmojiWarning  = "⚠️"
	EmojiAuth     = "🔐"
	EmojiUser     = "👤"
	EmojiDatabase = "🗄️"
	EmojiNetwork  = "🌐"
	EmojiMetrics  = "📊"
	EmojiHealth   = "💚"
	EmojiConfig   = "⚙️"
	EmojiOAuth    = "🔑"
	EmojiEmail    = "📧"
	EmojiApple    = "🍎"
	EmojiGoogle   = "🔷"
)

type Logger struct {
	*zap.Logger
}

func New(level string, isProduction bool) (*Logger, error) {
	var config zap.Config

	if isProduction {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

func (l *Logger) WithEmoji(emoji string, msg string) string {
	return emoji + " " + msg
}

func (l *Logger) Startup(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiStartup, msg), fields...)
}

func (l *Logger) Shutdown(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiShutdown, msg), fields...)
}

func (l *Logger) Request(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiRequest, msg), fields...)
}

func (l *Logger) Response(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiResponse, msg), fields...)
}

func (l *Logger) AuthSuccess(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiSuccess+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) AuthError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiError+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) AuthWarning(msg string, fields ...zap.Field) {
	l.Logger.Warn(l.WithEmoji(EmojiWarning+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) EmailAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiEmail, msg), fields...)
}

func (l *Logger) AppleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiApple, msg), fields...)
}

func (l *Logger) GoogleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiGoogle, msg), fields...)
}

func (l *Logger) Health(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiHealth, msg), fields...)
}

func (l *Logger) NetworkError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiNetwork+" "+EmojiError, msg), fields...)
}

func (l *Logger) DatabaseError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiDatabase+" "+EmojiError, msg), fields...)
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

func Must(level string, isProduction bool) *Logger {
	logger, err := New(level, isProduction)
	if err != nil {
		os.Exit(1)
	}
	return logger
}
