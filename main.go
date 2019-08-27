package main

import (
	"flag"
	"github.com/kubesphere/logsidecar-injector/injector"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	"net/http"
)

func main() {
	var config injector.Config
	config.AddFlags()
	injector.AddFilebeatTmplFlags()
	klog.InitFlags(nil)
	flag.Parse()

	injector.InitFilebeatTmpl()

	http.HandleFunc("/", injector.ServeLogSidecarPods)

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: injector.ConfigTLS(config),
	}
	server.ListenAndServeTLS("", "")
}
