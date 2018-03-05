package quack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

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
)

// AdmissionHook implements the OpenShift MutatingAdmissionHook interface.
// https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/apiserver/apiserver.go#L45
type AdmissionHook struct {
	client             *kubernetes.Clientset // Kubernetes client for calling Api
	ValuesMapName      string                // Source of templating values
	ValuesMapNamespace string                // Namespace the configmap lives in
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

	glog.V(2).Infof("Processing %s request for %s", req.Operation, requestName)

	// Load template values from configmap
	values, err := getValues(ah.client, ah.ValuesMapNamespace, ah.ValuesMapName)
	if err != nil {
		return errorResponse(resp, "Failed to get template values: %v", err)
	}

	// Run Templating
	glog.V(6).Infof("Input for %s: %s", requestName, string(req.Object.Raw))
	output, err := renderTemplate(req.Object.Raw, values)
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

func renderTemplate(input []byte, values map[string]string) ([]byte, error) {
	tmpl, err := template.New("object").Parse(string(input))
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
		if op.Path == lastAppliedConfigPath {
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
