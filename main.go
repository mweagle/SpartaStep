package main

import (
	"context"
	"math/rand"
	"os"
	"time"

	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	step "github.com/mweagle/Sparta/aws/step"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type lambdaRollResponse struct {
	Roll int `json:"roll"`
}

// Standard AWS λ function
func lambdaRollDie(ctx context.Context) (lambdaRollResponse, error) {
	return lambdaRollResponse{
		Roll: rand.Intn(5) + 1,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Main
func main() {
	lambdaFn := sparta.HandleAWSLambda("StepRollDie",
		lambdaRollDie,
		sparta.IAMRoleDefinition{})
	lambdaFn.Options.MemorySize = 128
	lambdaFn.Options.Tags = map[string]string{
		"myAccounting": "tag",
	}
	var lambdaFunctions []*sparta.LambdaAWSInfo
	lambdaFunctions = append(lambdaFunctions, lambdaFn)

	// Make all the Step states
	lambdaTaskState := step.NewLambdaTaskState("lambdaRollDie", lambdaFn)
	successState := step.NewSuccessState("success")
	delayState := step.NewWaitDelayState("tryAgainShortly", 3*time.Second)
	lambdaChoices := []step.ChoiceBranch{
		&step.Not{
			Comparison: &step.NumericGreaterThan{
				Variable: "$.roll",
				Value:    3,
			},
			Next: delayState,
		},
	}
	choiceState := step.NewChoiceState("checkRoll",
		lambdaChoices...).
		WithDefault(successState)

	// Hook up the transitions
	lambdaTaskState.Next(choiceState)
	delayState.Next(lambdaTaskState)

	// Startup the machine.
	stateMachineName := spartaCF.UserScopedStackName("StateMachine")
	startMachine := step.NewStateMachine(stateMachineName, lambdaTaskState)

	// Setup the hook to annotate
	workflowHooks := &sparta.WorkflowHooks{
		ServiceDecorators: []sparta.ServiceDecoratorHookHandler{
			startMachine.StateMachineDecorator(),
		},
	}

	userStackName := spartaCF.UserScopedStackName("SpartaStep")
	err := sparta.MainEx(userStackName,
		"Simple Sparta application that demonstrates AWS Step functions",
		lambdaFunctions,
		nil,
		nil,
		workflowHooks,
		false)
	if err != nil {
		os.Exit(1)
	}
}
