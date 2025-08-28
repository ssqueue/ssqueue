package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"

	"github.com/VictoriaMetrics/metrics"
	"github.com/negasus/tlog"

	"github.com/ssqueue/ssqueue/internal/application"
	"github.com/ssqueue/ssqueue/internal/config"
	"github.com/ssqueue/ssqueue/internal/front/http"
	"github.com/ssqueue/ssqueue/internal/service"
)

var version = "undefined"

func main() {
	metrics.GetOrCreateCounter("ssqueue_info{version=\"" + version + "\"}").Inc()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg := config.Load()

	logOpts := tlog.Options{
		AttrColor: tlog.TextColorGreen,
	}

	if cfg.Debug {
		logOpts.ShowSource = true
		logOpts.TrimSource = true
		logOpts.Level = slog.LevelDebug
	}

	h := tlog.New(&logOpts)
	slog.SetDefault(slog.New(h))

	errRun := run(ctx, cfg)
	if errRun != nil {
		slog.Error("error run application", "err", errRun)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config) error {
	var wg sync.WaitGroup

	slog.Info("starting application", "version", version)

	app := application.New()

	if !cfg.Snapshot.Disable {
		errSnapshot := fromSnapshot(cfg.Snapshot.Path, app)
		if errSnapshot != nil {
			slog.Error("error restore from snapshot", "err", errSnapshot)
		}
	}

	wg.Add(1)
	go app.Run(ctx, &wg)

	lnMain, errLnMain := net.Listen("tcp", cfg.Address)
	if errLnMain != nil {
		return errLnMain
	}
	defer lnMain.Close()

	srvMain := http.New(app)
	wg.Add(1)
	go srvMain.Run(ctx, &wg, lnMain)

	if cfg.ServiceAddress != "" {
		lnService, errLnService := net.Listen("tcp", cfg.ServiceAddress)
		if errLnService != nil {
			return errLnService
		}
		defer lnService.Close()

		srv := service.New()

		wg.Add(1)
		go srv.Run(ctx, &wg, lnService)
	}

	<-ctx.Done()

	wg.Wait()

	if !cfg.Snapshot.Disable {
		errToSnapshot := toSnapshot(cfg.Snapshot.Path, app)
		if errToSnapshot != nil {
			slog.Error("error save to snapshot", "err", errToSnapshot)
		}
	}

	slog.Info("done")

	return nil
}
