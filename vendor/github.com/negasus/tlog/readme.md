# TLog

> WIP

This is a handler for golang structure log

# Usage

Simple usage

```go
package main

import (
	"log/slog"
    "github.com/negasus/tlog"
)

func main() {
	h := tlog.New(nil)
	h.SetLevel(slog.LevelDebug)

	slog.SetDefault(slog.New(h))

	slog.Debug("Hello, world!", "key", "value")
}
```
