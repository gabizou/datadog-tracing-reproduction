package fanout

import (
	"context"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	kafkatrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/confluentinc/confluent-kafka-go/kafka.v2"
	"math/rand/v2"
	"time"
)

type FanOutEntityInput struct {
	Offset     uint32
	PageNumber uint32
}

type FanOutEntityOutput struct {
	Offset     uint32
	HasMore    bool
	PageNumber uint32
}

type Heartbeat = func(ctx context.Context, offset uint32) error

type Service interface {
	FanOutEntities(ctx context.Context, offset uint32, hb Heartbeat) (*FanOutEntityOutput, error)
}

func NewService(repo Repository, producer *kafkatrace.Producer) Service {
	return &svc{repo: repo, producer: producer}
}

type svc struct {
	repo     Repository
	producer *kafkatrace.Producer
}

var _entityTopic = "EntityUpserted"

func (s *svc) FanOutEntities(ctx context.Context, offset uint32, hb Heartbeat) (*FanOutEntityOutput, error) {
	var pageHasMore = false
	var lastOffset = offset
	err := s.repo.WithTX(ctx, func(ctx context.Context) error {
		return s.repo.ForEachPage(ctx, offset, func(ctx context.Context, entities []*Entity, hasMore bool) error {
			pageHasMore = hasMore

			for _, e := range entities {
				if e.ID > lastOffset {
					lastOffset = e.ID
				}

				if err := s.producer.Produce(&kafka.Message{
					Key:            []byte(e.ExternalID.String()),
					TopicPartition: kafka.TopicPartition{Topic: &_entityTopic},
				}, nil); err != nil {
					return err
				}
			}
			// Simulate some more processing, like sending to a message queue
			time.Sleep(rand.N[time.Duration](100) * time.Millisecond * 10)
			return hb(ctx, lastOffset)
		})
	})
	if err != nil {
		return nil, err
	}
	// This is a placeholder for the actual implementation
	return &FanOutEntityOutput{
		Offset:  lastOffset,
		HasMore: pageHasMore,
	}, nil
}
