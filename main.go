// Command goaiagent is a small REPL-based AI agent built on the Claude API.
// It demonstrates the agentic tool-use loop: the model decides when to call
// a tool, the SDK's tool runner executes it, and the result is fed back to
// the model until it produces a final answer.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/toolrunner"
)

const systemPrompt = `You are a concise command-line assistant with access to a calculator,
a temperature converter, and the current time. Use the tools whenever a
question depends on exact arithmetic, unit conversion, or the current time
instead of estimating. Keep answers short.`

// CalculatorInput is the schema for the "calculator" tool, generated
// automatically from these struct tags by the SDK's tool runner.
type CalculatorInput struct {
	Operation string  `json:"operation" jsonschema:"required,description=One of: add, subtract, multiply, divide"`
	A         float64 `json:"a" jsonschema:"required,description=The first operand"`
	B         float64 `json:"b" jsonschema:"required,description=The second operand"`
}

// TemperatureInput is the schema for the "convert_temperature" tool.
type TemperatureInput struct {
	Value float64 `json:"value" jsonschema:"required,description=The temperature value to convert"`
	From  string  `json:"from" jsonschema:"required,description=Source unit: celsius, fahrenheit, or kelvin"`
	To    string  `json:"to" jsonschema:"required,description=Target unit: celsius, fahrenheit, or kelvin"`
}

// CurrentTimeInput is the schema for the "current_time" tool.
type CurrentTimeInput struct {
	Timezone string `json:"timezone" jsonschema:"description=IANA timezone name, e.g. America/New_York. Leave empty for UTC."`
}

func textResult(s string) anthropic.BetaToolResultBlockParamContentUnion {
	return anthropic.BetaToolResultBlockParamContentUnion{
		OfText: &anthropic.BetaTextBlockParam{Text: s},
	}
}

func calculatorTool() anthropic.BetaTool {
	tool, err := toolrunner.NewBetaToolFromJSONSchema(
		"calculator",
		"Perform a single arithmetic operation (add, subtract, multiply, divide) on two numbers.",
		func(_ context.Context, in CalculatorInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			switch strings.ToLower(in.Operation) {
			case "add":
				return textResult(fmt.Sprintf("%g", in.A+in.B)), nil
			case "subtract":
				return textResult(fmt.Sprintf("%g", in.A-in.B)), nil
			case "multiply":
				return textResult(fmt.Sprintf("%g", in.A*in.B)), nil
			case "divide":
				if in.B == 0 {
					return anthropic.BetaToolResultBlockParamContentUnion{}, fmt.Errorf("division by zero")
				}
				return textResult(fmt.Sprintf("%g", in.A/in.B)), nil
			default:
				return anthropic.BetaToolResultBlockParamContentUnion{}, fmt.Errorf("unknown operation %q: must be add, subtract, multiply, or divide", in.Operation)
			}
		},
	)
	if err != nil {
		panic(err)
	}
	return tool
}

func temperatureTool() anthropic.BetaTool {
	tool, err := toolrunner.NewBetaToolFromJSONSchema(
		"convert_temperature",
		"Convert a temperature value between celsius, fahrenheit, and kelvin.",
		func(_ context.Context, in TemperatureInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			toCelsius := func(v float64, unit string) (float64, error) {
				switch strings.ToLower(unit) {
				case "celsius", "c":
					return v, nil
				case "fahrenheit", "f":
					return (v - 32) * 5 / 9, nil
				case "kelvin", "k":
					return v - 273.15, nil
				default:
					return 0, fmt.Errorf("unknown unit %q: must be celsius, fahrenheit, or kelvin", unit)
				}
			}
			fromCelsius := func(v float64, unit string) (float64, error) {
				switch strings.ToLower(unit) {
				case "celsius", "c":
					return v, nil
				case "fahrenheit", "f":
					return v*9/5 + 32, nil
				case "kelvin", "k":
					return v + 273.15, nil
				default:
					return 0, fmt.Errorf("unknown unit %q: must be celsius, fahrenheit, or kelvin", unit)
				}
			}
			c, err := toCelsius(in.Value, in.From)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{}, err
			}
			result, err := fromCelsius(c, in.To)
			if err != nil {
				return anthropic.BetaToolResultBlockParamContentUnion{}, err
			}
			return textResult(fmt.Sprintf("%g", result)), nil
		},
	)
	if err != nil {
		panic(err)
	}
	return tool
}

func currentTimeTool() anthropic.BetaTool {
	tool, err := toolrunner.NewBetaToolFromJSONSchema(
		"current_time",
		"Get the current date and time, optionally in a specific IANA timezone.",
		func(_ context.Context, in CurrentTimeInput) (anthropic.BetaToolResultBlockParamContentUnion, error) {
			loc := time.UTC
			if in.Timezone != "" {
				var err error
				loc, err = time.LoadLocation(in.Timezone)
				if err != nil {
					return anthropic.BetaToolResultBlockParamContentUnion{}, fmt.Errorf("unknown timezone %q", in.Timezone)
				}
			}
			return textResult(time.Now().In(loc).Format(time.RFC1123)), nil
		},
	)
	if err != nil {
		panic(err)
	}
	return tool
}

func main() {
	model := flag.String("model", "claude-opus-5", "Claude model ID to use")
	flag.Parse()

	client := anthropic.NewClient()
	tools := []anthropic.BetaTool{calculatorTool(), temperatureTool(), currentTimeTool()}

	fmt.Println("goaiagent - a tiny Claude-powered CLI agent")
	fmt.Println("Tools: calculator, convert_temperature, current_time")
	fmt.Println("Type a question, or 'exit' to quit.")

	var history []anthropic.BetaMessageParam
	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			break
		}

		history = append(history, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(input)))

		runner := client.Beta.Messages.NewToolRunner(tools, anthropic.BetaToolRunnerParams{
			BetaMessageNewParams: anthropic.BetaMessageNewParams{
				Model:     anthropic.Model(*model),
				MaxTokens: 4096,
				System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
				Messages:  history,
			},
			MaxIterations: 8,
		})

		message, err := runner.RunToCompletion(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			history = history[:len(history)-1]
			continue
		}

		history = append(history, message.ToParam())

		for _, block := range message.Content {
			if text, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
				fmt.Println(text.Text)
			}
		}
	}
}
