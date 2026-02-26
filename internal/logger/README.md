# Logger Package

A simple, structured logging package with level support and colored output.

## Features

- **Log Levels**: DEBUG, INFO, WARN, ERROR, FATAL
- **Colored Output**: Color-coded log levels for better readability
- **Configurable**: Set log level via environment variable
- **Prefixes**: Support for logger prefixes (e.g., per-module loggers)
- **Standard Interface**: Familiar Printf-style API

## Usage

### Basic Logging

```go
import "github.com/vpoluyaktov/vibebot/internal/logger"

logger.Debug("Debug message: %s", value)
logger.Info("Info message: %s", value)
logger.Warn("Warning message: %s", value)
logger.Error("Error message: %v", err)
logger.Fatal("Fatal error: %v", err) // Exits with code 1
```

### Configuration

Set log level via environment variable:

```bash
export LOG_LEVEL=debug  # Options: debug, info, warn, error, fatal
```

Or programmatically:

```go
logger.SetLevel(logger.DEBUG)
// or
logger.SetLevelFromString("debug")
```

### Custom Logger with Prefix

```go
// Create a logger with a prefix for a specific module
moduleLogger := logger.WithPrefix("[MyModule] ")
moduleLogger.Info("This is a module-specific log")
```

### Disable Standard Library Log

```go
// Disable output from the standard library's log package
logger.DisableStdLog()
```

## Log Levels

| Level | Description | Color |
|-------|-------------|-------|
| DEBUG | Detailed debugging information | Cyan |
| INFO  | General informational messages | Green |
| WARN  | Warning messages | Yellow |
| ERROR | Error messages | Red |
| FATAL | Fatal errors (exits program) | Magenta |

## Output Format

```
2026/02/26 15:52:10 [INFO] Workspace: /home/ubuntu/.vibebot/workspace
2026/02/26 15:52:10 [WARN] No user whitelist configured
2026/02/26 15:52:10 [ERROR] Failed to connect: connection refused
```

With colors enabled (default), log levels are color-coded for better visibility in terminals.

## Implementation Details

- Thread-safe (uses standard library's fmt.Fprint)
- Minimal dependencies (only standard library)
- Configurable output writer (defaults to stdout)
- Customizable timestamp format
- FATAL level calls os.Exit(1) after logging
