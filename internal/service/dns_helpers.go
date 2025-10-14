package service

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// DNSLogFields returns structured log fields describing the DNS workflow context.
func DNSLogFields(projectID, appID, clusterID, namespace, serviceName, host string) []zap.Field {
	fields := []zap.Field{
		zap.String("project_id", projectID),
		zap.String("app_id", appID),
	}
	if clusterID != "" {
		fields = append(fields, zap.String("cluster_id", clusterID))
	}
	if namespace != "" {
		fields = append(fields, zap.String("namespace", namespace))
	}
	if serviceName != "" {
		fields = append(fields, zap.String("service", serviceName))
	}
	if host != "" {
		fields = append(fields, zap.String("host", host))
	}
	return fields
}

func (s *Service) dnsLogger(projectID, appID, clusterID, namespace, serviceName, host string) *zap.Logger {
	return s.logger.With(DNSLogFields(projectID, appID, clusterID, namespace, serviceName, host)...)
}

// DNSError annotates DNS workflow failures with contextual identifiers.
func DNSError(operation, clusterID, namespace, serviceName, host string, err error) error {
	if err == nil {
		return nil
	}
	ctx := make([]string, 0, 4)
	if clusterID != "" {
		ctx = append(ctx, "cluster="+clusterID)
	}
	if namespace != "" {
		ctx = append(ctx, "namespace="+namespace)
	}
	if serviceName != "" {
		ctx = append(ctx, "service="+serviceName)
	}
	if host != "" {
		ctx = append(ctx, "host="+host)
	}
	if len(ctx) == 0 {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s (%s): %w", operation, strings.Join(ctx, " "), err)
}
