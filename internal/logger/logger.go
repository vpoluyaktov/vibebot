package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // Cyan
	INFO:  "\033[32m", // Green
	WARN:  "\033[33m", // Yellow
	ERROR: "\033[31m", // Red
	FATAL: "\033[35m", // Magenta
}

const colorReset = "\033[0m"

// Logger is a structured logger with level support
type Logger struct {
	level      LogLevel
	useColors  bool
	output     io.Writer
	prefix     string
	timeFormat string
}

var defaultLogger *Logger

func init() {
	defaultLogger = New(INFO, true, os.Stdout, "")
}

// New creates a new Logger instance
func New(level LogLevel, useColors bool, output io.Writer, prefix string) *Logger {
	return &Logger{
		level:      level,
		useColors:  useColors,
		output:     output,
		prefix:     prefix,
		timeFormat: "2006/01/02 15:04:05",
	}
}

// SetLevel sets the minimum log level for the default logger
func SetLevel(level LogLevel) {
	defaultLogger.level = level
}

// SetLevelFromString sets the log level from a string (debug, info, warn, error, fatal)
func SetLevelFromString(levelStr string) error {
	switch strings.ToLower(levelStr) {
	case "debug":
		SetLevel(DEBUG)
	case "info":
		SetLevel(INFO)
	case "warn", "warning":
		SetLevel(WARN)
	case "error":
		SetLevel(ERROR)
	case "fatal":
		SetLevel(FATAL)
	default:
		return fmt.Errorf("invalid log level: %s", levelStr)
	}
	return nil
}

// SetOutput sets the output writer for the default logger
func SetOutput(w io.Writer) {
	defaultLogger.output = w
}

// SetPrefix sets a prefix for all log messages
func SetPrefix(prefix string) {
	defaultLogger.prefix = prefix
}

// log writes a log message at the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(l.timeFormat)
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)

	var logLine string
	if l.useColors {
		color := levelColors[level]
		logLine = fmt.Sprintf("%s [%s%s%s] %s%s\n",
			timestamp, color, levelName, colorReset, l.prefix, message)
	} else {
		logLine = fmt.Sprintf("%s [%s] %s%s\n",
			timestamp, levelName, l.prefix, message)
	}

	fmt.Fprint(l.output, logLine)

	// Fatal logs should exit
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

// Package-level convenience functions using the default logger

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

// Fatal logs a fatal message and exits using the default logger
func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(format, args...)
}

// WithPrefix creates a new logger with the given prefix
func WithPrefix(prefix string) *Logger {
	return &Logger{
		level:      defaultLogger.level,
		useColors:  defaultLogger.useColors,
		output:     defaultLogger.output,
		prefix:     prefix,
		timeFormat: defaultLogger.timeFormat,
	}
}

// DisableStdLog disables the standard library's log package default output
func DisableStdLog() {
	log.SetOutput(io.Discard)
}
