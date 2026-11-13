package tools

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/ElzaraA/ai-buddy-avito/internal/store"
)

// Спецификация specs/contracts/mcp-tools.json обязана совпадать с кодом:
// те же инструменты, права, признак подтверждения и обязательные аргументы.
func TestSpecMatchesCode(t *testing.T) {
	raw, err := os.ReadFile("../../specs/contracts/mcp-tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Tools []struct {
			Name                 string `json:"name"`
			Permission           string `json:"permission"`
			RequiresConfirmation bool   `json:"requiresConfirmation"`
			InputSchema          struct {
				Required   []string       `json:"required"`
				Properties map[string]any `json:"properties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}

	st := store.New()
	reg := NewRegistry()
	for _, tool := range []Tool{
		&AccessRequest{}, &AbsenceCreateRequest{S: st}, &DirectoryFindOwner{}, &ChecklistGet{S: st}, &AbsenceGetPolicy{},
	} {
		reg.Register(tool)
	}
	code := reg.List()
	if len(code) != len(doc.Tools) {
		t.Fatalf("spec has %d tools, code has %d", len(doc.Tools), len(code))
	}
	sort.Slice(doc.Tools, func(i, j int) bool { return doc.Tools[i].Name < doc.Tools[j].Name })
	for i, c := range code {
		s := doc.Tools[i]
		if s.Name != c.Name || s.Permission != c.Permission || s.RequiresConfirmation != c.RequiresConfirmation {
			t.Fatalf("mismatch: spec %+v vs code %+v", s, c)
		}
		if c.Description == "" {
			t.Fatalf("%s: no description", c.Name)
		}
		codeReq, _ := c.InputSchema["required"].([]string)
		if !reflect.DeepEqual(append([]string{}, codeReq...), append([]string{}, s.InputSchema.Required...)) {
			t.Fatalf("%s: required %v in code vs %v in spec", c.Name, codeReq, s.InputSchema.Required)
		}
		codeProps, _ := c.InputSchema["properties"].(map[string]any)
		for name := range s.InputSchema.Properties {
			if _, ok := codeProps[name]; !ok {
				t.Fatalf("%s: property %q in spec but not in code", c.Name, name)
			}
		}
	}
}

type bare struct{ *ChecklistGet }

func (bare) Name() string { return "undocumented" }

func TestDescribeUnknownToolHasEmptyObjectSchema(t *testing.T) {
	s := Describe(bare{&ChecklistGet{}})
	if s.Name != "undocumented" || s.InputSchema["type"] != "object" {
		t.Fatalf("%+v", s)
	}
}
