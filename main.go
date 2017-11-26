package main

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	step "github.com/mweagle/Sparta/aws/step"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// Standard AWS λ function
func lambdaRollDie(w http.ResponseWriter, r *http.Request) {
	logger, loggerOK := r.Context().Value(sparta.ContextKeyLogger).(*logrus.Logger)
	if !loggerOK {
		http.Error(w,
			"Failed to access *logger",
			http.StatusInternalServerError)
		return
	}

	allData, allDataErr := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	if allDataErr != nil {
		http.Error(w, allDataErr.Error(), http.StatusInternalServerError)
		return
	}
	// Log some information
	logger.WithFields(logrus.Fields{
		"EventBody": string(allData),
	}).Info("Event")

	// Return a randomized value in the range [0, 6]
	rollBytes, rollBytesErr := json.Marshal(&struct {
		Roll int `json:"roll"`
	}{
		Roll: rand.Intn(5) + 1,
	})
	if rollBytesErr != nil {
		http.Error(w, rollBytesErr.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write(rollBytes)
}

////////////////////////////////////////////////////////////////////////////////
// Main
func main() {
	lambdaFn := sparta.HandleAWSLambda("StepRollDie",
		http.HandlerFunc(lambdaRollDie),
		sparta.IAMRoleDefinition{})
	lambdaFn.Options.MemorySize = 128
	lambdaFn.Options.Tags = map[string]string{
		"myAccounting": "tag",
	}
	var lambdaFunctions []*sparta.LambdaAWSInfo
	lambdaFunctions = append(lambdaFunctions, lambdaFn)

	// Make all the Step states
	lambdaTaskState := step.NewTaskState("lambdaRollDie", lambdaFn)
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
		ServiceDecorator: startMachine.StateMachineDecorator(),
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
