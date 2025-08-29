package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ssqueue/ssqueue/internal/application"
	"github.com/ssqueue/ssqueue/internal/front"
	"github.com/ssqueue/ssqueue/internal/messages"
)

const (
	defaultGetTimeout = time.Second * 20
)

type HTTP struct {
	app front.Application
}

func New(app front.Application) *HTTP {
	return &HTTP{
		app: app,
	}
}

func sendResponse(rw http.ResponseWriter, status int, v any) {
	data, errEncode := json.Marshal(v)
	if errEncode != nil {
		slog.Error("error encode output message", slog.String("error", errEncode.Error()))
		http.Error(rw, "error encode output message", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_, errWrite := rw.Write(data)
	if errWrite != nil {
		slog.Error("error write response", slog.String("error", errWrite.Error()))
	}
}

func (h *HTTP) Run(ctx context.Context, wg *sync.WaitGroup, ln net.Listener) {
	defer wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/send", h.handlerSend)
	mux.HandleFunc("/api/v1/get", h.handlerGet)

	server := &http.Server{Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		slog.Info("shutting down api server")
		errShutdown := server.Shutdown(context.Background())
		if errShutdown != nil {
			if errors.Is(errShutdown, context.Canceled) {
				return
			}
			slog.Error("error shutdown api server", slog.String("error", errShutdown.Error()))
		}
	}()

	slog.Info("start api server", slog.String("addr", ln.Addr().String()))
	err := server.Serve(ln)
	if err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("error serve api server", slog.String("error", err.Error()))
		}
	}
}

func (h *HTTP) handlerSend(rw http.ResponseWriter, req *http.Request) {
	type request struct {
		Name       string `json:"name"`
		Topic      string `json:"topic"`
		Data       string `json:"data"`
		Persistent bool   `json:"persistent"`
	}

	type response struct {
		ID string `json:"id"`
	}

	r := request{}

	errDecode := json.NewDecoder(req.Body).Decode(&r)
	if errDecode != nil {
		http.Error(rw, "bad request, invalid message", http.StatusBadRequest)
		return
	}

	internalID, err := h.app.Send(req.Context(), r.Topic, &messages.InputMessage{Data: r.Data, Persistent: r.Persistent, Name: r.Name})
	if err != nil {
		if errors.Is(err, application.ErrNoConsumers) {
			http.Error(rw, err.Error(), http.StatusGone)
			return
		}
		slog.Error("error send message", slog.String("error", err.Error()))
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}

	sendResponse(rw, http.StatusCreated, response{ID: internalID})
}

func (h *HTTP) handlerGet(rw http.ResponseWriter, req *http.Request) {
	type response struct {
		ID   string `json:"id"`
		From string `json:"from"`
		Data string `json:"data"`
	}

	name := req.URL.Query().Get("name")
	topic := req.URL.Query().Get("topic")

	timeout := defaultGetTimeout

	timeoutStr := req.URL.Query().Get("timeout")
	if timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			http.Error(rw, "bad request, invalid timeout", http.StatusBadRequest)
			return
		}
	}

	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()

	om, err := h.app.Get(ctx, topic)
	if err != nil {
		if errors.Is(err, application.ErrNotReady) {
			http.Error(rw, err.Error(), http.StatusServiceUnavailable)
			return
		}
		slog.Error("error get message", slog.String("error", err.Error()))
		http.Error(rw, "internal error", http.StatusInternalServerError)
		return
	}
	if om == nil {
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	slog.Log(ctx, slog.LevelInfo+1, "receive message", "tag", "trace", slog.String("topic", topic), slog.String("consumer", name), slog.String("producer", om.Name))

	sendResponse(rw, http.StatusOK, response{ID: om.ID, From: om.Name, Data: om.Data})
}
