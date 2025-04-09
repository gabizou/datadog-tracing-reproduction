package fanout

import (
	"context"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
	"time"
)

const (
	TaskQueueObjectFanout = "TaskQueueObjectFanout"
	WorkflowObjectFanout  = "WorkflowObjectFanout"
	ActivityFanOutPage    = "ActivityFanOutPage"
)

type FanoutProgress struct {
	PageNumber uint64
	Offset     uint32
}

func ActivityFanOutPageFunc(svc Service) func(ctx context.Context, in FanOutEntityInput) (out FanOutEntityOutput, err error) {
	return func(ctx context.Context, in FanOutEntityInput) (out FanOutEntityOutput, err error) {
		out.PageNumber = in.PageNumber
		var currentProgress FanoutProgress
		if activity.HasHeartbeatDetails(ctx) {
			err = activity.GetHeartbeatDetails(ctx, &currentProgress)
			if err != nil {
				return out, err
			}
		}
		offset := in.Offset
		if currentProgress.Offset >= offset {
			offset = currentProgress.Offset
		}
		res, err := svc.FanOutEntities(ctx, offset, func(ctx context.Context, offset uint32) error {
			currentProgress.Offset = offset
			activity.RecordHeartbeat(ctx, currentProgress)
			return nil
		})
		if err != nil {
			return out, err
		}
		out.HasMore = res.HasMore
		out.Offset = res.Offset
		return out, nil
	}
}

func ParentFanoutWorkflow(ctx workflow.Context) error {
	// This is a placeholder for the actual implementation
	aCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           TaskQueueObjectFanout,
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    time.Minute,
	})
	var page *FanOutEntityOutput
	err := workflow.ExecuteActivity(aCtx, ActivityFanOutPage).
		Get(aCtx, &page)
	if err != nil {
		return err
	}
	for page.HasMore {
		input := FanOutEntityInput{
			Offset:     page.Offset,
			PageNumber: page.PageNumber + 1,
		}
		err = workflow.ExecuteActivity(aCtx, ActivityFanOutPage, input).
			Get(aCtx, &page)
		if err != nil {
			return err
		}
	}

	return nil
}
