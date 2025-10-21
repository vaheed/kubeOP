package service

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ociBundleLayerMediaTypeTar           = "application/vnd.kubeop.bundle.v1+tar"
	ociBundleLayerMediaTypeTarGzip       = "application/vnd.kubeop.bundle.v1+tar+gzip"
	ociBundleFallbackMediaTypeTar        = string(ocispec.MediaTypeImageLayer)
	ociBundleFallbackMediaTypeGzip       = string(ocispec.MediaTypeImageLayerGzip)
	ociBundleMaxManifestBytes      int64 = 8 * 1024 * 1024 // 8 MiB safety limit
)

type ociBundlePlan struct {
	Ref          string
	CredentialID string
	Insecure     bool
	Digest       string
	MediaType    string
}

type ociBundleFetchResult struct {
	Documents []string
	Digest    string
	MediaType string
}

// OCIRegistryAuth exposes registry credentials for tests via type aliasing.
type OCIRegistryAuth = helmOCIAuth

// OCIBundleFetchResult exposes fetch outputs for tests via type aliasing.
type OCIBundleFetchResult = ociBundleFetchResult

var (
	ociBundleFetcherMu sync.RWMutex
	ociBundleFetcher   = defaultOCIBundleFetcher
)

// SetOCIBundleFetcher overrides the bundle fetcher. Primarily used in tests.
func SetOCIBundleFetcher(fn func(context.Context, string, bool, *OCIRegistryAuth) (OCIBundleFetchResult, error)) func() {
	if fn == nil {
		fn = defaultOCIBundleFetcher
	}
	ociBundleFetcherMu.Lock()
	prev := ociBundleFetcher
	ociBundleFetcher = fn
	ociBundleFetcherMu.Unlock()
	return func() {
		ociBundleFetcherMu.Lock()
		ociBundleFetcher = prev
		ociBundleFetcherMu.Unlock()
	}
}

func fetchOCIBundle(ctx context.Context, ref string, insecure bool, auth *helmOCIAuth) (ociBundleFetchResult, error) {
	ociBundleFetcherMu.RLock()
	fn := ociBundleFetcher
	ociBundleFetcherMu.RUnlock()
	return fn(ctx, ref, insecure, auth)
}

func defaultOCIBundleFetcher(ctx context.Context, ref string, insecure bool, auth *helmOCIAuth) (ociBundleFetchResult, error) {
	host, repo, err := parseOCIReference(ref)
	if err != nil {
		return ociBundleFetchResult{}, err
	}
	if !strings.Contains(repo, ":") && !strings.Contains(repo, "@") {
		return ociBundleFetchResult{}, errors.New("oci bundle ref must include a tag or digest")
	}
	addrs, err := resolveHelmChartTarget(ctx, host)
	if err != nil {
		return ociBundleFetchResult{}, err
	}
	transport := newOCIRegistryTransport(host, addrs)
	nameOpts := []name.Option{}
	if insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	parsedRef, err := name.ParseReference(strings.TrimPrefix(ref, "oci://"), nameOpts...)
	if err != nil {
		return ociBundleFetchResult{}, fmt.Errorf("parse oci bundle ref: %w", err)
	}
	authOpt := remote.WithAuth(authn.Anonymous)
	if auth != nil && (auth.Username != "" || auth.Password != "") {
		authOpt = remote.WithAuth(authn.FromConfig(authn.AuthConfig{Username: auth.Username, Password: auth.Password}))
	}
	img, err := remote.Image(parsedRef, remote.WithContext(ctx), remote.WithTransport(transport), authOpt)
	if err != nil {
		return ociBundleFetchResult{}, fmt.Errorf("fetch oci bundle: %w", err)
	}
	digest, err := img.Digest()
	if err != nil {
		return ociBundleFetchResult{}, fmt.Errorf("digest oci bundle: %w", err)
	}
	layers, err := img.Layers()
	if err != nil {
		return ociBundleFetchResult{}, fmt.Errorf("list oci bundle layers: %w", err)
	}
	for _, layer := range layers {
		mt, err := layer.MediaType()
		if err != nil {
			return ociBundleFetchResult{}, fmt.Errorf("oci bundle layer media type: %w", err)
		}
		mtStr := string(mt)
		if !isSupportedOCIBundleLayer(mtStr) {
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return ociBundleFetchResult{}, fmt.Errorf("read oci bundle layer: %w", err)
		}
		docs, readErr := extractOCIBundleDocuments(rc, ociBundleMaxManifestBytes)
		rc.Close()
		if readErr != nil {
			return ociBundleFetchResult{}, readErr
		}
		return ociBundleFetchResult{Documents: docs, Digest: digest.String(), MediaType: mtStr}, nil
	}
	return ociBundleFetchResult{}, errors.New("oci bundle did not contain a supported manifest layer")
}

func isSupportedOCIBundleLayer(mediaType string) bool {
	switch mediaType {
	case ociBundleLayerMediaTypeTar, ociBundleLayerMediaTypeTarGzip, ociBundleFallbackMediaTypeTar, ociBundleFallbackMediaTypeGzip:
		return true
	default:
		return false
	}
}

func extractOCIBundleDocuments(r io.Reader, maxBytes int64) ([]string, error) {
	if maxBytes <= 0 {
		return nil, errors.New("oci bundle size limit must be positive")
	}
	tr := tar.NewReader(r)
	var docs []string
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read oci bundle: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		clean := filepath.Clean(hdr.Name)
		if clean == "." || clean == string(filepath.Separator) || clean == "" {
			continue
		}
		if strings.HasPrefix(clean, "..") || strings.Contains(clean, "../") || strings.HasPrefix(clean, string(filepath.Separator)) {
			return nil, fmt.Errorf("oci bundle contains invalid path %q", hdr.Name)
		}
		ext := strings.ToLower(filepath.Ext(clean))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		remaining := maxBytes - total
		if remaining <= 0 {
			return nil, errors.New("oci bundle exceeds maximum manifest size")
		}
		var buf bytes.Buffer
		n, err := io.Copy(&buf, io.LimitReader(tr, remaining+1))
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read oci bundle file %s: %w", hdr.Name, err)
		}
		if int64(buf.Len()) > remaining {
			return nil, errors.New("oci bundle exceeds maximum manifest size")
		}
		total += n
		content := strings.TrimSpace(buf.String())
		if content != "" {
			docs = append(docs, content)
		}
	}
	if len(docs) == 0 {
		return nil, errors.New("oci bundle did not contain any Kubernetes manifests")
	}
	return docs, nil
}

func newOCIRegistryTransport(host string, addrs []netip.Addr) *http.Transport {
	allowed := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		allowed[addr.String()] = struct{}{}
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			hostPart, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ip := net.ParseIP(hostPart)
			if ip == nil {
				return nil, fmt.Errorf("oci bundle registry dial: non-ip address %s", hostPart)
			}
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				return nil, fmt.Errorf("oci bundle registry dial: invalid ip %s", hostPart)
			}
			if err := ensureHelmChartAddrAllowed(host, addr); err != nil {
				return nil, err
			}
			if _, ok := allowed[addr.String()]; !ok {
				return nil, fmt.Errorf("oci bundle registry dial: %s not in allowed targets", addr.String())
			}
			if port != "80" && port != "443" {
				return nil, fmt.Errorf("oci bundle registry dial: port %s is not permitted", port)
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		},
		ForceAttemptHTTP2: true,
	}
}
