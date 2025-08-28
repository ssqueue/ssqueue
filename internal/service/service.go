package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/metrics"
)

type Service struct {
	exposeProcessMetrics bool
}

func New() *Service {
	return &Service{}
}

func (s *Service) Run(ctx context.Context, wg *sync.WaitGroup, ln net.Listener) {
	defer wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/liveness", func(_ http.ResponseWriter, _ *http.Request) {})
	mux.HandleFunc("/metrics", func(rw http.ResponseWriter, _ *http.Request) {
		metrics.WritePrometheus(rw, s.exposeProcessMetrics)
	})

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
