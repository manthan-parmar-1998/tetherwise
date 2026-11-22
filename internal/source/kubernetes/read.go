package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	kyaml "sigs.k8s.io/yaml"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// Recorder receives one entry per external API read.
type Recorder interface {
	Record(service, operation, target string) error
}

// ReadFiles loads manifests from files and directories.
// known are resources already discovered (for example from Terraform) whose
// endpoints can be matched against env vars and log shippers.
func ReadFiles(paths []string, known []model.Resource) ([]model.Resource, []model.Dependency, error) {
	var objs []runtime.Object
	for _, path := range paths {
		got, err := readPath(path)
		if err != nil {
			return nil, nil, err
		}
		objs = append(objs, got...)
	}
	return extract(objs, known)
}

// ReadCluster lists workloads and ConfigMaps. It calls List only.
func ReadCluster(ctx context.Context, client kubernetes.Interface, known []model.Resource, rec Recorder) ([]model.Resource, []model.Dependency, error) {
	if client == nil {
		return nil, nil, errors.New("kubernetes client is nil")
	}
	var objs []runtime.Object
	lists := []struct {
		target string
		list   func() ([]runtime.Object, error)
	}{
		{"apps/v1/deployments", func() ([]runtime.Object, error) {
			got, err := client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return toObjects(got)
		}},
		{"apps/v1/statefulsets", func() ([]runtime.Object, error) {
			got, err := client.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return toObjects(got)
		}},
		{"apps/v1/daemonsets", func() ([]runtime.Object, error) {
			got, err := client.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return toObjects(got)
		}},
		{"v1/pods", func() ([]runtime.Object, error) {
			got, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return toObjects(got)
		}},
		{"v1/configmaps", func() ([]runtime.Object, error) {
			got, err := client.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			return toObjects(got)
		}},
	}
	for _, item := range lists {
		if rec != nil {
			if err := rec.Record("kubernetes", "List", item.target); err != nil {
				return nil, nil, fmt.Errorf("audit kubernetes list: %w", err)
			}
		}
		got, err := item.list()
		if err != nil {
			return nil, nil, fmt.Errorf("list %s: %w", item.target, err)
		}
		objs = append(objs, got...)
	}
	return extract(objs, known)
}

func toObjects(list metav1.ListInterface) ([]runtime.Object, error) {
	items, err := metaItems(list)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func metaItems(list metav1.ListInterface) ([]runtime.Object, error) {
	switch l := list.(type) {
	case *appsv1.DeploymentList:
		out := make([]runtime.Object, len(l.Items))
		for i := range l.Items {
			out[i] = &l.Items[i]
		}
		return out, nil
	case *appsv1.StatefulSetList:
		out := make([]runtime.Object, len(l.Items))
		for i := range l.Items {
			out[i] = &l.Items[i]
		}
		return out, nil
	case *appsv1.DaemonSetList:
		out := make([]runtime.Object, len(l.Items))
		for i := range l.Items {
			out[i] = &l.Items[i]
		}
		return out, nil
	case *corev1.PodList:
		out := make([]runtime.Object, len(l.Items))
		for i := range l.Items {
			out[i] = &l.Items[i]
		}
		return out, nil
	case *corev1.ConfigMapList:
		out := make([]runtime.Object, len(l.Items))
		for i := range l.Items {
			out[i] = &l.Items[i]
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported list %T", list)
	}
}

func readPath(path string) ([]runtime.Object, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat kubernetes path: %w", err)
	}
	if !info.IsDir() {
		return readFile(path)
	}
	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".yaml", ".yml", ".json":
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", path, err)
	}
	sort.Strings(files)
	var objs []runtime.Object
	for _, file := range files {
		got, err := readFile(file)
		if err != nil {
			return nil, err
		}
		objs = append(objs, got...)
	}
	return objs, nil
}

