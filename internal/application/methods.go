package application

import (
	"context"
	"crypto/rand"
	"errors"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"

	"github.com/ssqueue/ssqueue/internal/messages"
	"github.com/ssqueue/ssqueue/internal/queue"
)

var (
	ErrNoConsumers = errors.New("no consumers")
	ErrNotReady    = errors.New("not ready")
)

func (app *Application) Get(ctx context.Context, topic string) (*messages.OutputMessage, error) {
	if atomic.LoadInt64(&app.ready) != 1 {
		return nil, ErrNotReady
	}

	metrics.GetOrCreateCounter("ssqueue_method_get{topic=\"" + topic + "\"}").Inc()

	q := app.getQueue(topic)
	q.Inc()
	defer q.Dec()

	item := q.Pop(ctx)

	if item == nil {
		return nil, nil
	}

	return &messages.OutputMessage{ID: item.ID, Data: item.Data}, nil
}

func (app *Application) Send(_ context.Context, topic string, im *messages.InputMessage) (string, error) {
	if atomic.LoadInt64(&app.ready) != 1 {
		return "", ErrNotReady
	}

	metrics.GetOrCreateCounter("ssqueue_method_send{topic=\"" + topic + "\"}").Inc()

	item := queue.AcquireItem()
	item.ID = rand.Text()
	item.Data = im.Data

	added := app.getQueue(topic).Push(item, im.Persistent)
	if !added {
		return "", ErrNoConsumers
	}

	return item.ID, nil
}
