package discriminators

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionalAndRequiredDiscriminators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.go")
	source := `package api
 type Optional struct {
 Type *string
 }
 type Required struct {
 Type string
 }
 func (t *Union) FromOptional(v Optional) error {
 v.Type = "optional"
 return nil
 }
 func (t *Union) FromRequired(v Required) error {
 v.Type = "required"
 return nil
 }
 `
	// Generated declarations start at column zero.
	source = strings.ReplaceAll(source, "\n ", "\n")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() []byte {
		t.Helper()
		if output, err := exec.Command("python3", "../fix-type-discriminators.py", path).CombinedOutput(); err != nil {
			t.Fatalf("generator: %v\n%s", err, output)
		}
		output, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return output
	}
	first := string(run())
	if !strings.Contains(first, `tmp := "optional"; v.Type = &tmp`) {
		t.Fatal("optional discriminator was not repaired")
	}
	if !strings.Contains(first, `v.Type = "required"`) {
		t.Fatal("required discriminator was changed")
	}
	if second := string(run()); first != second {
		t.Fatal("repeated generation changed the output")
	}
}
