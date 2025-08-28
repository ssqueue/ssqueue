package tlog

import (
	"io"
	"log/slog"
)

type Options struct {
	ShowSource  bool
	TrimSource  bool
	Level       slog.Level
	AttrColor   TextColor
	LevelColor  map[slog.Level]TextColor
	TagAttrName string
	TimeFormat  string
	Writer      io.Writer
}
