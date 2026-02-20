package repositories

// Logger is the minimal logging contract required by repository implementations.
// It is satisfied by the platform's monitoring/logging.Logger and by most
// structured-logging libraries.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}
