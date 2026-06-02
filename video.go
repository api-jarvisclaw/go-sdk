package jarvisclaw

import (
	"context"
	"fmt"
)

// VideoGenerate submits a video generation job and returns a VideoJob with the initial status.
func (c *Client) VideoGenerate(ctx context.Context, model string, req *VideoRequest) (*VideoJob, error) {
	payload := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
	}
	if req.Duration > 0 {
		payload["duration"] = req.Duration
	}

	raw, err := c.doPost("/v1/video/generations", payload)
	if err != nil {
		return nil, err
	}

	return videoJobFromRaw(raw)
}

// VideoStatus checks the status of a video generation job by job ID.
func (c *Client) VideoStatus(ctx context.Context, jobID string) (*VideoJob, error) {
	raw, err := c.doGet("/v1/video/generations/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	return videoJobFromRaw(raw)
}

func videoJobFromRaw(raw map[string]any) (*VideoJob, error) {
	job := &VideoJob{}
	if v, ok := raw["id"].(string); ok {
		job.ID = v
	}
	if v, ok := raw["status"].(string); ok {
		job.Status = v
	}
	if v, ok := raw["url"].(string); ok {
		job.URL = v
	}
	if job.ID == "" {
		return nil, fmt.Errorf("unexpected video job response format")
	}
	return job, nil
}
