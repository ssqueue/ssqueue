package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/metrics"
	"github.com/negasus/tlog"
)

type Service struct {
	h                    *tlog.Handler
	exposeProcessMetrics bool
}

func New(h *tlog.Handler) *Service {
	return &Service{h: h}
}

func (s *Service) Run(ctx context.Context, wg *sync.WaitGroup, ln net.Listener) {
	defer wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", func(_ http.ResponseWriter, _ *http.Request) {})
	mux.HandleFunc("/metrics", func(rw http.ResponseWriter, _ *http.Request) {
		metrics.WritePrometheus(rw, s.exposeProcessMetrics)
	})
	mux.HandleFunc("/log/tag/on", s.handlerTag(s.h.TagOn))
	mux.HandleFunc("/log/tag/off", s.handlerTag(s.h.TagOff))

	server := &http.Server{Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		slog.Info("shutting down service server")
		errShutdown := server.Shutdown(context.Background())
		if errShutdown != nil {
			if errors.Is(errShutdown, context.Canceled) {
				return
			}
			slog.Error("error shutdown service server", slog.String("error", errShutdown.Error()))
		}
	}()

	slog.Info("start service server", slog.String("addr", ln.Addr().String()))
	err := server.Serve(ln)
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("error serve service server", slog.String("error", err.Error()))
		}
	}
}

func (s *Service) handlerTag(tagFunc func(tag string)) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		tag := req.URL.Query().Get("tag")
		if tag == "" {
			http.Error(rw, "tag is required", http.StatusBadRequest)
			return
		}

		tagFunc(tag)

		rw.WriteHeader(http.StatusOK)
		_, errWrite := rw.Write([]byte("ok"))
		if errWrite != nil {
			slog.Error("error write response", slog.String("error", errWrite.Error()))
		}
	}
}
