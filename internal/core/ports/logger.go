package ports

// Logger define la interfaz para logging
type Logger interface {
	// Debug logs a message at debug level
	Debug(msg string, args ...interface{})
	
	// Info logs a message at info level
	Info(msg string, args ...interface{})
	
	// Warn logs a message at warn level
	Warn(msg string, args ...interface{})
	
	// Error logs a message at error level
	Error(msg string, args ...interface{})
	
	// Fatal logs a message at fatal level and terminates the program
	Fatal(msg string, args ...interface{})

	// With adds key-value pairs to the logger
	With(args ...interface{}) Logger
} 