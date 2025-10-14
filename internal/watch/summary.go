package watch

import (
	"fmt"
	"math"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const summaryLimit = 320

func summarise(kind, eventType string, obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	base := fmt.Sprintf("%s %s %s/%s", strings.ToLower(eventType), strings.ToLower(kind), obj.GetNamespace(), obj.GetName())
	switch kind {
	case "Pod":
		return truncate(podSummary(eventType, obj), summaryLimit)
	case "Deployment":
		return truncate(deploymentSummary(eventType, obj), summaryLimit)
	case "Service":
		return truncate(serviceSummary(eventType, obj), summaryLimit)
	case "Ingress":
		return truncate(ingressSummary(eventType, obj), summaryLimit)
	case "Job":
		return truncate(jobSummary(eventType, obj), summaryLimit)
	case "CronJob":
		return truncate(cronJobSummary(eventType, obj), summaryLimit)
	case "HorizontalPodAutoscaler":
		return truncate(hpaSummary(eventType, obj), summaryLimit)
	case "PersistentVolumeClaim":
		return truncate(pvcSummary(eventType, obj), summaryLimit)
	case "ConfigMap":
		return truncate(configMapSummary(eventType, obj), summaryLimit)
	case "Secret":
		return truncate(secretSummary(eventType, obj), summaryLimit)
	case "Event":
		return truncate(eventSummary(obj), summaryLimit)
	case "Certificate":
		return truncate(certificateSummary(eventType, obj), summaryLimit)
	default:
		return truncate(base, summaryLimit)
	}
}

func podSummary(eventType string, obj *unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	restarts := int64(0)
	if statuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses"); found {
		for _, status := range statuses {
			if m, ok := status.(map[string]interface{}); ok {
				restarts += toInt64(m["restartCount"])
			}
		}
	}
	phasePart := ""
	if phase != "" {
		phasePart = fmt.Sprintf(" phase=%s", strings.ToLower(phase))
	}
	restartPart := ""
	if restarts > 0 {
		restartPart = fmt.Sprintf(" restarts=%d", restarts)
	}
	return fmt.Sprintf("%s pod %s/%s%s%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), phasePart, restartPart)
}

func deploymentSummary(eventType string, obj *unstructured.Unstructured) string {
	desired := toInt64(getNested(obj.Object, "spec", "replicas"))
	available := toInt64(getNested(obj.Object, "status", "availableReplicas"))
	updated := toInt64(getNested(obj.Object, "status", "updatedReplicas"))
	return fmt.Sprintf("%s deployment %s/%s ready=%d/%d updated=%d", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), available, desired, updated)
}

func serviceSummary(eventType string, obj *unstructured.Unstructured) string {
	svcType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
	clusterIP, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterIP")
	typePart := strings.ToLower(svcType)
	if typePart == "" {
		typePart = "clusterip"
	}
	ipPart := ""
	if clusterIP != "" && clusterIP != "None" {
		ipPart = " clusterIP=" + clusterIP
	}
	return fmt.Sprintf("%s service %s/%s type=%s%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), typePart, ipPart)
}

func ingressSummary(eventType string, obj *unstructured.Unstructured) string {
	rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	hosts := make([]string, 0, len(rules))
	for _, rule := range rules {
		if m, ok := rule.(map[string]interface{}); ok {
			if host, ok := m["host"].(string); ok && host != "" {
				hosts = append(hosts, host)
			}
		}
	}
	count := len(hosts)
	return fmt.Sprintf("%s ingress %s/%s hosts=%d", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), count)
}

func jobSummary(eventType string, obj *unstructured.Unstructured) string {
	succeeded := toInt64(getNested(obj.Object, "status", "succeeded"))
	failed := toInt64(getNested(obj.Object, "status", "failed"))
	active := toInt64(getNested(obj.Object, "status", "active"))
	return fmt.Sprintf("%s job %s/%s succeeded=%d failed=%d active=%d", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), succeeded, failed, active)
}

