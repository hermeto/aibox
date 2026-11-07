// Package kube wraps the client-go calls AIBox needs: server-side apply of unstructured
// objects and readiness checks for workloads and Argo CD Applications. It talks to the
// Kubernetes API directly (no kubectl or argocd binaries).
package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// FieldManager is the server-side apply field manager used by the CLI.
const FieldManager = "aibox"

// Well-known resources.
var (
	Applications = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	Deployments  = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	StatefulSets = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	Namespaces   = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	Pods         = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	Events       = schema.GroupVersionResource{Version: "v1", Resource: "events"}
	Jobs         = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	ConfigMaps   = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	Secrets      = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
)

// Client is a thin wrapper over the dynamic client.
type Client struct {
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
}

// NewForContext builds a client from the default kubeconfig loading rules and the given
// context name (e.g. "kind-aibox-local").
func NewForContext(kubeContext string) (*Client, error) { return NewForKubeconfig("", kubeContext) }

// NewForKubeconfig builds a client from an explicit kubeconfig file ("" = default loading
// rules) and context name.
func NewForKubeconfig(kubeconfig, kubeContext string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{CurrentContext: kubeContext}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig context %q: %w", kubeContext, err)
	}
	cfg.UserAgent = "aibox"
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	disc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Dynamic: dyn, Discovery: disc}, nil
}

// ServerVersion pings the API server.
func (c *Client) ServerVersion() (string, error) {
	v, err := c.Discovery.ServerVersion()
	if err != nil {
		return "", err
	}
	return v.GitVersion, nil
}