func readFile(path string) ([]runtime.Object, error) {
	f, err := os.Open(path) //nolint:gosec // G304: operator-selected manifest
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	objs, err := DecodeDocuments(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return objs, nil
}

var (
	scheme    = runtime.NewScheme()
	decode    runtime.Decoder
	schemeErr error
)

func init() {
	schemeErr = errors.Join(
		corev1.AddToScheme(scheme),
		appsv1.AddToScheme(scheme),
	)
	if schemeErr == nil {
		decode = serializer.NewCodecFactory(scheme).UniversalDeserializer()
	}
}

// DecodeDocuments parses a YAML or JSON stream into typed API objects.
// Documents whose kind is not a workload or ConfigMap are skipped.
func DecodeDocuments(r io.Reader) ([]runtime.Object, error) {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(r))
	var objs []runtime.Object
	for {
		doc, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return objs, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read document: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		obj, skip, err := decodeOne(doc)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		objs = append(objs, obj)
	}
}

func decodeOne(doc []byte) (runtime.Object, bool, error) {
	if schemeErr != nil {
		return nil, false, schemeErr
	}
	jsonDoc, err := kyaml.YAMLToJSON(doc)
	if err != nil {
		return nil, false, fmt.Errorf("yaml to json: %w", err)
	}
	if len(bytes.TrimSpace(jsonDoc)) == 0 || string(bytes.TrimSpace(jsonDoc)) == "null" {
		return nil, true, nil
	}
	obj, _, err := decode.Decode(jsonDoc, nil, nil)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("decode object: %w", err)
	}
	return obj, false, nil
}

type workload struct {
	res  model.Resource
	spec corev1.PodSpec
}

type configMap struct {
	res  model.Resource
	data map[string]string
}

func extract(objs []runtime.Object, known []model.Resource) ([]model.Resource, []model.Dependency, error) {
	var workloads []workload
	maps := map[string]configMap{}
	for _, obj := range objs {
		switch o := obj.(type) {
		case *appsv1.Deployment:
			workloads = append(workloads, workloadFrom("deployment", o.ObjectMeta, o.Spec.Template.ObjectMeta, o.Spec.Template.Spec))
		case *appsv1.StatefulSet:
			workloads = append(workloads, workloadFrom("statefulset", o.ObjectMeta, o.Spec.Template.ObjectMeta, o.Spec.Template.Spec))
		case *appsv1.DaemonSet:
			workloads = append(workloads, workloadFrom("daemonset", o.ObjectMeta, o.Spec.Template.ObjectMeta, o.Spec.Template.Spec))
		case *corev1.Pod:
			workloads = append(workloads, workloadFrom("pod", o.ObjectMeta, metav1.ObjectMeta{}, o.Spec))
		case *corev1.ConfigMap:
			cm := configMapFrom(o)
			maps[cm.res.Name] = cm
		default:
			continue
		}
	}

	resources := make([]model.Resource, 0, len(workloads)+len(maps))
	var deps []model.Dependency
	seenRes := map[string]model.Resource{}
	for _, w := range workloads {
		seenRes[w.res.ID] = w.res
		deps = append(deps, envDeps(w, known)...)
		for _, name := range mountedConfigMaps(w.spec) {
			cm, ok := maps[nsName(namespaceOf(w.res), name)]
			if !ok {
				continue
			}
			deps = append(deps, shipperDeps(w.res.ID, cm.data, known)...)
		}
	}
	mounted := map[string]struct{}{}
	for _, w := range workloads {
		for _, name := range mountedConfigMaps(w.spec) {
			mounted[nsName(namespaceOf(w.res), name)] = struct{}{}
		}
	}
	for key, cm := range maps {
		seenRes[cm.res.ID] = cm.res
		if _, ok := mounted[key]; ok {
			continue
		}
		deps = append(deps, shipperDeps(cm.res.ID, cm.data, known)...)
	}
	for _, res := range seenRes {
		resources = append(resources, res)
	}
	model.SortResources(resources)
	model.SortDependencies(deps)
	return resources, deps, nil
}

