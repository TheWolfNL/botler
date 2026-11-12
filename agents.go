package main

import (
	"fmt"

	"github.com/TheWolfNL/botler/internal/agent"
	"github.com/TheWolfNL/botler/internal/agent/claude"
)

// agents lists the AI tools Botler can hook into; add an adapter here to support another tool.
var agents = []agent.Agent{claude.Agent{}}

// defaultAgent handles `botler hook` without an agent ID.
const defaultAgent = "claude"

func findAgent(id string) (agent.Agent, error) {
	for _, a := range agents {
		if a.ID() == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("unknown agent %q", id)
}

// selectAgents returns the agent named by ids, or every agent when ids is empty.
func selectAgents(ids []string) ([]agent.Agent, error) {
	if len(ids) == 0 {
		return agents, nil
	}
	out := make([]agent.Agent, 0, len(ids))
	for _, id := range ids {
		a, err := findAgent(id)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// requestAgentURL opens a pre-filled GitHub issue asking for another AI tool.
const requestAgentURL = "https://github.com/TheWolfNL/botler/issues/new?labels=agent-request&title=Support+for+%3Ctool%3E&body=Which+AI+tool+would+you+like+Botler+to+support%3F%0A%0ALink+to+its+hooks+or+notification+docs%3A%0A%0AWhich+events+does+it+expose+%28done%2C+failed%2C+needs+input%29%3F"
