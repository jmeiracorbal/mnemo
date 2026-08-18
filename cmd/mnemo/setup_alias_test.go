package main

import (
	"testing"

	"github.com/jmeiracorbal/mnemo/internal/agentinit"
)

func TestIsSetupAgentAlias(t *testing.T) {
	if !isSetupAgentAlias("cursor") {
		t.Fatal("cursor should be a setup alias")
	}
	if !isSetupAgentAlias("all") {
		t.Fatal("all should be a setup alias")
	}
	if isSetupAgentAlias("status") {
		t.Fatal("status should not be a setup alias")
	}
	for _, agent := range agentinit.SupportedAgents {
		if !isSetupAgentAlias(agent) {
			t.Fatalf("%q should be a setup alias", agent)
		}
	}
}
