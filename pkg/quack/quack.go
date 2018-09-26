package quack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	mergepatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	"github.com/mattbaird/jsonpatch"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

const (
	lastAppliedConfigPath = "/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration"
	quackAnnotationPrefix = "/metadata/annotations/quack.pusher.com"
	leftDelimAnnotation   = "quack.pusher.com/left-delim"
	rightDelimAnnotation  = "quack.pusher.com/right-delim"
)

// AdmissionHook implements the OpenShift MutatingAdmissionHook interface.
// https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/apiserver/apiserver.go#L45
type AdmissionHook struct {
	client             *kubernetes.Clientset // Kubernetes client for calling Api
	ValuesMapName      string                // Source of templating values
	ValuesMapNamespace string                // Namespace the configmap lives in
	RequiredAnnotation string                // Annotation required before templating
}

// Initialize configures the AdmissionHook.
//
// Initializes connection Kubernetes Client
func (ah *AdmissionHook) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
	// Initialise a Kubernetes client
	client, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("failed to intialise kubernetes clientset: %v", err)
	}
	ah.client = client

	glog.Info("Webhook Initialization Complete.")
	return nil
}

// MutatingResource defines where the Webhook is hosted.
func (ah *AdmissionHook) MutatingResource() (schema.GroupVersionResource, string) {
	return schema.GroupVersionResource{
			Group:    "quack.pusher.com",
			Version:  "v1alpha1",
			Resource: "admissionreviews",
		},
		"AdmissionReview"
}

// Admit is the actual business logic of the webhook.
// This is the method that processes the request to the admission controller.
//
// Checks the operation is a create or update operation.
// Loads the template values from the configmap.
// Templates the values into the raw object (json) from the admission request.
// Calculates a JSON Patch to append to the admission response.
func (ah *AdmissionHook) Admit(req *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	resp := &admissionv1beta1.AdmissionResponse{}
	resp.UID = req.UID
	requestName := fmt.Sprintf("%s %s", req.Kind, podID(req.Namespace, req.Name))

	// Skip operations that aren't create or update
	if req.Operation != admissionv1beta1.Create &&
		req.Operation != admissionv1beta1.Update {
		glog.V(2).Infof("Skipping %s request for %s", req.Operation, requestName)
		resp.Allowed = true
		return resp
	}

	// Skip requests that do not have the required annotation
	annototationPresent, err := requestHasAnnotation(ah.RequiredAnnotation, req.Object.Raw)
	if err != nil {
		return errorResponse(resp, "Failed to read annotations: %v", err)
	}
	if !annototationPresent {
		glog.V(2).Infof("Skipping %s request for %s: Required annotation not present.", req.Operation, requestName)
		resp.Allowed = true
		return resp
	}

	glog.V(2).Infof("Processing %s request for %s", req.Operation, requestName)

	// Load template values from configmap
	values, err := getValues(ah.client, ah.ValuesMapNamespace, ah.ValuesMapName)
	if err != nil {
		return errorResponse(resp, "Failed to get template values: %v", err)
	}

	delims, err := getDelims(req.Object.Raw)
	if err != nil {
		return errorResponse(resp, "Invalid delimiters: %v", err)
	}

	templateInput, err := getTemplateInput(req.Object.Raw)
	if err != nil {
		return errorResponse(resp, "")
	}
	// Run Templating
	glog.V(6).Infof("Input for %s: %s", requestName, templateInput)

	output, err := renderTemplate(templateInput, values, delims)
	if err != nil {
		return errorResponse(resp, "Error rendering template: %v", err)
	}
	glog.V(6).Infof("Output for %s: %s", requestName, output)

	// Create a JSON Patch
	// https://tools.ietf.org/html/rfc6902
	patchBytes, err := createPatch(req.Object.Raw, output)
	if err != nil {
		return errorResponse(resp, "Error creating patch: %v", err)
	}

	// If the patch is non-zero, append it
	if string(patchBytes) != "[]" {
		glog.V(2).Infof("Patching %s", requestName)
		glog.V(4).Infof("Patch for %s: %s", requestName, string(patchBytes))
		resp.Patch = patchBytes
		resp.PatchType = func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}()
	}

	resp.Allowed = true
	return resp
}

func renderTemplate(input []byte, values map[string]string, delims delimiters) ([]byte, error) {
	tmpl, err := template.New("object").Delims(delims.left, delims.right).Parse(string(input))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}
	buff := new(bytes.Buffer)
	err = tmpl.Execute(buff, values)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}
	return buff.Bytes(), nil
}

func getValues(client *kubernetes.Clientset, namespace string, name string) (map[string]string, error) {
	getOpts := metav1.GetOptions{}
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(name, getOpts)
	if err != nil {
		return nil, fmt.Errorf("couldn't get configmap: %v", err)
	}
	return cm.Data, nil
}

