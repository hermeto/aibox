package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/hermeto/aibox/internal/kube"
)

// Sync statuses.
const (
	StatusSynced  = "synced"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
)

// JobAPI is the part of the Kubernetes API a sync needs (kube.Client implements it).
type JobAPI interface {
	Apply(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error
	JobStatus(ctx context.Context, ns, name string) (kube.JobState, error)
	DeleteJob(ctx context.Context, ns, name string) error
}

// Syncer mirrors models into a target bucket with in-cluster Jobs.
type Syncer struct {
	API    JobAPI
	Target Target
	// Verify runs the load-check Job after a mirror.
	Verify bool
	// Timeout bounds each Job (default 30m).
	Timeout time.Duration
	// PollInterval between Job status reads (default 5s).
	PollInterval time.Duration
	Log          io.Writer
}

// Result is the outcome for one model.
type Result struct {
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
	Message  string `json:"message,omitempty"`
}

// Report is the output of `aibox models sync`.
type Report struct {
	Profile  string   `json:"profile"`
	Bucket   string   `json:"bucket"`
	Endpoint string   `json:"endpoint,omitempty"`
	Models   []Result `json:"models"`
}

// Failed lists the ids of models that failed.
func (r *Report) Failed() []string {
	var ids []string
	for _, m := range r.Models {
		if m.Status == StatusFailed {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

// RenderText prints one line per model.
func (r *Report) RenderText(w io.Writer) error {
	where := "s3://" + r.Bucket
	if r.Endpoint != "" {
		where += " (" + r.Endpoint + ")"
	}
	if _, err := fmt.Fprintf(w, "Model sync for profile %s → %s\n", r.Profile, where); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "MODEL\tSTATUS\tVERIFIED\tMESSAGE")
	for _, m := range r.Models {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%t\t%s\n", m.ID, m.Status, m.Verified, m.Message)
	}
	return tw.Flush()
}

func (s *Syncer) logf(format string, args ...any) {
	if s.Log != nil {
		_, _ = fmt.Fprintf(s.Log, format+"\n", args...)
	}
}

func (s *Syncer) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return 30 * time.Minute
}

func (s *Syncer) poll() time.Duration {
	if s.PollInterval > 0 {
		return s.PollInterval
	}
	return 5 * time.Second
}

// Sync mirrors (and verifies) each model in order and reports per-model results.
func (s *Syncer) Sync(ctx context.Context, models []Model) *Report {
	rep := &Report{Bucket: s.Target.Bucket, Endpoint: s.Target.Endpoint}
	cm, err := Scripts()
	if err == nil {
		var obj *unstructured.Unstructured
		if obj, err = ToUnstructured(cm); err == nil {
			err = s.API.Apply(ctx, kube.ConfigMaps, obj)
		}
	}
	for i := range models {
		m := &models[i]
		if err != nil {
			rep.Models = append(rep.Models, Result{ID: m.ID, Revision: m.Source.Revision, Status: StatusFailed, Message: "sync scripts: " + err.Error()})
			continue
		}
		rep.Models = append(rep.Models, s.one(ctx, m))
	}
	return rep
}

func (s *Syncer) one(ctx context.Context, m *Model) Result {
	r := Result{ID: m.ID, Revision: m.Source.Revision}
	switch {
	case !m.Mirrorable():
		r.Status, r.Message = StatusSkipped, "no weights to mirror ("+m.Source.Type+" source)"
		return r
	case !m.Pinned():
		r.Status, r.Message = StatusFailed, "revision not pinned: run `go run ./hack/hfcheck` and pin the commit SHA it prints"
		return r
	}
	s.logf("Mirroring %s (%s@%s) to s3://%s/%s", m.ID, m.Source.Repo, m.Source.Revision, s.Target.Bucket, m.Prefix())
	msg, err := s.run(ctx, MirrorJob(m, s.Target, s.timeout()))
	if err != nil {
		r.Status, r.Message = StatusFailed, withHint(m, err.Error())
		return r
	}
	r.Status, r.Message = StatusSynced, msg
	if strings.HasPrefix(msg, "skipped") {
		r.Status = StatusSkipped
	}
	if !s.Verify {
		return r
	}
	s.logf("Verifying %s from the bucket", m.ID)
	vmsg, err := s.run(ctx, VerifyJob(m, s.Target, s.timeout()))
	if err != nil {
		r.Status, r.Message = StatusFailed, "load check: "+err.Error()
		return r
	}
	r.Verified = true
	r.Message += "; " + vmsg
	return r
}

// withHint explains how to unblock gated models.
func withHint(m *Model, msg string) string {
	if m.Gated || strings.Contains(msg, "gated") {
		return msg + " — accept the license at https://huggingface.co/" + m.Source.Repo +
			" and store a Hugging Face token (docs/runbooks/model-weights.md)"
	}
	return msg
}

// run replaces any earlier Job of the same name, applies job and waits until it finishes,
// returning its termination message.
func (s *Syncer) run(ctx context.Context, job *batchv1.Job) (string, error) {
	ns, name := job.Namespace, job.Name
	st, err := s.API.JobStatus(ctx, ns, name)
	if err != nil {
		return "", err
	}
	if st.Found {
		if err := s.API.DeleteJob(ctx, ns, name); err != nil {
			return "", err
		}
		if err := s.wait(ctx, ns, name, func(st kube.JobState) bool { return !st.Found }); err != nil {
			return "", fmt.Errorf("previous job %s/%s not deleted: %w", ns, name, err)
		}
	}
	obj, err := ToUnstructured(job)
	if err != nil {
		return "", err
	}
	if err := s.API.Apply(ctx, kube.Jobs, obj); err != nil {
		return "", err
	}
	var final kube.JobState
	err = s.wait(ctx, ns, name, func(st kube.JobState) bool {
		final = st
		return st.Succeeded || st.Failed
	})
	if err != nil {
		return "", fmt.Errorf("job %s/%s: %w (kubectl -n %s logs job/%s)", ns, name, err, ns, name)
	}
	if final.Failed {
		msg := final.Message
		if msg == "" {
			msg = "job failed"
		}
		return "", fmt.Errorf("%s (kubectl -n %s logs job/%s)", msg, ns, name)
	}
	return final.Message, nil
}

var errTimeout = errors.New("timed out")

func (s *Syncer) wait(ctx context.Context, ns, name string, done func(kube.JobState) bool) error {
	deadline := time.Now().Add(s.timeout() + time.Minute)
	for {
		st, err := s.API.JobStatus(ctx, ns, name)
		if err != nil {
			return err
		}
		if done(st) {
			return nil
		}
		if time.Now().After(deadline) {
			return errTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.poll()):
		}
	}
}