// Apply server-side applies obj (namespaced when obj has a namespace).
func (c *Client) Apply(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error {
	var ri dynamic.ResourceInterface = c.Dynamic.Resource(gvr)
	if ns := obj.GetNamespace(); ns != "" {
		ri = c.Dynamic.Resource(gvr).Namespace(ns)
	}
	_, err := ri.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{FieldManager: FieldManager, Force: true})
	if err != nil {
		return fmt.Errorf("apply %s %s/%s: %w", gvr.Resource, obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

// AppStatus summarizes an Argo CD Application.
type AppStatus struct {
	Name    string `json:"name"`
	Sync    string `json:"sync"`
	Health  string `json:"health"`
	Message string `json:"message,omitempty"`
}

// Ready reports whether the Application is Synced and Healthy.
func (a AppStatus) Ready() bool { return a.Sync == "Synced" && a.Health == "Healthy" }

func (a AppStatus) String() string {
	s := fmt.Sprintf("%s: sync=%s health=%s", a.Name, orUnknown(a.Sync), orUnknown(a.Health))
	if a.Message != "" {
		s += " (" + a.Message + ")"
	}
	return s
}

func orUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

// ApplicationStatus extracts sync/health status and the most useful message.
func ApplicationStatus(u *unstructured.Unstructured) AppStatus {
	st := AppStatus{Name: u.GetName()}
	st.Sync, _, _ = unstructured.NestedString(u.Object, "status", "sync", "status")
	st.Health, _, _ = unstructured.NestedString(u.Object, "status", "health", "status")
	var msgs []string
	if conds, ok, _ := unstructured.NestedSlice(u.Object, "status", "conditions"); ok {
		for _, c := range conds {
			if m, ok := c.(map[string]any); ok {
				if s, ok := m["message"].(string); ok && s != "" {
					msgs = append(msgs, s)
				}
			}
		}
	}
	if m, ok, _ := unstructured.NestedString(u.Object, "status", "operationState", "message"); ok && m != "" {
		msgs = append(msgs, m)
	}
	if m, ok, _ := unstructured.NestedString(u.Object, "status", "health", "message"); ok && m != "" {
		msgs = append(msgs, m)
	}
	st.Message = strings.Join(msgs, "; ")
	return st
}

// ListApplications returns the status of every Application in ns, sorted by name.
func (c *Client) ListApplications(ctx context.Context, ns string) ([]AppStatus, error) {
	list, err := c.Dynamic.Resource(Applications).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]AppStatus, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ApplicationStatus(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// WorkloadReady reports whether a Deployment or StatefulSet has rolled out completely.
func WorkloadReady(u *unstructured.Unstructured) bool {
	gen := u.GetGeneration()
	observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	replicas, found, _ := unstructured.NestedInt64(u.Object, "spec", "replicas")
	if !found {
		replicas = 1
	}
	updated, _, _ := unstructured.NestedInt64(u.Object, "status", "updatedReplicas")
	ready, _, _ := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
	available, hasAvailable, _ := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
	if !hasAvailable {
		available = ready
	}
	return observed >= gen && updated >= replicas && ready >= replicas && available >= replicas
}

// NotReadyWorkloads lists Deployments and StatefulSets in ns that have not rolled out.
// It returns the total number of workloads found as well.
func (c *Client) NotReadyWorkloads(ctx context.Context, ns string) (notReady []string, total int, err error) {
	for _, gvr := range []schema.GroupVersionResource{Deployments, StatefulSets} {
		list, err := c.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, 0, err
		}
		for i := range list.Items {
			total++
			if !WorkloadReady(&list.Items[i]) {
				notReady = append(notReady, strings.TrimSuffix(gvr.Resource, "s")+"/"+list.Items[i].GetName())
			}
		}
	}
	sort.Strings(notReady)
	return notReady, total, nil
}

// PodSummary lists pods in ns as "name phase (reason)" lines for diagnostics.
func (c *Client) PodSummary(ctx context.Context, ns string) ([]string, error) {
	list, err := c.Dynamic.Resource(Pods).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []string
	for i := range list.Items {
		p := &list.Items[i]
		phase, _, _ := unstructured.NestedString(p.Object, "status", "phase")
		line := fmt.Sprintf("%s/%s %s", ns, p.GetName(), phase)
		if sts, ok, _ := unstructured.NestedSlice(p.Object, "status", "containerStatuses"); ok {
			for _, s := range sts {
				m, _ := s.(map[string]any)
				if w, ok, _ := unstructured.NestedString(m, "state", "waiting", "reason"); ok {
					line += " waiting:" + w
				}
			}
		}
		out = append(out, line)
	}
	return out, nil
}

// JobState summarizes a batch/v1 Job.
type JobState struct {
	Found     bool
	Succeeded bool
	Failed    bool
	// Message is the termination message of the Job's most recent finished container
	// (scripts write their result there; on errors Kubernetes falls back to the log tail),
	// else the reason of the Failed condition.
	Message string
}

// JobStatusFrom derives a JobState from a Job and its pods.
func JobStatusFrom(job *unstructured.Unstructured, pods []unstructured.Unstructured) JobState {
	st := JobState{Found: true}
	conds, _, _ := unstructured.NestedSlice(job.Object, "status", "conditions")
	for _, c := range conds {
		m, _ := c.(map[string]any)
		if m["status"] != "True" {
			continue
		}
		switch m["type"] {
		case "Complete":
			st.Succeeded = true
		case "Failed":
			st.Failed = true
			if msg, _ := m["message"].(string); msg != "" {
				st.Message = msg
			}
		}
	}
	var latest string
	for i := range pods {
		sts, _, _ := unstructured.NestedSlice(pods[i].Object, "status", "containerStatuses")
		for _, s := range sts {
			m, _ := s.(map[string]any)
			msg, _, _ := unstructured.NestedString(m, "state", "terminated", "message")
			at, _, _ := unstructured.NestedString(m, "state", "terminated", "finishedAt")
			if msg = strings.TrimSpace(msg); msg != "" && at >= latest {
				latest, st.Message = at, msg
			}
		}
	}
	return st
}

// JobStatus reads a Job and the termination messages of its pods.
func (c *Client) JobStatus(ctx context.Context, ns, name string) (JobState, error) {
	job, err := c.Dynamic.Resource(Jobs).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return JobState{}, nil
	}
	if err != nil {
		return JobState{}, err
	}
	pods, err := c.Dynamic.Resource(Pods).Namespace(ns).List(ctx, metav1.ListOptions{LabelSelector: "batch.kubernetes.io/controller-uid=" + string(job.GetUID())})
	if err != nil {
		return JobState{}, err
	}
	return JobStatusFrom(job, pods.Items), nil
}

// DeleteJob deletes a Job and, in the background, its pods.
func (c *Client) DeleteJob(ctx context.Context, ns, name string) error {
	policy := metav1.DeletePropagationBackground
	err := c.Dynamic.Resource(Jobs).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
