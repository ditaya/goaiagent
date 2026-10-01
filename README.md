# goaiagent

A tiny command-line AI agent in Go, built on the [Claude API](https://docs.claude.com/) and
the official [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) tool runner.

It's intentionally small: one file, three tools, no framework. The point is to show the
actual mechanics of an agentic loop — the model deciding when it needs a tool, the tool
running locally, and the result flowing back to the model — without any boilerplate in
the way.

## How it works

```
 you  ──────▶  Claude (claude-opus-5)
              │
              │  "I need the calculator tool for this"
              ▼
        tool runner executes calculator(...)  locally, in this process
              │
              ▼
        result fed back to Claude
              │
              ▼
        final answer  ──────▶  you
```

The agent keeps a running conversation (`history`) across turns, and for each turn hands
it to `client.Beta.Messages.NewToolRunner(...)`, which drives the request → tool-call →
tool-result loop automatically (up to `MaxIterations` rounds) and returns Claude's final
response.

## Tools

| Tool | Does |
|---|---|
| `calculator` | add / subtract / multiply / divide two numbers |
| `convert_temperature` | convert between celsius, fahrenheit, kelvin |
| `current_time` | current date/time, optionally in an IANA timezone |

Each tool is a plain Go function; its JSON schema is generated automatically from struct
tags (see `CalculatorInput`, `TemperatureInput`, `CurrentTimeInput` in `main.go`) — no
schema is written by hand.

## Setup

Requires Go 1.21+ and an Anthropic API key.

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go run .
```

## Example session

```
goaiagent - a tiny Claude-powered CLI agent
Tools: calculator, convert_temperature, current_time
Type a question, or 'exit' to quit.

> what's 212 fahrenheit in celsius, and what time is it in Tokyo?
212°F is 100°C, and the current time in Tokyo is ... JST.

> exit
```

## Flags

```
go run . -model claude-opus-5
```

`-model` overrides the Claude model ID (defaults to `claude-opus-5`).

## Extending it

To add a tool: define an input struct with `jsonschema` tags, write a
`func(ctx, input) (anthropic.BetaToolResultBlockParamContentUnion, error)`, wrap it with
`toolrunner.NewBetaToolFromJSONSchema(name, description, fn)`, and add it to the `tools`
slice in `main()`. Returning a non-nil `error` automatically surfaces as a failed tool
call to Claude, so it can retry or explain the failure.

## License

MIT — see [LICENSE](LICENSE).