func workloadFrom(kind string, meta, podMeta metav1.ObjectMeta, spec corev1.PodSpec) workload {
	ns := meta.Namespace
	if ns == "" {
		ns = "default"
	}
	team, owner := teamOwner(meta, podMeta)
	res := model.Resource{
		ID:       "k8s:" + kind + "/" + ns + "/" + meta.Name,
		Provider: "k8s",
		Kind:     kind,
		Name:     ns + "/" + meta.Name,
		Team:     team,
		Owner:    owner,
	}
	return workload{res: res, spec: spec}
}

func configMapFrom(cm *corev1.ConfigMap) configMap {
	ns := cm.Namespace
	if ns == "" {
		ns = "default"
	}
	team, owner := teamOwner(cm.ObjectMeta)
	res := model.Resource{
		ID:       "k8s:configmap/" + ns + "/" + cm.Name,
		Provider: "k8s",
		Kind:     "configmap",
		Name:     ns + "/" + cm.Name,
		Team:     team,
		Owner:    owner,
	}
	return configMap{res: res, data: cm.Data}
}

func namespaceOf(res model.Resource) string {
	ns, _, _ := strings.Cut(res.Name, "/")
	return ns
}

func nsName(ns, name string) string {
	return ns + "/" + name
}

func teamOwner(metas ...metav1.ObjectMeta) (team, owner string) {
	for _, meta := range metas {
		if team == "" {
			team = first(meta.Labels["team"], meta.Annotations["team"], meta.Annotations["tetherwise.io/team"])
		}
		if owner == "" {
			owner = first(meta.Labels["owner"], meta.Annotations["owner"], meta.Annotations["tetherwise.io/owner"])
		}
	}
	return team, owner
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func envDeps(w workload, known []model.Resource) []model.Dependency {
	var deps []model.Dependency
	for _, c := range allContainers(w.spec) {
		for _, env := range c.Env {
			if env.Value == "" {
				continue
			}
			for _, match := range matchKnown(env.Value, known) {
				deps = append(deps, model.Dependency{
					From: w.res.ID, To: match.id, Kind: kindFromEnv(env.Name),
					Source: model.SourceK8sEnv, Sources: []string{model.SourceK8sEnv},
					Confidence: match.confidence,
				})
			}
		}
	}
	return deps
}

func kindFromEnv(name string) string {
	n := strings.ToUpper(name)
	switch {
	case strings.Contains(n, "LOG"):
		return model.KindWritesLogsTo
	case strings.Contains(n, "READ"), strings.Contains(n, "QUERY"):
		return model.KindReadsFrom
	default:
		return model.KindConnectsTo
	}
}

func allContainers(spec corev1.PodSpec) []corev1.Container {
	out := make([]corev1.Container, 0, len(spec.Containers)+len(spec.InitContainers))
	out = append(out, spec.Containers...)
	out = append(out, spec.InitContainers...)
	return out
}

func mountedConfigMaps(spec corev1.PodSpec) []string {
	var names []string
	seen := map[string]struct{}{}
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for _, vol := range spec.Volumes {
		if vol.ConfigMap != nil {
			add(vol.ConfigMap.Name)
		}
	}
	for _, c := range allContainers(spec) {
		for _, env := range c.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				add(env.ValueFrom.ConfigMapKeyRef.Name)
			}
		}
		for _, from := range c.EnvFrom {
			if from.ConfigMapRef != nil {
				add(from.ConfigMapRef.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

func shipperDeps(from string, data map[string]string, known []model.Resource) []model.Dependency {
	var deps []model.Dependency
	for _, body := range data {
		for _, host := range fluentBitHosts(body) {
			deps = append(deps, hostDeps(from, host, model.SourceFluentBit, known)...)
		}
		for _, host := range vectorEndpoints(body) {
			deps = append(deps, hostDeps(from, host, model.SourceVector, known)...)
		}
	}
	return deps
}

func hostDeps(from, host, source string, known []model.Resource) []model.Dependency {
	var deps []model.Dependency
	for _, match := range matchKnown(host, known) {
		deps = append(deps, model.Dependency{
			From: from, To: match.id, Kind: model.KindWritesLogsTo,
			Source: source, Sources: []string{source}, Confidence: match.confidence,
		})
	}
	return deps
}
