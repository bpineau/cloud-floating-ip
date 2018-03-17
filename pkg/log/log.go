package log

// Logger handle logs, ideally honoring the Quiet config parameter
type Logger interface {
	Infof(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatal(v ...interface{})
}
