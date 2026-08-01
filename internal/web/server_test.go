package web

import "testing"

func TestRecompressJobLifecycle(t *testing.T) {
	s := &Server{recompressJobs: make(map[string]*recompressJob)}

	jobID := s.createRecompressJob("/tmp/sample.zip")
	if jobID == "" {
		t.Fatal("expected a non-empty job id")
	}

	job := s.getRecompressJob(jobID)
	if job == nil {
		t.Fatal("expected job to exist")
	}
	if job.Status != "queued" {
		t.Fatalf("expected queued status, got %s", job.Status)
	}

	s.updateRecompressJob(jobID, "running", 25, "Preparing recompression")
	job = s.getRecompressJob(jobID)
	if job == nil {
		t.Fatal("expected job after update")
	}
	if job.Percent != 25 {
		t.Fatalf("expected percent 25, got %d", job.Percent)
	}
	if job.Message != "Preparing recompression" {
		t.Fatalf("unexpected message: %s", job.Message)
	}

	s.completeRecompressJob(jobID, 100, "Finished", "", 42)
	job = s.getRecompressJob(jobID)
	if job == nil {
		t.Fatal("expected job after completion")
	}
	if job.Status != "completed" {
		t.Fatalf("expected completed status, got %s", job.Status)
	}
	if job.Percent != 100 {
		t.Fatalf("expected percent 100, got %d", job.Percent)
	}
}
