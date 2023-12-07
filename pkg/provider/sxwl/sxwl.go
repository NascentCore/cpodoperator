package sxwl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

var _ Scheduler = &sxwl{}

type sxwl struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
}

// GetAssignedTaskList implements Scheduler.
func (s *sxwl) GetAssignedTaskList() error {
	panic("unimplemented")
}

// upload cpod info to marketmanager.
// func UploadCPodStatus(up UploadPayload) bool {
// 	bytesData, err := json.Marshal(up)
// 	if err != nil {
// 		log.SLogger.Errorw("data error", "error", err)
// 		return false
// 	}
// 	req, err := http.NewRequest(http.MethodPost, config.BASE_URL+config.URLPATH_UPLOAD_CPOD_STATUS, bytes.NewBuffer(bytesData))
// 	if err != nil {
// 		log.SLogger.Errorw("build request error", "error", err)
// 		return false
// 	}
// 	req.Header.Add("Authorization", "Bearer "+config.ACCESS_KEY)
// 	req.Header.Add("Content-Type", "application/json")
// 	resp, err := http.DefaultClient.Do(req)
// 	//resp, err := http.Post(config.BASE_URL+config.URLPATH_UPLOAD_CPOD_STATUS, "application/json", bytes.NewReader(bytesData))
// 	if err != nil {
// 		log.SLogger.Errorw("upload status err", "error", err)
// 		return false
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode != 200 {
// 		respData, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			log.SLogger.Errorw("statuscode != 200 and read body err", "code", resp.StatusCode, "error", err)
// 		} else {
// 			log.SLogger.Warnw("statuscode != 200", "code", resp.StatusCode, "body", string(respData))
// 		}
// 		return false
// 	}
// 	return true
// }

// TaskCallBack upload cpodjob status
func (s *sxwl) TaskCallBack(states []State) error {
	urlStr, err := url.JoinPath(s.baseURL, "/api/userJob/cpod_status")
	if err != nil {
		return err
	}
	reqBytes, _ := json.Marshal(states)
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
