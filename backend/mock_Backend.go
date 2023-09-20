// Code generated by mockery v2.20.0. DO NOT EDIT.

package backend

import (
	context "context"

	contextpropagation "github.com/cschleiden/go-workflows/internal/contextpropagation"
	converter "github.com/cschleiden/go-workflows/converter"

	core "github.com/cschleiden/go-workflows/internal/core"

	history "github.com/cschleiden/go-workflows/internal/history"

	metrics "github.com/cschleiden/go-workflows/metrics"

	mock "github.com/stretchr/testify/mock"

	slog "log/slog"

	task "github.com/cschleiden/go-workflows/backend/task"

	trace "go.opentelemetry.io/otel/trace"
)

// MockBackend is an autogenerated mock type for the Backend type
type MockBackend struct {
	mock.Mock
}

// CancelWorkflowInstance provides a mock function with given fields: ctx, instance, cancelEvent
func (_m *MockBackend) CancelWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, cancelEvent *history.Event) error {
	ret := _m.Called(ctx, instance, cancelEvent)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance, *history.Event) error); ok {
		r0 = rf(ctx, instance, cancelEvent)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CompleteActivityTask provides a mock function with given fields: ctx, instance, activityID, event
func (_m *MockBackend) CompleteActivityTask(ctx context.Context, instance *core.WorkflowInstance, activityID string, event *history.Event) error {
	ret := _m.Called(ctx, instance, activityID, event)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance, string, *history.Event) error); ok {
		r0 = rf(ctx, instance, activityID, event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CompleteWorkflowTask provides a mock function with given fields: ctx, _a1, instance, state, executedEvents, activityEvents, timerEvents, workflowEvents
func (_m *MockBackend) CompleteWorkflowTask(ctx context.Context, _a1 *task.Workflow, instance *core.WorkflowInstance, state core.WorkflowInstanceState, executedEvents []*history.Event, activityEvents []*history.Event, timerEvents []*history.Event, workflowEvents []history.WorkflowEvent) error {
	ret := _m.Called(ctx, _a1, instance, state, executedEvents, activityEvents, timerEvents, workflowEvents)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *task.Workflow, *core.WorkflowInstance, core.WorkflowInstanceState, []*history.Event, []*history.Event, []*history.Event, []history.WorkflowEvent) error); ok {
		r0 = rf(ctx, _a1, instance, state, executedEvents, activityEvents, timerEvents, workflowEvents)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ContextPropagators provides a mock function with given fields:
func (_m *MockBackend) ContextPropagators() []contextpropagation.ContextPropagator {
	ret := _m.Called()

	var r0 []contextpropagation.ContextPropagator
	if rf, ok := ret.Get(0).(func() []contextpropagation.ContextPropagator); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]contextpropagation.ContextPropagator)
		}
	}

	return r0
}

// Converter provides a mock function with given fields:
func (_m *MockBackend) Converter() converter.Converter {
	ret := _m.Called()

	var r0 converter.Converter
	if rf, ok := ret.Get(0).(func() converter.Converter); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(converter.Converter)
		}
	}

	return r0
}

