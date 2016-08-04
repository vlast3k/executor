// This file was generated by counterfeiter
package faketransformer

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-incubator/garden"
	"github.com/tedsuo/ifrit"
)

type FakeTransformer struct {
	StepForStub        func(log_streamer.LogStreamer, *models.Action, garden.Container, string, []executor.PortMapping, lager.Logger) steps.Step
	stepForMutex       sync.RWMutex
	stepForArgsForCall []struct {
		arg1 log_streamer.LogStreamer
		arg2 *models.Action
		arg3 garden.Container
		arg4 string
		arg5 []executor.PortMapping
		arg6 lager.Logger
	}
	stepForReturns struct {
		result1 steps.Step
	}
	StepsRunnerStub        func(lager.Logger, executor.Container, garden.Container, log_streamer.LogStreamer) (ifrit.Runner, error)
	stepsRunnerMutex       sync.RWMutex
	stepsRunnerArgsForCall []struct {
		arg1 lager.Logger
		arg2 executor.Container
		arg3 garden.Container
		arg4 log_streamer.LogStreamer
	}
	stepsRunnerReturns struct {
		result1 ifrit.Runner
		result2 error
	}
}

func (fake *FakeTransformer) StepFor(arg1 log_streamer.LogStreamer, arg2 *models.Action, arg3 garden.Container, arg4 string, arg5 []executor.PortMapping, arg6 lager.Logger) steps.Step {
	fake.stepForMutex.Lock()
	fake.stepForArgsForCall = append(fake.stepForArgsForCall, struct {
		arg1 log_streamer.LogStreamer
		arg2 *models.Action
		arg3 garden.Container
		arg4 string
		arg5 []executor.PortMapping
		arg6 lager.Logger
	}{arg1, arg2, arg3, arg4, arg5, arg6})
	fake.stepForMutex.Unlock()
	if fake.StepForStub != nil {
		return fake.StepForStub(arg1, arg2, arg3, arg4, arg5, arg6)
	} else {
		return fake.stepForReturns.result1
	}
}

func (fake *FakeTransformer) StepForCallCount() int {
	fake.stepForMutex.RLock()
	defer fake.stepForMutex.RUnlock()
	return len(fake.stepForArgsForCall)
}

func (fake *FakeTransformer) StepForArgsForCall(i int) (log_streamer.LogStreamer, *models.Action, garden.Container, string, []executor.PortMapping, lager.Logger) {
	fake.stepForMutex.RLock()
	defer fake.stepForMutex.RUnlock()
	return fake.stepForArgsForCall[i].arg1, fake.stepForArgsForCall[i].arg2, fake.stepForArgsForCall[i].arg3, fake.stepForArgsForCall[i].arg4, fake.stepForArgsForCall[i].arg5, fake.stepForArgsForCall[i].arg6
}

func (fake *FakeTransformer) StepForReturns(result1 steps.Step) {
	fake.StepForStub = nil
	fake.stepForReturns = struct {
		result1 steps.Step
	}{result1}
}

func (fake *FakeTransformer) StepsRunner(arg1 lager.Logger, arg2 executor.Container, arg3 garden.Container, arg4 log_streamer.LogStreamer) (ifrit.Runner, error) {
	fake.stepsRunnerMutex.Lock()
	fake.stepsRunnerArgsForCall = append(fake.stepsRunnerArgsForCall, struct {
		arg1 lager.Logger
		arg2 executor.Container
		arg3 garden.Container
		arg4 log_streamer.LogStreamer
	}{arg1, arg2, arg3, arg4})
	fake.stepsRunnerMutex.Unlock()
	if fake.StepsRunnerStub != nil {
		return fake.StepsRunnerStub(arg1, arg2, arg3, arg4)
	} else {
		return fake.stepsRunnerReturns.result1, fake.stepsRunnerReturns.result2
	}
}

func (fake *FakeTransformer) StepsRunnerCallCount() int {
	fake.stepsRunnerMutex.RLock()
	defer fake.stepsRunnerMutex.RUnlock()
	return len(fake.stepsRunnerArgsForCall)
}

func (fake *FakeTransformer) StepsRunnerArgsForCall(i int) (lager.Logger, executor.Container, garden.Container, log_streamer.LogStreamer) {
	fake.stepsRunnerMutex.RLock()
	defer fake.stepsRunnerMutex.RUnlock()
	return fake.stepsRunnerArgsForCall[i].arg1, fake.stepsRunnerArgsForCall[i].arg2, fake.stepsRunnerArgsForCall[i].arg3, fake.stepsRunnerArgsForCall[i].arg4
}

func (fake *FakeTransformer) StepsRunnerReturns(result1 ifrit.Runner, result2 error) {
	fake.StepsRunnerStub = nil
	fake.stepsRunnerReturns = struct {
		result1 ifrit.Runner
		result2 error
	}{result1, result2}
}

var _ transformer.Transformer = new(FakeTransformer)
