package main

import (
	"database/sql"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gabizou/datadog-temporal-issue/pkg/fanout"
	"github.com/jackc/pgx/v5/stdlib"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/datadog/tracing"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	kafkatrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/confluentinc/confluent-kafka-go/kafka.v2"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorm.io/gorm.v1"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

const (
	_driverName = "pgx"
)

func main() {
	tracer.Start(
		tracer.WithService("fanout-worker"),
	)
	defer tracer.Stop()
	// Instrument Temporal client
	c, err := client.Dial(client.Options{
		HostPort:     client.DefaultHostPort,
		Interceptors: []interceptor.ClientInterceptor{tracing.NewTracingInterceptor(tracing.TracerOptions{})},
	})

	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	w := worker.New(c, fanout.TaskQueueObjectFanout, worker.Options{})
	w.RegisterWorkflowWithOptions(fanout.ParentFanoutWorkflow, workflow.RegisterOptions{
		Name: fanout.WorkflowObjectFanout,
	})
	d := stdlib.GetDefaultDriver()
	contains := false
	for _, driver := range sql.Drivers() {
		if driver == _driverName {
			contains = true
			break
		}
	}
	if !contains {
		sql.Register(_driverName, d)
	}

	// instrument the database driver and gorm
	dsn := "host=localhost user=go-app password=go-app dbname=go-app port=5433 sslmode=disable"
	open, err := sqltrace.Open(_driverName, dsn)
	if err != nil {
		log.Fatalf("Unable to open sql trace driver: %v", err)
	}
	db, err := gormtrace.Open(postgres.New(postgres.Config{Conn: open}), &gorm.Config{})
	if err != nil {
		log.Fatalf("Unable to open gorm driver: %v", err)
	}

	repo := fanout.NewRepository(db)
	repo = fanout.NewTracingRepository()(repo)

	// instrument the Kafka producer
	kafkaCfg := &kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
	}
	producer, err := kafkatrace.NewProducer(kafkaCfg)
	if err != nil {
		log.Fatalf("Unable to create Kafka producer: %v", err)
	}

	// Instrument the service layer
	svc := fanout.NewService(repo, producer)
	svc = fanout.NewTracer()(svc)

	w.RegisterActivityWithOptions(
		fanout.ActivityFanOutPageFunc(svc),
		activity.RegisterOptions{
			Name: fanout.ActivityFanOutPage,
		})

	// Now run the worker
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
