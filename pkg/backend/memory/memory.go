package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cschleiden/go-dt/internal/command"
	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/core"
	"github.com/cschleiden/go-dt/pkg/core/tasks"
	"github.com/cschleiden/go-dt/pkg/history"
	"github.com/google/uuid"
)

// TODO: This will have to move somewhere else
type workflowState struct {
	Name string

	Created time.Time
}

// Simple in-memory backend for development
type memoryBackend struct {
	instanceStore map[string]map[string]workflowState

	mu sync.Mutex

	// workflows not yet picked up
	workflows chan *tasks.WorkflowTask

	lockedWorkflows map[string]*tasks.WorkflowTask

	activities chan *tasks.ActivityTask

	lockedActivities map[string]*tasks.ActivityTask
}

func NewMemoryBackend() backend.Backend {
	return &memoryBackend{
		instanceStore: make(map[string]map[string]workflowState),
		mu:            sync.Mutex{},

		// Queue of unlocked workflow instances
		workflows:       make(chan *tasks.WorkflowTask, 100),
		lockedWorkflows: make(map[string]*tasks.WorkflowTask),

		activities:       make(chan *tasks.ActivityTask, 100),
		lockedActivities: make(map[string]*tasks.ActivityTask),
	}
}

func (mb *memoryBackend) CreateWorkflowInstance(ctx context.Context, m core.TaskMessage) error {
	attrs, ok := m.HistoryEvent.Attributes.(history.ExecutionStartedAttributes)
	if !ok {
		return errors.New("invalid workflow instance creation event")
	}

	mb.mu.Lock()
	defer mb.mu.Unlock()

	x, ok := mb.instanceStore[m.WorkflowInstance.GetInstanceID()]
	if !ok {
		x = make(map[string]workflowState)
		mb.instanceStore[m.WorkflowInstance.GetInstanceID()] = x
	}

	// TODO: Check for existing workflow instances
	newState := workflowState{
		Name: attrs.Name,
	}

	x[m.WorkflowInstance.GetExecutionID()] = newState

	// Add to queue
	// TODO: Check if this already exists
	mb.workflows <- &tasks.WorkflowTask{
		WorkflowInstance: m.WorkflowInstance,
		History:          []history.HistoryEvent{m.HistoryEvent},
	}

	return nil
}

func (mb *memoryBackend) GetWorkflowTask(ctx context.Context) (*tasks.WorkflowTask, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.workflows:
		mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()] = t
		return t, nil
	}
}

func (mb *memoryBackend) CompleteWorkflowTask(_ context.Context, t tasks.WorkflowTask, commands []command.Command) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	_, ok := mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()]
	if !ok {
		panic("could not unlock workflow instance")
	}

	// Check if completed

	// else: Schedule commands
	scheduledActivity := false

	for _, c := range commands {
		switch c.Type {
		case command.CommandType_ScheduleActivityTask:
			a := c.Attr.(command.ScheduleActivityTaskCommandAttr)
			mb.activities <- &tasks.ActivityTask{
				WorkflowInstance: t.WorkflowInstance,
				ID:               uuid.NewString(),
				Event: history.NewHistoryEvent(
					history.HistoryEventType_ActivityScheduled,
					int64(c.ID),
					history.ActivityScheduledAttributes{
						Name:    a.Name,
						Version: a.Version,
						Inputs:  [][]byte{},
					},
				),
			}

			scheduledActivity = true

		default:
			// panic("unsupported command")
		}
	}

	// Return to queue
	if !scheduledActivity {
		// Unlock workflow instance
		delete(mb.lockedWorkflows, t.WorkflowInstance.GetExecutionID())
		mb.workflows <- &t
	}

	return nil
}

func (mb *memoryBackend) GetActivityTask(ctx context.Context) (*tasks.ActivityTask, error) {
	select {
	case <-ctx.Done():
		return nil, nil

	case t := <-mb.activities:
		mb.lockedActivities[t.ID] = t
		return t, nil
	}
}

func (mb *memoryBackend) CompleteActivityTask(_ context.Context, t tasks.ActivityTask, event history.HistoryEvent) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	delete(mb.lockedActivities, t.ID)

	// Continue workflow
	wt := mb.lockedWorkflows[t.WorkflowInstance.GetExecutionID()]

	wt.History = append(wt.History, event)

	delete(mb.lockedWorkflows, t.WorkflowInstance.GetExecutionID())
	mb.workflows <- wt

	return nil
}
