package front

import (
	"context"

	"github.com/ssqueue/ssqueue/internal/messages"
)

type Application interface {
	Get(ctx context.Context, topic string) (om *messages.OutputMessage, err error)
	Send(ctx context.Context, topic string, im *messages.InputMessage) (id string, err error)
}
