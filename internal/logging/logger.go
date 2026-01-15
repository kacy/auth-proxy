package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Emoji constants for log categorization
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

// Logger wraps zap.Logger with emoji-enhanced methods.
type Logger struct {
	*zap.Logger
}

// New creates a new Logger instance.
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

	// Set log level
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

	// Output to stdout for container logging
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

// WithEmoji adds an emoji prefix to the message.
func (l *Logger) WithEmoji(emoji string, msg string) string {
	return emoji + " " + msg
}

// Startup logs a startup message.
func (l *Logger) Startup(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiStartup, msg), fields...)
}

// Shutdown logs a shutdown message.
func (l *Logger) Shutdown(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiShutdown, msg), fields...)
}

// Request logs an incoming request.
func (l *Logger) Request(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiRequest, msg), fields...)
}

// Response logs an outgoing response.
func (l *Logger) Response(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiResponse, msg), fields...)
}

// AuthSuccess logs a successful authentication.
func (l *Logger) AuthSuccess(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiSuccess+" "+EmojiAuth, msg), fields...)
}

// AuthError logs an authentication error.
func (l *Logger) AuthError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiError+" "+EmojiAuth, msg), fields...)
}

// AuthWarning logs an authentication warning.
func (l *Logger) AuthWarning(msg string, fields ...zap.Field) {
	l.Logger.Warn(l.WithEmoji(EmojiWarning+" "+EmojiAuth, msg), fields...)
}

// EmailAuth logs email authentication events.
func (l *Logger) EmailAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiEmail, msg), fields...)
}

// AppleAuth logs Apple authentication events.
func (l *Logger) AppleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiApple, msg), fields...)
}

// GoogleAuth logs Google authentication events.
func (l *Logger) GoogleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiGoogle, msg), fields...)
}

// Health logs health check events.
func (l *Logger) Health(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiHealth, msg), fields...)
}

// NetworkError logs network-related errors.
func (l *Logger) NetworkError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiNetwork+" "+EmojiError, msg), fields...)
}

// DatabaseError logs database-related errors.
func (l *Logger) DatabaseError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiDatabase+" "+EmojiError, msg), fields...)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Must creates a logger or panics.
func Must(level string, isProduction bool) *Logger {
	logger, err := New(level, isProduction)
	if err != nil {
		os.Exit(1)
	}
	return logger
}
