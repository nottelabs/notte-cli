package cmd

import (
	"testing"
)

func TestSkillCommandStructure(t *testing.T) {
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

func TestSkillAddUpgradeFlag(t *testing.T) {
	upgrade := skillAddCmd.Flags().Lookup("upgrade")
	if upgrade == nil {
		t.Fatal("skillAddCmd should have an --upgrade flag")
	}
	if upgrade.Shorthand != "f" {
		t.Errorf("expected --upgrade shorthand to be 'f', got %q", upgrade.Shorthand)
	}
	if upgrade.Value.Type() != "bool" {
		t.Errorf("expected --upgrade to be a bool flag, got %s", upgrade.Value.Type())
	}
}

func TestSkillRemoveCommandStructure(t *testing.T) {
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

func TestSkillSourcePointsToSkillsRepo(t *testing.T) {
	// Regression guard: the npx skills tool clones whatever repo this points
	// at and searches it for SKILL.md files. The skill content lives in
	// nottelabs/notte-skills; pointing at nottelabs/notte-cli (the CLI repo)
	// would find only the empty submodule directory and report "No skills
	// found".
	if skillSource != "nottelabs/notte-skills" {
		t.Errorf("skillSource should be 'nottelabs/notte-skills', got %q", skillSource)
	}
	if skillName != "notte-browser" {
		t.Errorf("skillName should be 'notte-browser', got %q", skillName)
	}
}
