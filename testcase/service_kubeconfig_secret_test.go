package testcase

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"kubeop/internal/service"
)

func TestMintServiceAccountSecretWaitsForToken(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := fake.NewSimpleClientset()
	if _, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant"}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	if _, err := client.CoreV1().ServiceAccounts("tenant").Create(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "user-sa"}}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create sa: %v", err)
	}

	go func() {
		// Wait for secret creation then populate data
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				secrets, _ := client.CoreV1().Secrets("tenant").List(ctx, metav1.ListOptions{})
				for _, s := range secrets.Items {
					if s.Type == corev1.SecretTypeServiceAccountToken {
						s.Data = map[string][]byte{
							corev1.ServiceAccountTokenKey: []byte("tok"),
							"ca.crt":                      []byte("CA"),
						}
						_, _ = client.CoreV1().Secrets("tenant").Update(ctx, &s, metav1.UpdateOptions{})
						return
					}
				}
			}
		}
	}()

	svc := &service.Service{}
	secret, err := service.TestMintServiceAccountSecret(svc, ctx, client, "tenant", "user-sa")
	if err != nil {
		t.Fatalf("mint secret failed: %v", err)
	}
	if len(secret.Data[corev1.ServiceAccountTokenKey]) == 0 {
		t.Fatalf("expected token data")
	}
	if len(secret.Data["ca.crt"]) == 0 {
		t.Fatalf("expected ca data")
	}
}
