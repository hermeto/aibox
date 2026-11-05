package kube

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
)

func obj(apiVersion, kind, name string, fields map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"apiVersion": apiVersion, "kind": kind,
		"metadata": map[string]any{"name": name, "namespace": "argocd", "generation": int64(1)}}}
	for k, v := range fields {
		u.Object[k] = v
	}
	return u
}

func app(name, sync, health, msg string) *unstructured.Unstructured {
	return obj("argoproj.io/v1alpha1", "Application", name, map[string]any{"status": map[string]any{
		"sync":       map[string]any{"status": sync},
		"health":     map[string]any{"status": health},
		"conditions": []any{map[string]any{"type": "ComparisonError", "message": msg}},
	}})
}

func deploy(name string, replicas, ready int64) *unstructured.Unstructured {
	return obj("apps/v1", "Deployment", name, map[string]any{
		"spec":   map[string]any{"replicas": replicas},
		"status": map[string]any{"observedGeneration": int64(1), "updatedReplicas": ready, "readyReplicas": ready, "availableReplicas": ready},
	})
}

func newFake(objs ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	gvrToList := map[schema.GroupVersionResource]string{
		Applications: "ApplicationList", Deployments: "DeploymentList", StatefulSets: "StatefulSetList", Pods: "PodList",
	}
	return &Client{Dynamic: dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToList, objs...)}
}

func TestListApplicationsAndStatus(t *testing.T) {
	c := newFake(app("serving", "OutOfSync", "Progressing", "repo not found"), app("aibox-root", "Synced", "Healthy", ""))
	apps, err := c.ListApplications(context.Background(), "argocd")
	if err != nil || len(apps) != 2 || apps[0].Name != "aibox-root" {
		t.Fatalf("%v %v", apps, err)
	}
	if !apps[0].Ready() || apps[1].Ready() {
		t.Fatal("ready evaluation")
	}
	if got := apps[1].String(); got != "serving: sync=OutOfSync health=Progressing (repo not found)" {
		t.Fatalf("String() = %q", got)
	}
	if (AppStatus{Name: "x"}).String() != "x: sync=Unknown health=Unknown" {
		t.Fatal("unknown rendering")
	}
}

func TestWorkloadReadiness(t *testing.T) {
	c := newFake(deploy("argocd-server", 1, 1), deploy("argocd-repo-server", 1, 0))
	notReady, total, err := c.NotReadyWorkloads(context.Background(), "argocd")
	if err != nil || total != 2 || len(notReady) != 1 || notReady[0] != "deployment/argocd-repo-server" {
		t.Fatalf("%v %d %v", notReady, total, err)
	}
	noSpec := obj("apps/v1", "StatefulSet", "x", map[string]any{"status": map[string]any{"observedGeneration": int64(1), "updatedReplicas": int64(1), "readyReplicas": int64(1)}})
	if !WorkloadReady(noSpec) {
		t.Fatal("default replicas=1 with ready=1 should be ready")
	}
}

func TestPodSummary(t *testing.T) {
	pod := obj("v1", "Pod", "p1", map[string]any{"status": map[string]any{
		"phase":             "Pending",
		"containerStatuses": []any{map[string]any{"state": map[string]any{"waiting": map[string]any{"reason": "ImagePullBackOff"}}}},
	}})
	lines, err := newFake(pod).PodSummary(context.Background(), "argocd")
	if err != nil || len(lines) != 1 || lines[0] != "argocd/p1 Pending waiting:ImagePullBackOff" {
		t.Fatalf("%v %v", lines, err)
	}
}

func TestJobStatusFrom(t *testing.T) {
	job := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Failed", "status": "True", "message": "BackoffLimitExceeded"},
		map[string]any{"type": "Complete", "status": "False"},
	}}}}
	pod := func(at, msg string) unstructured.Unstructured {
		return unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"containerStatuses": []any{
			map[string]any{"state": map[string]any{"terminated": map[string]any{"finishedAt": at, "message": msg}}},
		}}}}
	}
	st := JobStatusFrom(job, nil)
	if !st.Found || !st.Failed || st.Succeeded || st.Message != "BackoffLimitExceeded" {
		t.Fatalf("%+v", st)
	}
	st = JobStatusFrom(job, []unstructured.Unstructured{pod("2026-11-05T10:00:00Z", "first try"), pod("2026-11-05T10:05:00Z", "gated: accept the license\n")})
	if st.Message != "gated: accept the license" {
		t.Fatalf("latest termination message wins: %+v", st)
	}
	done := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Complete", "status": "True"},
	}}}}
	if st := JobStatusFrom(done, []unstructured.Unstructured{pod("2026-11-05T10:00:00Z", "synced: 3 files")}); !st.Succeeded || st.Message != "synced: 3 files" {
		t.Fatalf("%+v", st)
	}
}
