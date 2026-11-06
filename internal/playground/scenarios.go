package playground

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Scenario runs an actual CLI command against synthetic API responses.
type Scenario struct {
	ID           string     `json:"id"`
	Group        string     `json:"group"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Args         []string   `json:"args"`
	Source       string     `json:"source"`
	ExpectedExit int        `json:"expectedExit"`
	Responses    []Response `json:"responses"`
}

// Response supplies a JSON response for an exact API URL path.
type Response struct {
	Path   string          `json:"path"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

//go:embed scenarios.json
var scenarioData []byte

// LoadScenarios loads the built-in synthetic examples.
func LoadScenarios() ([]Scenario, error) {
	return decodeScenarios(scenarioData)
}

// LoadScenariosFile reloads editable examples without rebuilding the playground.
func LoadScenariosFile(path string) ([]Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenarios: %w", err)
	}
	return decodeScenarios(data)
}

func decodeScenarios(data []byte) ([]Scenario, error) {
	var scenarios []Scenario
	if err := json.Unmarshal(data, &scenarios); err != nil {
		return nil, fmt.Errorf("decode scenarios: %w", err)
	}
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("scenarios must contain at least one example")
	}
	ids := make(map[string]bool, len(scenarios))
	for _, scenario := range scenarios {
		if scenario.ID == "" || ids[scenario.ID] {
			return nil, fmt.Errorf("scenario IDs must be nonempty and unique: %q", scenario.ID)
		}
		ids[scenario.ID] = true
		if scenario.ExpectedExit < 0 || scenario.ExpectedExit > 255 {
			return nil, fmt.Errorf("scenario %q has an invalid expected exit code", scenario.ID)
		}
		paths := make(map[string]bool, len(scenario.Responses))
		for _, response := range scenario.Responses {
			if !strings.HasPrefix(response.Path, "/") || strings.ContainsAny(response.Path, "?#") || paths[response.Path] {
				return nil, fmt.Errorf("scenario %q needs unique response URL paths: %q", scenario.ID, response.Path)
			}
			paths[response.Path] = true
			if response.Status < 200 || response.Status > 599 || !json.Valid(response.Body) {
				return nil, fmt.Errorf("scenario %q has an invalid response for %q", scenario.ID, response.Path)
			}
		}
	}
	return scenarios, nil
}
