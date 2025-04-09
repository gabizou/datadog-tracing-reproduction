package main

import (
	"context"
	"github.com/gabizou/datadog-temporal-issue/pkg/fanout"
	"go.temporal.io/sdk/client"
	"log"
)

func main() {
	// Instrument Temporal client
	c, err := client.Dial(client.Options{HostPort: client.DefaultHostPort})
	if err != nil {
		log.Fatalf("failure to start workflow: %v", err)
	}
	defer c.Close()
	run, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		TaskQueue: fanout.TaskQueueObjectFanout,
	}, fanout.WorkflowObjectFanout)
	if err != nil {
		log.Fatalf("error scheduling workflow: %v", err)
	}
	err = run.Get(context.Background(), nil)
	if err != nil {
		log.Fatalf("failed to run workflow: %v", err)
	}

}