func createPatch(old []byte, new []byte) ([]byte, error) {
	patch, err := jsonpatch.CreatePatch(old, new)
	if err != nil {
		return nil, fmt.Errorf("error calculating patch: %v", err)
	}

	allowedOps := []jsonpatch.JsonPatchOperation{}
	for _, op := range patch {
		// Don't patch the lastAppliedConfig created by kubectl
		if op.Path == lastAppliedConfigPath || strings.HasPrefix(op.Path, quackAnnotationPrefix) {
			continue
		}
		allowedOps = append(allowedOps, op)
	}

	patchBytes, err := json.Marshal(allowedOps)
	if err != nil {
		return nil, fmt.Errorf("error marshalling patch: %v", err)
	}
	return patchBytes, nil
}

func getTemplateInput(data []byte) ([]byte, error) {
	// Fetch object meta into object
	objectMeta, err := getObjectMeta(data)
	if err != nil {
		return nil, fmt.Errorf("error reading object metadata: %v", err)
	}

	var patchedData []byte
	for annotation := range objectMeta.Annotations {
		if strings.HasPrefix(annotation, "quack.pusher.com") {
			// Remove annotations from input template
			patch := []byte(fmt.Sprintf(`[
				{"op": "remove", "path": "/metadata/annotations/%s"}
			]`, strings.Replace(annotation, "/", "~1", -1)))
			patchedData, err = applyPatch(data, patch)
			if err != nil {
				return nil, fmt.Errorf("error removing annotation %s: %v", annotation, err)
			}
		}
	}

	return patchedData, nil
}

func requestHasAnnotation(requiredAnnotation string, raw []byte) (bool, error) {
	if requiredAnnotation == "" {
		return true, nil
	}

	// Fetch object meta into object
	objectMeta, err := getObjectMeta(raw)
	if err != nil {
		return false, fmt.Errorf("error reading object metadata: %v", err)
	}

	glog.V(6).Infof("Requested Object Annotations: %v", objectMeta.Annotations)

	// Check required annotation exists in struct
	if _, ok := objectMeta.Annotations[requiredAnnotation]; ok {
		return true, nil
	}
	return false, nil
}

func getObjectMeta(raw []byte) (metav1.ObjectMeta, error) {
	requestMeta := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{},
	}
	err := json.Unmarshal(raw, &requestMeta)
	if err != nil {
		return metav1.ObjectMeta{}, fmt.Errorf("failed to unmarshal input: %v", err)
	}
	return requestMeta.ObjectMeta, nil
}

func applyPatch(data, patchBytes []byte) ([]byte, error) {
	patch, err := mergepatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to decode patch: %v", err)
	}

	// Apply patch to remove annotations
	patchedData, err := patch.Apply(data)
	if err != nil {
		return nil, fmt.Errorf("unable to apply patch: %v", err)
	}
	return patchedData, nil
}

type delimiters struct {
	left  string
	right string
}

func getDelims(raw []byte) (delimiters, error) {
	// Fetch object meta into object
	requestMeta := struct {
		metav1.ObjectMeta `json:"metadata"`
	}{
		ObjectMeta: metav1.ObjectMeta{},
	}
	err := json.Unmarshal(raw, &requestMeta)
	if err != nil {
		return delimiters{}, fmt.Errorf("failed ot unmarshal input: %v", err)
	}

	glog.V(6).Infof("Requested Object Annotations: %v", requestMeta.ObjectMeta.Annotations)

	left, lOk := requestMeta.ObjectMeta.Annotations[leftDelimAnnotation]
	right, rOk := requestMeta.ObjectMeta.Annotations[rightDelimAnnotation]

	// If one annotation is set but not the other, this is an error
	if lOk != rOk {
		return delimiters{}, fmt.Errorf("must set either both %s and %s, or neither", leftDelimAnnotation, rightDelimAnnotation)
	}

	// lOk == rOk, if neither set, not an error
	if lOk == false {
		return delimiters{}, nil
	}

	if left == "" || right == "" {
		return delimiters{}, fmt.Errorf("delimiters must not be empty")
	}

	return delimiters{
		left:  left,
		right: right,
	}, nil
}

func errorResponse(resp *admissionv1beta1.AdmissionResponse, message string, args ...interface{}) *admissionv1beta1.AdmissionResponse {
	glog.Errorf(message, args...)
	resp.Allowed = false
	resp.Result = &metav1.Status{
		Status: metav1.StatusFailure, Code: http.StatusInternalServerError, Reason: metav1.StatusReasonInternalError,
		Message: fmt.Sprintf(message, args...),
	}
	return resp
}

func podID(namespace string, name string) string {
	if namespace != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return name
}
