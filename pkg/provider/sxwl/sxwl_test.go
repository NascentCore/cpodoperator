package sxwl

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func Test_sxwl_GetAssignedJobList(t *testing.T) {
	baseURL, _ := os.LookupEnv("SXWL_BASE_URL")
	accessKey, _ := os.LookupEnv("SXWL_ACCESS_KEY")
	identity, _ := os.LookupEnv("SXWL_IDENTITY")

	if baseURL == "" {
		t.Skip("skip test")
	}

	s := &sxwl{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:   baseURL,
		accessKey: accessKey,
		identity:  identity,
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "ok",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.GetAssignedJobList()
			if (err != nil) != tt.wantErr {
				t.Errorf("sxwl.GetAssignedJobList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_sxwl_HeartBeat(t *testing.T) {
	baseURL, _ := os.LookupEnv("SXWL_BASE_URL")
	accessKey, _ := os.LookupEnv("SXWL_ACCESS_KEY")
	identity, _ := os.LookupEnv("SXWL_IDENTITY")

	if baseURL == "" {
		t.Skip("skip test")
	}

	s := &sxwl{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:   baseURL,
		accessKey: accessKey,
		identity:  identity,
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "ok",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.HeartBeat(HeartBeatPayload{})
			if (err != nil) != tt.wantErr {
				t.Errorf("sxwl.HeartBeat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
