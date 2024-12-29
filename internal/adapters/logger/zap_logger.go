package logger

import (
	"dev.rubentxu.devops-platform/core/ports"
	"go.uber.org/zap"
)

// ZapLogger implementa la interfaz Logger usando zap
type ZapLogger struct {
	logger *zap.SugaredLogger
}

// NewZapLogger crea una nueva instancia de ZapLogger
func NewZapLogger() (*ZapLogger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	sugar := logger.Sugar()
	return &ZapLogger{logger: sugar}, nil
}

// Debug implementa Logger.Debug
func (l *ZapLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debugw(msg, args...)
}

// Info implementa Logger.Info
func (l *ZapLogger) Info(msg string, args ...interface{}) {
	l.logger.Infow(msg, args...)
}

// Warn implementa Logger.Warn
func (l *ZapLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warnw(msg, args...)
}

// Error implementa Logger.Error
func (l *ZapLogger) Error(msg string, args ...interface{}) {
	l.logger.Errorw(msg, args...)
}

// Fatal implementa Logger.Fatal
func (l *ZapLogger) Fatal(msg string, args ...interface{}) {
	l.logger.Fatalw(msg, args...)
}

// With implementa Logger.With
func (l *ZapLogger) With(args ...interface{}) ports.Logger {
	return &ZapLogger{logger: l.logger.With(args...)}
}
