package main

import (
    "flag"
    "os"
    "net/http"

    corev1 "k8s.io/api/core/v1"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"

    v1alpha1 "github.com/vaheed/kubeop/internal/operator/apis/paas/v1alpha1"
    "github.com/vaheed/kubeop/internal/operator/controllers"
    api "github.com/vaheed/kubeop/internal/api"
)

func main() {
    var metricsAddr string
    var healthAddr string
    var leaderElect bool
    flag.StringVar(&metricsAddr, "metrics-bind-address", ":8081", "metrics address")
    flag.StringVar(&healthAddr, "health-probe-bind-address", ":8082", "health address")
    flag.BoolVar(&leaderElect, "leader-elect", false, "enable leader election")
    flag.Parse()

    ctrl.SetLogger(zap.New())

    scheme := clientgoscheme.Scheme
    _ = corev1.AddToScheme(scheme)
    _ = v1alpha1.AddToScheme(scheme)

    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{Scheme: scheme})
    if err != nil {
        panic(err)
    }

    if err := (&controllers.TenantReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil { panic(err) }
    if err := (&controllers.ProjectReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil { panic(err) }
    if err := (&controllers.AppReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil { panic(err) }
    dnsURL := os.Getenv("DNS_MOCK_URL")
    acmeURL := os.Getenv("ACME_MOCK_URL")
    if err := (&controllers.DNSRecordReconciler{Client: mgr.GetClient(), Endpoint: dnsURL}).SetupWithManager(mgr); err != nil { panic(err) }
    if err := (&controllers.CertificateReconciler{Client: mgr.GetClient(), Endpoint: acmeURL}).SetupWithManager(mgr); err != nil { panic(err) }

    // Lightweight HTTP for health/version/metrics using default registry
    go func() {
        mux := http.NewServeMux()
        mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
        mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
        mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); w.Write([]byte(`{"service":"operator","version":"dev"}`)) })
        mux.Handle("/metrics", api.PromHandler())
        _ = http.ListenAndServe(":8082", mux)
    }()

    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        os.Exit(1)
    }
}