func cronJobSummary(eventType string, obj *unstructured.Unstructured) string {
	schedule, _, _ := unstructured.NestedString(obj.Object, "spec", "schedule")
	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	suspendPart := "running"
	if suspended {
		suspendPart = "suspended"
	}
	return fmt.Sprintf("%s cronjob %s/%s schedule=%s status=%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), schedule, suspendPart)
}

func hpaSummary(eventType string, obj *unstructured.Unstructured) string {
	desired := toInt64(getNested(obj.Object, "status", "desiredReplicas"))
	current := toInt64(getNested(obj.Object, "status", "currentReplicas"))
	return fmt.Sprintf("%s hpa %s/%s current=%d desired=%d", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), current, desired)
}

func pvcSummary(eventType string, obj *unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	capacity, _, _ := unstructured.NestedString(obj.Object, "status", "capacity", "storage")
	capPart := ""
	if capacity != "" {
		capPart = " capacity=" + capacity
	}
	return fmt.Sprintf("%s pvc %s/%s phase=%s%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), strings.ToLower(phase), capPart)
}

func configMapSummary(eventType string, obj *unstructured.Unstructured) string {
	data, found, _ := unstructured.NestedMap(obj.Object, "data")
	count := 0
	if found {
		count = len(data)
	}
	return fmt.Sprintf("%s configmap %s/%s data_keys=%d", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), count)
}

func secretSummary(eventType string, obj *unstructured.Unstructured) string {
	secretType, _, _ := unstructured.NestedString(obj.Object, "type")
	if secretType == "" {
		secretType = "Opaque"
	}
	return fmt.Sprintf("%s secret %s/%s type=%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), strings.ToLower(secretType))
}

func eventSummary(obj *unstructured.Unstructured) string {
	reason, _, _ := unstructured.NestedString(obj.Object, "reason")
	typeValue, _, _ := unstructured.NestedString(obj.Object, "type")
	message, _, _ := unstructured.NestedString(obj.Object, "message")
	count := toInt64(getNested(obj.Object, "count"))
	if message == "" {
		message = "(no message)"
	}
	return fmt.Sprintf("event %s %s: %s (count=%d)", strings.ToLower(reason), strings.ToLower(typeValue), message, count)
}

func certificateSummary(eventType string, obj *unstructured.Unstructured) string {
	issuerRef, _, _ := unstructured.NestedMap(obj.Object, "spec", "issuerRef")
	issuerName := ""
	if v, ok := issuerRef["name"].(string); ok {
		issuerName = v
	}
	readyStatus := "unknown"
	if conds, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions"); found {
		for _, cond := range conds {
			if m, ok := cond.(map[string]interface{}); ok {
				typeVal, _ := m["type"].(string)
				statusVal, _ := m["status"].(string)
				if strings.EqualFold(typeVal, "Ready") {
					readyStatus = strings.ToLower(statusVal)
					break
				}
			}
		}
	}
	renewalTime := ""
	if ts, _, _ := unstructured.NestedString(obj.Object, "status", "renewalTime"); ts != "" {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			renewalTime = parsed.UTC().Format(time.RFC3339)
		}
	}
	if renewalTime != "" {
		return fmt.Sprintf("%s certificate %s/%s ready=%s issuer=%s renewal=%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), readyStatus, issuerName, renewalTime)
	}
	return fmt.Sprintf("%s certificate %s/%s ready=%s issuer=%s", strings.ToLower(eventType), obj.GetNamespace(), obj.GetName(), readyStatus, issuerName)
}

func truncate(in string, limit int) string {
	if limit <= 0 || len(in) <= limit {
		return in
	}
	if limit <= 3 {
		return in[:limit]
	}
	return strings.TrimSpace(in[:limit-3]) + "..."
}

func getNested(obj map[string]interface{}, fields ...string) interface{} {
	val, found, _ := unstructured.NestedFieldCopy(obj, fields...)
	if !found {
		return nil
	}
	return val
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return 0
		}
		return int64(t)
	case float32:
		return int64(t)
	case string:
		return 0
	case nil:
		return 0
	default:
		return 0
	}
}
