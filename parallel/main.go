package main

import (
	"context"
	"os"

	sparta "github.com/mweagle/Sparta"
	spartaStep "github.com/mweagle/Sparta/aws/step"

	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
)

func createDataLambda(ctx context.Context,
	props map[string]interface{}) (map[string]interface{}, error) {

	return map[string]interface{}{
		"ship-date": "2016-03-14T01:59:00Z",
		"detail": map[string]interface{}{
			"delivery-partner": "UQS",
			"shipped": []map[string]interface{}{
				{
					"prod":      "R31",
					"dest-code": 9511,
					"quantity":  1344,
				},
				{
					"prod":      "S39",
					"dest-code": 9511,
					"quantity":  40,
				},
				{
					"prod":      "R31",
					"dest-code": 9833,
					"quantity":  12,
				},
				{
					"prod":      "R40",
					"dest-code": 9860,
					"quantity":  887,
				},
				{
					"prod":      "R40",
					"dest-code": 9511,
					"quantity":  1220,
				},
			},
		},
	}, nil
}

// Standard AWS λ function
func mapCallback(ctx context.Context,
	props map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"Message": "Hello",
		"Event":   props,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Main
func main() {
	lambdaMapFn, _ := sparta.NewAWSLambda("mapLambdaCallback",
		mapCallback,
		sparta.IAMRoleDefinition{})

	// Make all the Step states
	lambdaMapTaskState := spartaStep.NewLambdaTaskState("lambdaMapData", lambdaMapFn)
	mapMachine := spartaStep.NewStateMachine("mapStateName", lambdaMapTaskState)
	mapState := spartaStep.NewMapState("mapResults", mapMachine)
	mapState.ItemsPath = "$.shipped"
	node := mapState.WithInputPath("$.detail")

	// Then create the producer machine
	lambdaProducerFn, _ := sparta.NewAWSLambda("produceData",
		createDataLambda,
		sparta.IAMRoleDefinition{})
	lambdaProducerTaskState := spartaStep.NewLambdaTaskState("lambdaProduceData",
		lambdaProducerFn)

	// Hook up the transitions
	stateMachineName := spartaCF.UserScopedStackName("TestMapStateMachine")
	lambdaProducerTaskState.Next(node)
	stateMachine := spartaStep.NewStateMachine(stateMachineName,
		lambdaProducerTaskState)

	// Startup the machine.
	// Setup the hook to annotate
	workflowHooks := &sparta.WorkflowHooks{
		ServiceDecorators: []sparta.ServiceDecoratorHookHandler{
			stateMachine.StateMachineDecorator(),
		},
	}

	userStackName := spartaCF.UserScopedStackName("SpartaStep")
	err := sparta.MainEx(userStackName,
		"Simple Sparta application that demonstrates AWS Step functions",
		[]*sparta.LambdaAWSInfo{lambdaMapFn, lambdaProducerFn},
		nil,
		nil,
		workflowHooks,
		false)
	if err != nil {
		os.Exit(1)
	}
}
