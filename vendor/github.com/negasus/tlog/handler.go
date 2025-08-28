package tlog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"sync"
)

type TextColor string

const (
	textColorReset TextColor = "\033[0m"

	TextColorBlack         TextColor = "\033[30m"
	TextColorRed           TextColor = "\033[31m"
	TextColorGreen         TextColor = "\033[32m"
	TextColorYellow        TextColor = "\033[33m"
	TextColorBlue          TextColor = "\033[34m"
	TextColorMagenta       TextColor = "\033[35m"
	TextColorCyan          TextColor = "\033[36m"
	TextColorWhite         TextColor = "\033[37m"
	TextColorBrightBlack   TextColor = "\033[90m"
	TextColorBrightRed     TextColor = "\033[91m"
	TextColorBrightGreen   TextColor = "\033[92m"
	TextColorBrightYellow  TextColor = "\033[93m"
	TextColorBrightBlue    TextColor = "\033[94m"
	TextColorBrightMagenta TextColor = "\033[95m"
	TextColorBrightCyan    TextColor = "\033[96m"
	TextColorBrightWhite   TextColor = "\033[97m"
)

var defaultHandler *Handler

func DefaultHandler() *Handler {
	if defaultHandler == nil {
		defaultHandler = New(nil)
	}
	return defaultHandler
}

type Handler struct {
	wMx   sync.RWMutex
	w     io.Writer
	group string
	attrs []slog.Attr

	trimSource   bool
	showSource   bool
	sourcePrefix int
	tagAttrName  string
	timeFormat   string

	levelMx sync.RWMutex
	level   slog.Level

	tagsMx sync.RWMutex
	tags   map[string]struct{}

	attrColor TextColor
	colors    map[slog.Level]TextColor
}

func (h *Handler) TagOn(tag string) {
	h.tagsMx.Lock()
	defer h.tagsMx.Unlock()
	h.tags[tag] = struct{}{}
}

func (h *Handler) TagOff(tag string) {
	h.tagsMx.Lock()
	defer h.tagsMx.Unlock()
	delete(h.tags, tag)
}

func New(opts *Options) *Handler {
	h := Handler{
		w:           os.Stdout,
		tagAttrName: "tag",
		tags:        make(map[string]struct{}),
		colors: map[slog.Level]TextColor{
			slog.LevelDebug: TextColorBrightMagenta,
			slog.LevelInfo:  TextColorBlue,
			slog.LevelWarn:  TextColorYellow,
			slog.LevelError: TextColorRed,
		},
		timeFormat: "2006-01-02 15:04:05",
	}

	if opts != nil {
		h.showSource = opts.ShowSource
		h.trimSource = opts.TrimSource
		h.level = opts.Level
		h.attrColor = opts.AttrColor
		for k, v := range opts.LevelColor {
			h.colors[k] = v
		}
		if opts.TagAttrName != "" {
			h.tagAttrName = opts.TagAttrName
		}
		if opts.TimeFormat != "" {
			h.timeFormat = opts.TimeFormat
		}
		if opts.Writer != nil {
			h.w = opts.Writer
		}
	}

	if h.trimSource {
		dir, err := os.Getwd()
		if err == nil {
			h.sourcePrefix = len(dir) + 1
		}
	}

	return &h
}

func (h *Handler) SetLevel(level slog.Level) {
	h.levelMx.Lock()
	defer h.levelMx.Unlock()
	h.level = level
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	h.levelMx.RLock()
	defer h.levelMx.RUnlock()
	return h.level <= level
}

func (h *Handler) Handle(_ context.Context, record slog.Record) error {
	var tagValue string
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Value.Kind() == slog.KindString && attr.Key == h.tagAttrName {
			tagValue = attr.Value.String()
			return false
		}
		return true
	})

	if tagValue != "" {
		h.tagsMx.RLock()
		_, ok := h.tags[tagValue]
		h.tagsMx.RUnlock()
		if !ok {
			return nil
		}
	}

	buf := bytes.NewBuffer(nil)

	tm := record.Time.Format(h.timeFormat)
	buf.WriteString(tm)
	buf.WriteByte('\t')

	clr, okClr := h.colors[record.Level]
	if okClr {
		buf.WriteString(string(clr))
		buf.WriteString(record.Level.String())
		buf.WriteString(string(textColorReset))
	} else {
		buf.WriteString(record.Level.String())
	}
	buf.WriteByte('\t')

	if h.showSource {
		_, file, line, ok := runtime.Caller(3)
		if ok {
			if h.sourcePrefix > 0 {
				file = file[h.sourcePrefix:]
			}

			buf.WriteString(file)
			buf.WriteByte(':')
			buf.WriteString(strconv.Itoa(line))
			buf.WriteByte('\t')
		}
	}
	buf.WriteString(record.Message)

	if record.NumAttrs() == 0 {
		buf.WriteByte('\n')

		h.wMx.Lock()
		defer h.wMx.Unlock()
		_, err := h.w.Write(buf.Bytes())
		return err
	}

	if h.attrColor != "" {
		buf.WriteString(string(h.attrColor))
	}

	buf.WriteString("\t{")

	var attrIdx int
	for _, attr := range h.attrs {
		h.writeAttr(attrIdx, buf, attr)
		attrIdx++
	}

	record.Attrs(func(attr slog.Attr) bool {
		h.writeAttr(attrIdx, buf, attr)
		attrIdx++
		return true
	})

	buf.WriteByte('}')

	if h.attrColor != "" {
		buf.WriteString(string(textColorReset))
	}

	buf.WriteByte('\n')

	h.wMx.Lock()
	defer h.wMx.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *Handler) writeAttr(idx int, buf *bytes.Buffer, attr slog.Attr) {
	if idx > 0 {
		buf.WriteString(", ")
	}
	if len(h.group) > 0 {
		buf.WriteString(h.group)
		buf.WriteByte('.')
	}
	buf.WriteString(attr.Key)
	buf.WriteByte('=')
	buf.WriteString(attr.Value.String())
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := h.clone()
	newH.attrs = append(newH.attrs, attrs...)
	return newH
}

func (h *Handler) WithGroup(name string) slog.Handler {
	newHandler := h.clone()
	if len(newHandler.group) > 0 {
		newHandler.group += "."
	}
	newHandler.group += name
	return newHandler
}

func (h *Handler) clone() *Handler {
	newHandler := Handler{
		wMx:          sync.RWMutex{},
		w:            h.w,
		group:        h.group,
		attrs:        make([]slog.Attr, len(h.attrs)),
		trimSource:   h.trimSource,
		showSource:   h.showSource,
		sourcePrefix: h.sourcePrefix,
		tagAttrName:  h.tagAttrName,
		timeFormat:   h.timeFormat,
		levelMx:      sync.RWMutex{},
		level:        h.level,
		tagsMx:       sync.RWMutex{},
		tags:         make(map[string]struct{}, len(h.tags)),
		attrColor:    h.attrColor,
		colors:       make(map[slog.Level]TextColor),
	}
	copy(newHandler.attrs, h.attrs)
	for k := range h.tags {
		newHandler.tags[k] = struct{}{}
	}
	for k, v := range h.colors {
		newHandler.colors[k] = v
	}
	return &newHandler
}
