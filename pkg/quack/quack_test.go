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

func TestRenderTemplateDoesntRemoveQuackAnnotations(t *testing.T) {
	values := make(map[string]string)
	input := struct {
		ObjectMeta metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"quack.pusher.com/foo": "bar",
			},
		},
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
		ObjectMeta metav1.ObjectMeta `json:"metadata"`
	}{}
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to unmarshal template output: %v", err)
	}

	assert.Equal(t, input.ObjectMeta.Annotations, output.ObjectMeta.Annotations, "Annotations should not be changed")
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

func TestGetTemplateInput(t *testing.T) {
	type testObject struct {
		metav1.ObjectMeta `json:"metadata"`
		Foo               string            `json:"foo"`
		Status            map[string]string `json:"status"`
	}

	object := testObject{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"annotation":           "value",
				"quack.pusher.com/foo": "bar",
			},
		},
		Foo: "bar",
	}
	objectNoQuackAnnotations := testObject{
		Foo: "bar",
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"annotation": "value",
			},
		},
	}
	ignoredPaths := []string{}

	objectRaw, err := json.Marshal(object)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal input: %v", err)
	}

	template, err := getTemplateInput(objectRaw, ignoredPaths)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getTemplateInput: %v", err)
	}

	templateObject := testObject{}
	err = json.Unmarshal(template, &templateObject)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Error in unmarshall: %v", err)
	}
	assert.Equal(t, objectNoQuackAnnotations, templateObject, "Object should have no quack annotations")
}

func TestGetTemplateInputRemovesIgnoredPaths(t *testing.T) {
	type testObject struct {
		metav1.ObjectMeta `json:"metadata"`
		Foo               string            `json:"foo"`
		Status            map[string]string `json:"status"`
	}

	object := testObject{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"annotation":                "value",
				"quack.pusher.com/template": "true",
				"other/annotation":          "bar",
			},
		},
		Foo: "bar",
	}
	objectNoOtherAnnotation := testObject{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"annotation": "value",
			},
		},
		Foo: "bar",
	}
	ignoredPaths := []string{"/metadata/annotations/other~1annotation"}

	objectRaw, err := json.Marshal(object)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal input: %v", err)
	}
	objectNoOtherRaw, err := json.Marshal(objectNoOtherAnnotation)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal input: %v", err)
	}

	template, err := getTemplateInput(objectRaw, ignoredPaths)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getTemplateInput: %v", err)
	}

	assert.NotNil(t, template, "template should not be nil")

	templateObject := testObject{}
	err = json.Unmarshal(template, &templateObject)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Error in unmarshall: %v", err)
	}
	assert.Equal(t, objectNoOtherAnnotation, templateObject, "Object should have no ignored paths")

	template, err = getTemplateInput(objectNoOtherRaw, ignoredPaths)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getTemplateInput: %v", err)
	}

	assert.NotNil(t, template, "template should not be nil")

	err = json.Unmarshal(template, &templateObject)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Error in unmarshall: %v", err)
	}

	assert.Equal(t, objectNoOtherAnnotation, templateObject, "Object should have no ignored paths")
}

func TestGetTemplateInputRemovesStatus(t *testing.T) {
	type testObject struct {
		metav1.ObjectMeta `json:"metadata"`
		Foo               string            `json:"foo"`
		Status            map[string]string `json:"status"`
	}

	object := testObject{
		Foo: "bar",
		Status: map[string]string{
			"condition": "{{ .Condition }}",
		},
	}
	objectWithoutStatus := testObject{
		Foo: "bar",
	}

	objectRaw, err := json.Marshal(object)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal input: %v", err)
	}
	ignoredPaths := []string{}

	template, err := getTemplateInput(objectRaw, ignoredPaths)
	if err != nil {
		assert.FailNowf(t, "methodError", "Error in getTemplateInput: %v", err)
	}

	templateObject := testObject{}
	err = json.Unmarshal(template, &templateObject)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Error in unmarshall: %v", err)
	}
	assert.Equal(t, objectWithoutStatus, templateObject, "Object should have no quack annotations")
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
	objectWithLeftDelimiter := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				leftDelimAnnotation: "[[",
			},
		},
	}
	objectWithRightDelimiter := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
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
	objectWithLeftDelimiterRaw, err := json.Marshal(objectWithLeftDelimiter)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with left delimeter' input: %v", err)
	}
	objectWithRightDelimiterRaw, err := json.Marshal(objectWithRightDelimiter)
	if err != nil {
		assert.FailNowf(t, "jsonError", "Failed to marshal 'with right delimeter' input: %v", err)
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
	withLeftDelimeter, leftErr := getDelims(objectWithLeftDelimiterRaw)
	withRightDelimeter, rightErr := getDelims(objectWithRightDelimiterRaw)
	withEmptyDelimeters, emptyErr := getDelims(objectWithEmptyDelimitersRaw)

	assert.Equal(t, delimiters{}, withNoAnnotations, "Object with no annotations should return empty delimiters")
	assert.Equal(t, delimiters{left: "[[", right: "]]"}, withSetDelimters, "Object with set delimiters should return `left: [[, right: ]]`")
	assert.Equal(t, delimiters{}, withLeftDelimeter, "Object with empty delimiter should return empty delimiters")
	assert.NotNil(t, leftErr, "Object with only left delimiter should return error")
	assert.Equal(t, delimiters{}, withRightDelimeter, "Object with empty delimiter should return empty delimiters")
	assert.NotNil(t, rightErr, "Object with only right delimiter should return error")
	assert.Equal(t, delimiters{}, withEmptyDelimeters, "Object with empty delimiter should return empty delimiters")
	assert.NotNil(t, emptyErr, "Object with empty left delimiter should return error")
}

func TestRequestHasStatus(t *testing.T) {
	withStatus := `{
			"status": {
				"foo": "bar",
				"baz": 3
			}
		}`
	hasStatus, err := requestHasStatus([]byte(withStatus))
	assert.Equal(t, nil, err, "Error should not have occurred")
	assert.Equal(t, true, hasStatus, "Expected object with status to return true")

	withoutStatus := `{
				"foo": "bar"
			}`
	hasStatus, err = requestHasStatus([]byte(withoutStatus))
	assert.Equal(t, nil, err, "Error should not have occurred")
	assert.Equal(t, false, hasStatus, "Expected object without status to return false")
}
