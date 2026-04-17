package handler

import (
	"context"
	"time"

	"github.com/puri-cp/puri/services/submission/executor"

	submissionv1 "github.com/puri-cp/puri/gen/submission/v1"
)

type examplePair struct {
	Input  string
	Output string
}

type exampleRunOutcome struct {
	Passed    bool
	Overall   string
	TestCases []*submissionv1.TestCaseResult
	Internal  error
}

func runAgainstExamples(
	ctx context.Context,
	runner *executor.DockerRunner,
	lang, source string,
	examples []examplePair,
	timeLimit, memoryLimit string,
) exampleRunOutcome {
	out := exampleRunOutcome{Passed: true, Overall: "ACCEPTED"}

	for i, ex := range examples {
		runCtx, cancel := context.WithTimeout(ctx, parseDuration(timeLimit)+10*time.Second)
		res := runner.Run(runCtx, lang, source, ex.Input, timeLimit, memoryLimit)
		cancel()

		if res.Error != nil && res.Result == "INTERNAL_ERROR" {
			out.Internal = res.Error
			return out
		}

		passed := normalizeOutput(res.Output) == normalizeOutput(ex.Output)
		result := res.Result
		if !passed && result == "ACCEPTED" {
			result = "WRONG_ANSWER"
		}
		if !passed {
			out.Passed = false
			if out.Overall == "ACCEPTED" {
				out.Overall = result
			}
		}

		out.TestCases = append(out.TestCases, &submissionv1.TestCaseResult{
			Index:           int32(i + 1),
			Passed:          passed,
			ActualOutput:    res.Output,
			ExpectedOutput:  ex.Output,
			Result:          result,
			ExecutionTimeMs: res.ExecutionTimeMs,
		})
	}

	return out
}
