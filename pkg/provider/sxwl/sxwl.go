package sxwl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

var _ Scheduler = &sxwl{}

type sxwl struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
	identity   string
}

// GetAssignedJobList implements Scheduler.
func (s *sxwl) GetAssignedJobList() ([]PortalJob, error) {
	urlStr, err := url.JoinPath(s.baseURL, "/api/userJob/cpod_jobs")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("cpodid", s.identity)
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Authorization", "Bearer "+s.accessKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var res []PortalJob
	if err = json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *sxwl) HeartBeat(payload HeartBeatPayload) error {
	urlStr, err := url.JoinPath(s.baseURL, "/api/userJob/cpod_status")
	if err != nil {
		return err
	}
	reqBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewBuffer(reqBytes))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", "Bearer "+s.accessKey)
	req.Header.Add("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload: %v ", resp.StatusCode)
	}
	return nil
}
