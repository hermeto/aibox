package models

import (
	"embed"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// Names shared with the platform (platform/models, platform/secrets) and the Pod Identity
// association of the 004 IAM module.
const (
	Namespace        = "aibox-models"
	ServiceAccount   = "model-sync"
	TokenSecret      = "hf-token" // ExternalSecret target; key "token"
	ScriptsConfigMap = "model-sync-scripts"
	// Image runs scripts/mirror.py; its dependencies are pip-installed from the
	// hash-locked scripts/requirements.txt (no custom image).
	Image = "python:3.14.10-slim@sha256:930e1abba7c4e8f96ed93e8012c35d174d7732752731bbd5e8e757d7ec6f3966"
)

// Job kinds.
const (
	KindMirror = "mirror"
	KindVerify = "verify"
)

//go:embed scripts/mirror.py scripts/requirements.txt
var scripts embed.FS

// Target is the object storage a sync writes to.
type Target struct {
	Bucket   string `json:"bucket"`
	Endpoint string `json:"endpoint,omitempty"` // empty means AWS S3
	Region   string `json:"region"`
	// Anonymous sends unsigned requests (the local store has no identities); on AWS the
	// Jobs get credentials from EKS Pod Identity.
	Anonymous bool `json:"-"`
	// CreateBucket creates a missing bucket (local store; AWS buckets come from Terraform).
	CreateBucket bool `json:"-"`
}

// LocalTarget is the in-cluster SeaweedFS store of the local profile (platform/models).
func LocalTarget() Target {
	return Target{Bucket: "aibox-weights", Endpoint: "http://seaweedfs." + Namespace + ".svc:8333", Region: "us-east-1", Anonymous: true, CreateBucket: true}
}

// JobName is the name of a model's mirror or verify Job (dots are not valid in pod
// hostnames, so they become dashes).
func JobName(kind, modelID string) string { return kind + "-" + strings.ReplaceAll(modelID, ".", "-") }

// Scripts returns the ConfigMap that carries mirror.py and its locked requirements.
func Scripts() (*corev1.ConfigMap, error) {
	data := map[string]string{}
	for _, name := range []string{"mirror.py", "requirements.txt"} {
		b, err := scripts.ReadFile("scripts/" + name)
		if err != nil {
			return nil, err
		}
		data[name] = string(b)
	}
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: ScriptsConfigMap, Namespace: Namespace, Labels: labels("scripts", "")},
		Data:       data,
	}, nil
}

func labels(component, modelID string) map[string]string {
	l := map[string]string{
		"app.kubernetes.io/name":      "model-sync",
		"app.kubernetes.io/component": component,
		"app.kubernetes.io/part-of":   "aibox",
	}
	if modelID != "" {
		l["aibox.dev/model"] = modelID
	}
	return l
}

// MirrorJob copies the model's revision from Hugging Face to the target bucket.
func MirrorJob(m *Model, t Target, timeout time.Duration) *batchv1.Job {
	return job(KindMirror, m, t, timeout)
}

// VerifyJob streams the mirrored objects and checks them against SHA256SUMS: the proof
// that a pod can load the model from the bucket.
func VerifyJob(m *Model, t Target, timeout time.Duration) *batchv1.Job {
	return job(KindVerify, m, t, timeout)
}

func job(kind string, m *Model, t Target, timeout time.Duration) *batchv1.Job {
	env := []corev1.EnvVar{
		{Name: "BUCKET", Value: t.Bucket},
		{Name: "PREFIX", Value: m.Prefix()},
		{Name: "HF_REPO", Value: m.Source.Repo},
		{Name: "REVISION", Value: m.Source.Revision},
		{Name: "AWS_REGION", Value: t.Region},
		{Name: "WORK_DIR", Value: "/work"},
		{Name: "HOME", Value: "/work/home"},
		{Name: "HF_HOME", Value: "/work/hf"},
		{Name: "TMPDIR", Value: "/work/tmp"},
		{Name: "HF_HUB_DISABLE_PROGRESS_BARS", Value: "1"},
		{Name: "PYTHONDONTWRITEBYTECODE", Value: "1"},
		{Name: "PIP_DISABLE_PIP_VERSION_CHECK", Value: "1"},
		{Name: "PIP_NO_CACHE_DIR", Value: "1"},
	}
	if t.Endpoint != "" {
		env = append(env, corev1.EnvVar{Name: "S3_ENDPOINT", Value: t.Endpoint})
	}
	if t.Anonymous {
		env = append(env, corev1.EnvVar{Name: "S3_ANONYMOUS", Value: "true"})
	}
	backoff := int32(0)
	if kind == KindMirror {
		backoff = 2 // a retry resumes: uploaded files are not uploaded again
		if t.CreateBucket {
			env = append(env, corev1.EnvVar{Name: "CREATE_BUCKET", Value: "true"})
		}
		env = append(env, corev1.EnvVar{Name: "HF_TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: TokenSecret}, Key: "token", Optional: ptr.To(true),
		}}})
	}
	script := "set -eu\n" +
		"mkdir -p \"$HOME\" \"$HF_HOME\" \"$TMPDIR\"\n" +
		"python -m pip install --quiet --require-hashes --only-binary=:all: --target /work/py -r /scripts/requirements.txt\n" +
		"PYTHONPATH=/work/py exec python /scripts/mirror.py " + kind + "\n"
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(kind, m.ID),
			Namespace: Namespace,
			Labels:    labels(kind, m.ID),
			Annotations: map[string]string{
				"aibox.dev/revision": m.Source.Revision,
				"aibox.dev/source":   "huggingface.co/" + m.Source.Repo,
				"aibox.dev/target":   "s3://" + t.Bucket + "/" + m.Prefix(),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(backoff),
			ActiveDeadlineSeconds:   ptr.To(int64(timeout / time.Second)),
			TTLSecondsAfterFinished: ptr.To(int32(86400)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels(kind, m.ID)},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.To(true),
						RunAsUser:      ptr.To(int64(65532)),
						RunAsGroup:     ptr.To(int64(65532)),
						FSGroup:        ptr.To(int64(65532)),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					Containers: []corev1.Container{{
						Name:                     kind,
						Image:                    Image,
						Command:                  []string{"/bin/sh", "-c", script},
						Env:                      env,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:              resource.MustParse("250m"),
								corev1.ResourceMemory:           resource.MustParse("512Mi"),
								corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
							},
							Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							ReadOnlyRootFilesystem:   ptr.To(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "work", MountPath: "/work"},
							{Name: "scripts", MountPath: "/scripts", ReadOnly: true},
						},
					}},
					Volumes: []corev1.Volume{
						// One file at a time: the largest shard plus the Python packages.
						{Name: "work", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: ptr.To(resource.MustParse("32Gi"))}}},
						{Name: "scripts", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: ScriptsConfigMap},
						}}},
					},
				},
			},
		},
	}
}

// ToUnstructured converts a typed object for server-side apply (without the empty status).
func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	delete(m, "status")
	return &unstructured.Unstructured{Object: m}, nil
}
