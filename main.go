package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/kubesphere/logsidecar-injector/injector"
	"golang.org/x/sync/errgroup"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog"
	"net/http"
)

func main() {
	var config injector.Config
	config.AddFlags()
	klog.InitFlags(nil)
	flag.Parse()
	if err := injector.ReloadInjectorConfig(&config); err != nil {
		klog.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	tlsRouter := httprouter.New()
	tlsRouter.POST("/", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		injector.ServeLogSidecarPods(writer, request)
	})

	webReload := make(chan chan error)
	tlsConfig, err := config.TLSConfig(ctx.Done(), webReload)
	if err != nil {
		klog.Fatal(err)
	}
	tlsServer := &http.Server{
		Addr:      ":8443",
		Handler:   tlsRouter,
		TLSConfig: tlsConfig,
	}

	router := httprouter.New()
	router.POST("/-/reload", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		if err := injector.ReloadInjectorConfig(&config); err != nil {
			m := fmt.Sprintf("failed to reload config: %s", err)
			http.Error(writer, m, http.StatusInternalServerError)
			klog.Error(m)
		} else {
			klog.Info("config reloaded")
		}

		errc := make(chan error)
		defer close(errc)
		webReload <- errc
		if err := <-errc; err != nil {
			m := fmt.Sprintf("failed to reload certs: %s", err)
			http.Error(writer, m, http.StatusInternalServerError)
			klog.Error(m)
		} else {
			klog.Info("certs reloaded")
		}
	})
	server := &http.Server{
		Addr:    ":9443",
		Handler: router,
	}

	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		return tlsServer.ListenAndServeTLS("", "")
	})
	wg.Go(func() error {
		return server.ListenAndServe()
	})

	select {
	case <-ctx.Done():
	}
	cancel()
	if err := wg.Wait(); err != nil {
		klog.Fatalf("Unhandled error received: %v. Exiting...\n", err)
	}
}
