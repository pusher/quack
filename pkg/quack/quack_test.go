package quack

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		assert.FailNowf(t, "jsonError", "Failed to marshal test input: %v", err)
	}

	fmt.Printf("Template Test Input: %s\n", string(inputBytes))

	outputBytes, err := renderTemplate(inputBytes, values, delimiters{})
	if err != nil {
		assert.FailNowf(t, "methodError", "Failed rendering template: %v", err)
	}

	fmt.Printf("Template Test Output: %s\n", string(outputBytes))
	output := struct {
		Alpha string
		Beta  string
	}{}
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to unmarshal template output: %v", err)
	}

	assert.Equal(t, values["A"], output.Alpha, "Value for A should be substituted for Alpha output")
	assert.Equal(t, values["B"], output.Beta, "Value for B should be substituted for Beta output")
}

func TestRenderTemplateWithDelims(t *testing.T) {
	values := map[string]string{
		"A": "alpha",
		"B": "beta",
	}
	input := struct {
		Alpha string
		Beta  string
	}{
		"[[- .A -]]",
		"[[- .B -]]",
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal test input: %v", err)
	}

	fmt.Printf("Template Test Input: %s\n", string(inputBytes))

	delims := delimiters{
		left:  "[[",
		right: "]]",
	}

	outputBytes, err := renderTemplate(inputBytes, values, delims)
	if err != nil {
		assert.FailNowf(t, "methodError", "Failed rendering template: %v", err)
	}

	fmt.Printf("Template Test Output: %s\n", string(outputBytes))
	output := struct {
		Alpha string
		Beta  string
	}{}
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to unmarshal template output: %v", err)
	}

	assert.Equal(t, values["A"], output.Alpha, "Value for A should be substituted for Alpha output")
	assert.Equal(t, values["B"], output.Beta, "Value for B should be substituted for Beta output")
}

func TestRequestHasAnnotation(t *testing.T) {
	requiredAnnotation := "quack-required"
	objectWithRequired := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				requiredAnnotation: "true",
			},
		},
	}
	objectWithoutRequired := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"quack-not-required": "true",
			},
		},
	}

	objectWithRequiredRaw, err := json.Marshal(objectWithRequired)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with required' input: %v", err)
	}
	objectWithoutRequiredRaw, err := json.Marshal(objectWithoutRequired)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'without required' input: %v", err)
	}

	fmt.Printf("Annotation Test Input (with annotation): %s\n", string(objectWithRequiredRaw))
	fmt.Printf("Annotation Test Input (without annotation): %s\n", string(objectWithoutRequiredRaw))
	withRequired, err := requestHasAnnotation(requiredAnnotation, objectWithRequiredRaw)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in requestHasAnnotation: %v", err)
	}
	withoutRequired, err := requestHasAnnotation(requiredAnnotation, objectWithoutRequiredRaw)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in requestHasAnnotation %v", err)
	}

	noAnnotation, err := requestHasAnnotation("", objectWithRequiredRaw)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in requestHasAnnotation %v", err)
	}

	assert.True(t, withRequired, "Object with required annotation should return true")
	assert.False(t, withoutRequired, "Object without required annotation should return false")
	assert.True(t, noAnnotation, "Specifying no required annotation should return true")
}

func TestGetDelims(t *testing.T) {
	objectWithNoAnnotations := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{}
	objectWithSetDelimiters := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				leftDelimAnnotation:  "[[",
				rightDelimAnnotation: "]]",
			},
		},
	}
	objectWithEmptyDelimiters := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				leftDelimAnnotation:  "",
				rightDelimAnnotation: "]]",
			},
		},
	}

	objectWithNoAnnotationsRaw, err := json.Marshal(objectWithNoAnnotations)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with no annotations' input: %v", err)
	}
	objectWithSetDelimitersRaw, err := json.Marshal(objectWithSetDelimiters)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with set delimeters' input: %v", err)
	}
	objectWithEmptyDelimitersRaw, err := json.Marshal(objectWithEmptyDelimiters)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with empty delimeter' input: %v", err)
	}

	withNoAnnotations, err := getDelims(objectWithNoAnnotationsRaw)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getDelims: %v", err)
	}
	withSetDelimters, err := getDelims(objectWithSetDelimitersRaw)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getDelims: %v", err)
	}
	withEmptyDelimeters, err := getDelims(objectWithEmptyDelimitersRaw)

	assert.Equal(t, delimiters{}, withNoAnnotations, "Object with no annotations should return empty delimiters")
	assert.Equal(t, delimiters{left: "[[", right: "]]"}, withSetDelimters, "Object with set delimiters should return `left: [[, right: ]]`")
	assert.Equal(t, delimiters{}, withEmptyDelimeters, "Object with empty delimiter should return empty delimiters")
	assert.Equal(t, "left delimiter must not be empty", err.Error(), "Object with empty left delimiter should return error")
}
