package quack

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/golang/glog"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// AdmissionHook implements the OpenShift MutatingAdmissionHook interface.
// https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/apiserver/apiserver.go#L45
type AdmissionHook struct {
	client             *kubernetes.Clientset
	ValuesMapName      string
	ValuesMapNamespace string
}

// Initialize configures the AdmissionHook.
func (ah *AdmissionHook) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
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

	values, err := getValues(ah.client, ah.ValuesMapNamespace, ah.ValuesMapName)
	if err != nil {
		glog.Errorf("Failed to get template values: %v", err)
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusInternalServerError, Reason: metav1.StatusReasonInternalError,
			Message: fmt.Sprintf("failed to get template values: %v", err),
		}
		return resp
	}

	glog.V(4).Infof("Input for %s: %s", requestName, string(req.Object.Raw))
	output, err := renderTemplate(req.Object.Raw, values)
	if err != nil {
		glog.Errorf("Error rendering template: %v", err)
		resp.Allowed = false
		resp.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusInternalServerError, Reason: metav1.StatusReasonInternalError,
			Message: fmt.Sprintf("Error rendering template: %v", err),
		}
		return resp
	}
	glog.V(4).Infof("Output for %s: %s", requestName, output)

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

func podID(namespace string, name string) string {
	if namespace != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return name
}
