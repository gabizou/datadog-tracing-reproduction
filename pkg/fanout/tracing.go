package fanout

import (
	"context"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func NewTracer() func(service Service) Service {
	return func(service Service) Service {
		return tracerSvc{
			svc: service,
		}
	}
}

type tracerSvc struct {
	svc Service
}

func (t tracerSvc) FanOutEntities(ctx context.Context, offset uint32, hb Heartbeat) (out *FanOutEntityOutput, err error) {
	// Start a new span
	span := tracer.StartSpan("FanOutEntities",
		tracer.Tag("fanout.offset", offset),
	)
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	return t.svc.FanOutEntities(ctx, offset, hb)
}

func NewTracingRepository() func(Repository) Repository {
	return func(repo Repository) Repository {
		return &tracerRepo{repo: repo}
	}
}

type tracerRepo struct {
	repo Repository
}

func (t *tracerRepo) WithTX(ctx context.Context, cb func(ctx2 context.Context) error) error {
	return t.repo.WithTX(ctx, cb)
}

func (t *tracerRepo) ForEachPage(
	ctx context.Context,
	offset uint32,
	cb func(ctx context.Context, entities []*Entity, hasMore bool) error,
) error {
	callback := func(ctx context.Context, entities []*Entity, hasMore bool) (err error) {
		span, ctx := tracer.StartSpanFromContext(ctx, "fanout.ForEachPage",
			tracer.Tag("fanout.count", len(entities)),
		)
		defer func() {
			if len(entities) > 0 {
				span.SetTag("fanout.page.first", entities[0].ID)
				span.SetTag("fanout.page.last", entities[len(entities)-1].ID)
				span.SetTag("fanout.page.has_more", hasMore)
			}
			span.Finish(tracer.WithError(err))
		}()
		return cb(ctx, entities, hasMore)
	}
	return t.repo.ForEachPage(ctx, offset, callback)
}
