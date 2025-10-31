package kube

import (
    "context"
    "os"

    appsv1 "k8s.io/api/apps/v1"
    autoscalingv2 "k8s.io/api/autoscaling/v2"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
)

// GetConfigFromEnv returns a rest.Config from $KUBECONFIG or in-cluster config.
func GetConfigFromEnv() (*rest.Config, error) {
    if path := os.Getenv("KUBECONFIG"); path != "" {
        return clientcmd.BuildConfigFromFlags("", path)
    }
    return rest.InClusterConfig()
}

// EnsureNamespace ensures a namespace exists.
func EnsureNamespace(ctx context.Context, kc *kubernetes.Clientset, name string, labels map[string]string) error {
    if _, err := kc.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{}); err == nil {
        return nil
    }
    ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
    _, err := kc.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
    return err
}

// UpsertConfigMap creates or updates a ConfigMap.
func UpsertConfigMap(ctx context.Context, kc *kubernetes.Clientset, ns, name string, data map[string]string) error {
    cm, err := kc.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
    if err == nil {
        cm.Data = data
        _, err = kc.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
        return err
    }
    cm = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name}, Data: data}
    _, err = kc.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
    return err
}

// EnsureAdmissionEnvFromConfigMap adds envFrom to admission deployment for a ConfigMap.
func EnsureAdmissionEnvFromConfigMap(ctx context.Context, kc *kubernetes.Clientset, ns, deployName, cmName string) error {
    d, err := kc.AppsV1().Deployments(ns).Get(ctx, deployName, metav1.GetOptions{})
    if err != nil { return err }
    if len(d.Spec.Template.Spec.Containers) == 0 {
        return nil
    }
    // check if already present
    for _, ef := range d.Spec.Template.Spec.Containers[0].EnvFrom {
        if ef.ConfigMapRef != nil && ef.ConfigMapRef.Name == cmName { return nil }
    }
    d.Spec.Template.Spec.Containers[0].EnvFrom = append(d.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: cmName}}})
    _, err = kc.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
    return err
}

// EnsureOperatorHPA creates or updates an HPA for the operator Deployment.
func EnsureOperatorHPA(ctx context.Context, kc *kubernetes.Clientset, ns string, enabled bool, min, max int32, targetCPU int32) error {
    name := "kubeop-operator"
    hpan := name
    if !enabled {
        // delete HPA if exists
        _ = kc.AutoscalingV2().HorizontalPodAutoscalers(ns).Delete(ctx, hpan, metav1.DeleteOptions{})
        return nil
    }
    hpa, err := kc.AutoscalingV2().HorizontalPodAutoscalers(ns).Get(ctx, hpan, metav1.GetOptions{})
    targetType := autoscalingv2.UtilizationMetricType
    cpu := corev1.ResourceCPU
    metric := autoscalingv2.MetricSpec{
        Type: autoscalingv2.ResourceMetricSourceType,
        Resource: &autoscalingv2.ResourceMetricSource{
            Name: cpu,
            Target: autoscalingv2.MetricTarget{
                Type:               targetType,
                AverageUtilization: &targetCPU,
            },
        },
    }
    if err == nil {
        hpa.Spec.MinReplicas = &min
        hpa.Spec.MaxReplicas = max
        hpa.Spec.Metrics = []autoscalingv2.MetricSpec{metric}
        _, err = kc.AutoscalingV2().HorizontalPodAutoscalers(ns).Update(ctx, hpa, metav1.UpdateOptions{})
        return err
    }
    // create
    hpa = &autoscalingv2.HorizontalPodAutoscaler{
        ObjectMeta: metav1.ObjectMeta{Name: hpan, Namespace: ns},
        Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
            ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: name, APIVersion: appsv1.SchemeGroupVersion.String()},
            MinReplicas:    &min,
            MaxReplicas:    max,
            Metrics:        []autoscalingv2.MetricSpec{metric},
            Behavior:       nil,
        },
    }
    _, err = kc.AutoscalingV2().HorizontalPodAutoscalers(ns).Create(ctx, hpa, metav1.CreateOptions{})
    return err
}

// SetDeploymentReplicas sets replica count for deployment.
func SetDeploymentReplicas(ctx context.Context, kc *kubernetes.Clientset, ns, name string, replicas int32) error {
    d, err := kc.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
    if err != nil { return err }
    d.Spec.Replicas = &replicas
    _, err = kc.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
    return err
}
