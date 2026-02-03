package cmd

import (
	"testing"
)

func TestSkillCommandStructure(t *testing.T) {
	// Verify skill command exists and has correct properties
	if skillCmd == nil {
		t.Fatal("skillCmd is nil")
	}

	if skillCmd.Use != "skill" {
		t.Errorf("expected Use to be 'skill', got %s", skillCmd.Use)
	}

	if skillCmd.Short == "" {
		t.Error("skillCmd.Short should not be empty")
	}
}

func TestSkillAddCommandStructure(t *testing.T) {
	// Verify skill add command exists and has correct properties
	if skillAddCmd == nil {
		t.Fatal("skillAddCmd is nil")
	}

	if skillAddCmd.Use != "add" {
		t.Errorf("expected Use to be 'add', got %s", skillAddCmd.Use)
	}

	if skillAddCmd.Short == "" {
		t.Error("skillAddCmd.Short should not be empty")
	}

	if skillAddCmd.RunE == nil {
		t.Error("skillAddCmd.RunE should not be nil")
	}
}

func TestSkillRemoveCommandStructure(t *testing.T) {
	// Verify skill remove command exists and has correct properties
	if skillRemoveCmd == nil {
		t.Fatal("skillRemoveCmd is nil")
	}

	if skillRemoveCmd.Use != "remove" {
		t.Errorf("expected Use to be 'remove', got %s", skillRemoveCmd.Use)
	}

	if skillRemoveCmd.Short == "" {
		t.Error("skillRemoveCmd.Short should not be empty")
	}

	if skillRemoveCmd.RunE == nil {
		t.Error("skillRemoveCmd.RunE should not be nil")
	}

	// Check that 'rm' is an alias
	hasAlias := false
	for _, alias := range skillRemoveCmd.Aliases {
		if alias == "rm" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Error("skillRemoveCmd should have 'rm' as an alias")
	}
}

func TestSkillSubcommands(t *testing.T) {
	// Verify both add and remove are registered as subcommands of skill
	subcommands := make(map[string]bool)
	for _, cmd := range skillCmd.Commands() {
		subcommands[cmd.Use] = true
	}

	if !subcommands["add"] {
		t.Error("'add' command should be a subcommand of 'skill'")
	}

	if !subcommands["remove"] {
		t.Error("'remove' command should be a subcommand of 'skill'")
	}
}