// CreateWorkflowInstance provides a mock function with given fields: ctx, instance, event
func (_m *MockBackend) CreateWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance, event *history.Event) error {
	ret := _m.Called(ctx, instance, event)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance, *history.Event) error); ok {
		r0 = rf(ctx, instance, event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExtendActivityTask provides a mock function with given fields: ctx, activityID
func (_m *MockBackend) ExtendActivityTask(ctx context.Context, activityID string) error {
	ret := _m.Called(ctx, activityID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, activityID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExtendWorkflowTask provides a mock function with given fields: ctx, taskID, instance
func (_m *MockBackend) ExtendWorkflowTask(ctx context.Context, taskID string, instance *core.WorkflowInstance) error {
	ret := _m.Called(ctx, taskID, instance)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *core.WorkflowInstance) error); ok {
		r0 = rf(ctx, taskID, instance)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetActivityTask provides a mock function with given fields: ctx
func (_m *MockBackend) GetActivityTask(ctx context.Context) (*task.Activity, error) {
	ret := _m.Called(ctx)

	var r0 *task.Activity
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*task.Activity, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *task.Activity); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*task.Activity)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStats provides a mock function with given fields: ctx
func (_m *MockBackend) GetStats(ctx context.Context) (*Stats, error) {
	ret := _m.Called(ctx)

	var r0 *Stats
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*Stats, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *Stats); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Stats)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflowInstanceHistory provides a mock function with given fields: ctx, instance, lastSequenceID
func (_m *MockBackend) GetWorkflowInstanceHistory(ctx context.Context, instance *core.WorkflowInstance, lastSequenceID *int64) ([]*history.Event, error) {
	ret := _m.Called(ctx, instance, lastSequenceID)

	var r0 []*history.Event
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance, *int64) ([]*history.Event, error)); ok {
		return rf(ctx, instance, lastSequenceID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance, *int64) []*history.Event); ok {
		r0 = rf(ctx, instance, lastSequenceID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*history.Event)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *core.WorkflowInstance, *int64) error); ok {
		r1 = rf(ctx, instance, lastSequenceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflowInstanceState provides a mock function with given fields: ctx, instance
func (_m *MockBackend) GetWorkflowInstanceState(ctx context.Context, instance *core.WorkflowInstance) (core.WorkflowInstanceState, error) {
	ret := _m.Called(ctx, instance)

	var r0 core.WorkflowInstanceState
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance) (core.WorkflowInstanceState, error)); ok {
		return rf(ctx, instance)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance) core.WorkflowInstanceState); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Get(0).(core.WorkflowInstanceState)
	}

	if rf, ok := ret.Get(1).(func(context.Context, *core.WorkflowInstance) error); ok {
		r1 = rf(ctx, instance)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkflowTask provides a mock function with given fields: ctx
func (_m *MockBackend) GetWorkflowTask(ctx context.Context) (*task.Workflow, error) {
	ret := _m.Called(ctx)

	var r0 *task.Workflow
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*task.Workflow, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *task.Workflow); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*task.Workflow)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Logger provides a mock function with given fields:
func (_m *MockBackend) Logger() *slog.Logger {
	ret := _m.Called()

	var r0 *slog.Logger
	if rf, ok := ret.Get(0).(func() *slog.Logger); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*slog.Logger)
		}
	}

	return r0
}

// Metrics provides a mock function with given fields:
func (_m *MockBackend) Metrics() metrics.Client {
	ret := _m.Called()

	var r0 metrics.Client
	if rf, ok := ret.Get(0).(func() metrics.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metrics.Client)
		}
	}

	return r0
}

// RemoveWorkflowInstance provides a mock function with given fields: ctx, instance
func (_m *MockBackend) RemoveWorkflowInstance(ctx context.Context, instance *core.WorkflowInstance) error {
	ret := _m.Called(ctx, instance)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *core.WorkflowInstance) error); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SignalWorkflow provides a mock function with given fields: ctx, instanceID, event
func (_m *MockBackend) SignalWorkflow(ctx context.Context, instanceID string, event *history.Event) error {
	ret := _m.Called(ctx, instanceID, event)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *history.Event) error); ok {
		r0 = rf(ctx, instanceID, event)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Tracer provides a mock function with given fields:
func (_m *MockBackend) Tracer() trace.Tracer {
	ret := _m.Called()

	var r0 trace.Tracer
	if rf, ok := ret.Get(0).(func() trace.Tracer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(trace.Tracer)
		}
	}

	return r0
}

type mockConstructorTestingTNewMockBackend interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockBackend creates a new instance of MockBackend. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockBackend(t mockConstructorTestingTNewMockBackend) *MockBackend {
	mock := &MockBackend{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
