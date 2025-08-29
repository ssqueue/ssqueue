package queue

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
)

var itemsPool = sync.Pool{}

func AcquireItem() *Item {
	if v := itemsPool.Get(); v != nil {
		return v.(*Item)
	}
	return &Item{}
}

func ReleaseItem(item *Item) {
	item.reset()
	itemsPool.Put(item)
}

type Item struct {
	ID   string `json:"id,omitempty"`
	Data string `json:"data,omitempty"`
	Name string `json:"name,omitempty"`
}

func (i *Item) reset() {
	i.ID = ""
	i.Data = ""
}

type Queue struct {
	topic          string
	mu             sync.RWMutex
	items          []*Item
	notify         chan struct{}
	consumersCount int64
	count          int64
}

func New(topic string) *Queue {
	return &Queue{
		topic:  topic,
		items:  make([]*Item, 0, 256),
		notify: make(chan struct{}),
	}
}

func (q *Queue) FromSnapshot(src []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return json.Unmarshal(src, &q.items)
}

func (q *Queue) ToSnapshot() ([]byte, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, nil
	}

	return json.Marshal(q.items)
}

func (q *Queue) ConsumersCount() int {
	return int(atomic.LoadInt64(&q.consumersCount))
}

func (q *Queue) Count() int {
	return int(atomic.LoadInt64(&q.count))
}

func (q *Queue) Inc() {
	atomic.AddInt64(&q.consumersCount, 1)
}

func (q *Queue) Dec() {
	atomic.AddInt64(&q.consumersCount, -1)
}

func (q *Queue) Push(item *Item, isPersistent bool) bool {
	if !isPersistent && q.ConsumersCount() == 0 {
		return false
	}

	q.mu.Lock()
	q.items = append(q.items, item)
	atomic.AddInt64(&q.count, 1)
	q.mu.Unlock()
	q.signal()

	return true
}

func (q *Queue) Pop(ctx context.Context) *Item {
	for {
		q.mu.Lock()
		if len(q.items) > 0 {
			v := q.items[0]
			q.items = q.items[1:]
			atomic.AddInt64(&q.count, -1)
			q.mu.Unlock()
			return v
		}
		q.mu.Unlock()

		select {
		case <-ctx.Done():
			return nil
		case <-q.notify:
		}
	}
}

func (q *Queue) signal() {
	q.mu.Lock()
	defer q.mu.Unlock()

	close(q.notify)
	q.notify = make(chan struct{})
}
