package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/ssqueue/ssqueue/internal/queue"
)

type Application struct {
	ready int64
	qMu   sync.RWMutex
	q     map[string]*queue.Queue
}

func New() *Application {
	app := &Application{
		q: make(map[string]*queue.Queue),
	}

	return app
}

func (app *Application) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	atomic.StoreInt64(&app.ready, 1)

	slog.Info("application started")

	<-ctx.Done()

	slog.Info("application shutting down")

	atomic.StoreInt64(&app.ready, 0)
}

func (app *Application) FromSnapshot(src []byte) error {

	snapshots := make(map[string]string)
	errDecode := json.Unmarshal(src, &snapshots)
	if errDecode != nil {
		return errDecode
	}

	for topic, data := range snapshots {
		q := app.getQueue(topic)
		app.qMu.Lock()
		err := q.FromSnapshot([]byte(data))
		app.qMu.Unlock()
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) ToSnapshot() ([]byte, error) {
	app.qMu.RLock()
	defer app.qMu.RUnlock()

	snapshots := make(map[string]string)

	for topic, q := range app.q {
		data, err := q.ToSnapshot()
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			continue
		}
		snapshots[topic] = string(data)
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	return json.Marshal(snapshots)
}

func (app *Application) getQueue(topic string) *queue.Queue {
	app.qMu.RLock()
	q, ok := app.q[topic]
	app.qMu.RUnlock()
	if ok {
		return q
	}

	app.qMu.Lock()
	defer app.qMu.Unlock()

	q, ok = app.q[topic]
	if ok {
		return q
	}

	q = queue.New(topic)
	app.q[topic] = q

	return q
}
