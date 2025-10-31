package main

import (
    "context"
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "log"
    "math/big"
    "net/http"
    "time"

    // admissionv1 "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"

    api "github.com/vaheed/kubeop/internal/api"
    "github.com/vaheed/kubeop/internal/admission"
)

const (
    ns = "kubeop-system"
    secretName = "kubeop-admission-tls"
    svcName = "kubeop-admission"
    vwhName = "kubeop-admission-webhook"
    mwhName = "kubeop-admission-webhook"
)

func main() {
    cfg, err := rest.InClusterConfig()
    if err != nil { log.Fatalf("in cluster config: %v", err) }
    kc, err := kubernetes.NewForConfig(cfg)
    if err != nil { log.Fatalf("k8s client: %v", err) }

    caPEM, certPEM, keyPEM, err := ensureTLSSecret(kc)
    if err != nil { log.Fatalf("ensure tls: %v", err) }
    if err := patchWebhooksCABundle(kc, caPEM); err != nil { log.Printf("patch webhooks: %v", err) }

    cert, err := tls.X509KeyPair(certPEM, keyPEM)
    if err != nil { log.Fatalf("pair: %v", err) }

    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); w.Write([]byte(`{"service":"admission","version":"dev"}`)) })
    mux.Handle("/metrics", api.PromHandler())
    mux.HandleFunc("/mutate", admission.ServeMutate)
    mux.HandleFunc("/validate", admission.ServeValidate)

    srv := &http.Server{ Addr: ":8443", Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}} }
    log.Println("admission webhook listening on :8443")
    log.Fatal(srv.ListenAndServeTLS("", ""))
}

func ensureTLSSecret(kc *kubernetes.Clientset) ([]byte, []byte, []byte, error) {
    ctx := context.Background()
    sec, err := kc.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
    if err == nil {
        return sec.Data["ca.crt"], sec.Data["tls.crt"], sec.Data["tls.key"], nil
    }
    // generate self-signed CA and server cert for service DNS names
    ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "kubeop-admission-ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365*24*time.Hour), IsCA: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
    caKey, _ := rsa.GenerateKey(rand.Reader, 2048)
    caDER, _ := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
    caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})

    server := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: svcName+"."+ns+".svc"}, DNSNames: []string{svcName, svcName+"."+ns, svcName+"."+ns+".svc"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365*24*time.Hour), ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
    serverKey, _ := rsa.GenerateKey(rand.Reader, 2048)
    serverDER, _ := x509.CreateCertificate(rand.Reader, server, ca, &serverKey.PublicKey, caKey)
    certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
    keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})

    // store secret
    secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns}, Type: corev1.SecretTypeTLS, Data: map[string][]byte{"ca.crt": caPEM, "tls.crt": certPEM, "tls.key": keyPEM}}
    _, err = kc.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
    if err != nil { return nil, nil, nil, err }
    return caPEM, certPEM, keyPEM, nil
}

func patchWebhooksCABundle(kc *kubernetes.Clientset, ca []byte) error {
    ctx := context.Background()
    // Patch ValidatingWebhookConfiguration
    patch := []byte(`[{"op":"replace","path":"/webhooks/0/clientConfig/caBundle","value":"` + pemB64(ca) + `"}]`)
    _, err := kc.AdmissionregistrationV1().ValidatingWebhookConfigurations().Patch(ctx, vwhName, types.JSONPatchType, patch, metav1.PatchOptions{})
    if err != nil { return err }
    // Patch MutatingWebhookConfiguration
    _, err = kc.AdmissionregistrationV1().MutatingWebhookConfigurations().Patch(ctx, mwhName, types.JSONPatchType, patch, metav1.PatchOptions{})
    return err
}

func pemB64(b []byte) string { return string(b) }
