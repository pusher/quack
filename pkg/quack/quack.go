package quack

import (
	"fmt"

	"github.com/golang/glog"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
)

// AdmissionHook implements the OpenShift MutatingAdmissionHook interface.
// https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/apiserver/apiserver.go#L45
type AdmissionHook struct{}

// Initialize configures the AdmissionHook.
func (ah *AdmissionHook) Initialize(kubeClientConfig *restclient.Config, stopCh <-chan struct{}) error {
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

	// Skip operations that aren't create or update
	if req.Operation != admissionv1beta1.Create &&
		req.Operation != admissionv1beta1.Update {
		glog.Infof("Skipping %s request for %s %s", req.Operation, req.Kind, podID(req.Namespace, req.Name))
		resp.Allowed = true
		return resp
	}

	glog.Infof("Processing %s request for %s %s", req.Operation, req.Kind, podID(req.Namespace, req.Name))
	resp.Allowed = true
	return resp
}

func podID(namespace string, name string) string {
	if namespace != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return name
}
