package kube

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/api/meta"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/discovery"
    "k8s.io/client-go/discovery/cached/memory"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/restmapper"
    "sigs.k8s.io/yaml"
)

// ApplyDir walks a directory and applies every YAML document found.
func ApplyDir(ctx context.Context, cfg *rest.Config, dir string, defaultNS string) error {
    dc, err := dynamic.NewForConfig(cfg)
    if err != nil { return err }
    mapper, err := restMapper(cfg)
    if err != nil { return err }
    return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() { return nil }
        if !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) { return nil }
        f, err := os.Open(path)
        if err != nil { return err }
        defer f.Close()
        return applyFile(ctx, dc, mapper, f, defaultNS)
    })
}

func applyFile(ctx context.Context, dc dynamic.Interface, mapper meta.RESTMapper, r io.Reader, defaultNS string) error {
    reader := bufio.NewReader(r)
    var b strings.Builder
    for {
        line, err := reader.ReadString('\n')
        if err != nil && err != io.EOF { return err }
        if strings.HasPrefix(line, "---") {
            if strings.TrimSpace(b.String()) != "" {
                if err := applyDoc(ctx, dc, mapper, []byte(b.String()), defaultNS); err != nil { return err }
                b.Reset()
            }
        } else {
            b.WriteString(line)
        }
        if err == io.EOF { break }
    }
    if strings.TrimSpace(b.String()) != "" {
        if err := applyDoc(ctx, dc, mapper, []byte(b.String()), defaultNS); err != nil { return err }
    }
    return nil
}

func applyDoc(ctx context.Context, dc dynamic.Interface, mapper meta.RESTMapper, data []byte, defaultNS string) error {
    var obj unstructured.Unstructured
    if err := yaml.Unmarshal(data, &obj.Object); err != nil { return err }
    if obj.GetNamespace() == "" && defaultNS != "" {
        obj.SetNamespace(defaultNS)
    }
    gvk := obj.GroupVersionKind()
    m, err := mapper.RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
    if err != nil { return err }
    var ri dynamic.ResourceInterface
    if m.Scope.Name() == meta.RESTScopeNameNamespace {
        ns := obj.GetNamespace()
        if ns == "" { ns = defaultNS }
        ri = dc.Resource(m.Resource).Namespace(ns)
    } else {
        ri = dc.Resource(m.Resource)
    }
    // Try create, then update on conflict
    _, err = ri.Create(ctx, &obj, metav1.CreateOptions{})
    if errors.IsAlreadyExists(err) {
        current, getErr := ri.Get(ctx, obj.GetName(), metav1.GetOptions{})
        if getErr != nil { return getErr }
        obj.SetResourceVersion(current.GetResourceVersion())
        _, err = ri.Update(ctx, &obj, metav1.UpdateOptions{})
    }
    return err
}

func restMapper(cfg *rest.Config) (meta.RESTMapper, error) {
    dc, err := discovery.NewDiscoveryClientForConfig(cfg)
    if err != nil { return nil, err }
    cached := memory.NewMemCacheClient(dc)
    return restmapper.NewDeferredDiscoveryRESTMapper(cached), nil
}
