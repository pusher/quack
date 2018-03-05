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

type admissionHook apiserver.AdmissionHook

func main() {
	flagset := pflag.NewFlagSet("quack", pflag.ExitOnError)

	ah := &quack.AdmissionHook{}

	flagset.StringVarP(&ah.ValuesMapName, "values-configmap", "c", "quack-values", "Defines the name of the ConfigMap to load templating values from")
	flagset.StringVarP(&ah.ValuesMapNamespace, "values-configmap-namespace", "n", "quack", "Defines the namespace to load the Values ConfigMap from")

	runAdmissionServer(flagset, ah)
}

// Originally from: https://github.com/openshift/generic-admission-server/blob/v1.9.0/pkg/cmd/cmd.go
func runAdmissionServer(flagset *pflag.FlagSet, admissionHooks ...admissionHook) {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	stopCh := genericapiserver.SetupSignalHandler()

	// done to avoid cannot use admissionHooks (type []AdmissionHook) as type []apiserver.AdmissionHook in argument to "github.com/openshift/kubernetes-namespace-reservation/pkg/genericadmissionserver/cmd/server".NewCommandStartAdmissionServer
	var castSlice []apiserver.AdmissionHook
	for i := range admissionHooks {
		castSlice = append(castSlice, admissionHooks[i])
	}

	cmd := server.NewCommandStartAdmissionServer(os.Stdout, os.Stderr, stopCh, castSlice...)
	cmd.Short = "Launch Quack Templating Server"
	cmd.Long = "Launch Quack Templating Server"

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	cmd.PersistentFlags().AddFlagSet(flagset)
	flag.CommandLine.Parse([]string{})

	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}
