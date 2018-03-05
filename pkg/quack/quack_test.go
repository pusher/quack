package quack

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderTemplate(t *testing.T) {
	values := map[string]string{
		"A": "alpha",
		"B": "beta",
	}
	input := struct {
		Alpha string
		Beta  string
	}{
		"{{- .A -}}",
		"{{- .B -}}",
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		assert.Errorf(t, err, "Failed to marshal test input")
	}

	fmt.Printf("Template Test Input: %s\n", string(inputBytes))

	outputBytes, err := renderTemplate(inputBytes, values)
	if err != nil {
		assert.Errorf(t, err, "Failed rendering template")
	}

	fmt.Printf("Template Test Output: %s\n", string(outputBytes))
	output := struct {
		Alpha string
		Beta  string
	}{}
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		assert.Errorf(t, err, "Failed to unmarshal template output")
	}

	assert.Equal(t, values["A"], output.Alpha, "Value for A should be substituted for Alpha output")
	assert.Equal(t, values["B"], output.Beta, "Value for B should be substituted for Beta output")
}
