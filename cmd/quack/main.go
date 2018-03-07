package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/golang/glog"
	"github.com/openshift/generic-admission-server/pkg/apiserver"
	"github.com/openshift/generic-admission-server/pkg/cmd/server"
	"github.com/pusher/quack/pkg/quack"
	"github.com/spf13/pflag"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/logs"
)

func main() {
	flagset := pflag.NewFlagSet("quack", pflag.ExitOnError)

	// quack.AdmissionHook is the main package
	ah := &quack.AdmissionHook{}

	// Set flags to populate admission hook configuration
	flagset.StringVarP(&ah.ValuesMapName, "values-configmap", "c", "quack-values", "Defines the name of the ConfigMap to load templating values from")
	flagset.StringVarP(&ah.ValuesMapNamespace, "values-configmap-namespace", "n", "quack", "Defines the namespace to load the Values ConfigMap from")
	flagset.StringVarP(&ah.RequiredAnnotation, "required-annotation", "a", "", "Require annotation on objects before templating them")

	// Run server
	runAdmissionServer(flagset, ah)
}

// Originally from: https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/cmd/cmd.go
func runAdmissionServer(flagset *pflag.FlagSet, admissionHooks ...apiserver.AdmissionHook) {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	stopCh := genericapiserver.SetupSignalHandler()

	cmd := server.NewCommandStartAdmissionServer(os.Stdout, os.Stderr, stopCh, admissionHooks...)
	cmd.Short = "Launch Quack Templating Server"
	cmd.Long = "Launch Quack Templating Server"

	// Add admission hook flags
	cmd.PersistentFlags().AddFlagSet(flagset)

	// Flags for glog
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	// Fix glog printing "Error: logging before flag.Parse"
	flag.CommandLine.Parse([]string{})

	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}
