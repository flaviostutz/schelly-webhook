package main

import (
	"fmt"
	"time"

	schellyhook "github.com/flaviostutz/schelly-webhook"
	"github.com/sirupsen/logrus"
)

//SampleBackuper sample backuper
type SampleBackuper struct {
}

var backups = []string{}
var runningID = ""

func main() {
	logrus.Infof("Starting Sample in-memory Schelly Webhook")
	backups = make([]string, 0)
	sampleBackuper := SampleBackuper{}
	schellyhook.Initialize(sampleBackuper)
}

func (sb SampleBackuper) Init() error {
	return nil
}

func (sb SampleBackuper) RegisterFlags() error {
	return nil
}

func (sb SampleBackuper) CreateNewBackup(apiID string, timeout time.Duration, shellContext *schellyhook.ShellContext) error {
	logrus.Infof("createBackup(): Triggering fake backup. Will delay 20s...")
	backups = append(backups, apiID)
	runningID = apiID
	time.Sleep(20 * time.Second)
	runningID = ""
	return nil
}

func (sb SampleBackuper) GetAllBackups() ([]schellyhook.SchellyResponse, error) {
	results := make([]schellyhook.SchellyResponse, 0)
	for _, apiID := range backups {
		status := "available"
		dataID := apiID + "123"
		if runningID == apiID {
			status = "running"
			dataID = apiID
		}
		sr := schellyhook.SchellyResponse{
			ID:      apiID,
			DataID:  dataID,
			Status:  status,
			Message: "",
			SizeMB:  100,
		}
		results = append(results, sr)
	}
	return results, nil
}

func (sb SampleBackuper) GetBackup(apiID string) (*schellyhook.SchellyResponse, error) {
	status := "available"
	dataID := apiID + "123"
	if runningID == apiID {
		status = "running"
		dataID = apiID
	}
	if contains(backups, apiID) {
		return &schellyhook.SchellyResponse{
			ID:      apiID,
			DataID:  dataID,
			Status:  status,
			Message: "",
			SizeMB:  100,
		}, nil
	} else {
		return nil, nil
	}
}

func (sb SampleBackuper) DeleteBackup(apiID string) error {
	logrus.Infof("deleteBackup(): Deleting backup %s...", apiID)
	cl := len(backups)
	backups = remove(backups, apiID)
	if cl == len(backups) {
		//backup not found
		return fmt.Errorf("Backup %s not found", apiID)
	} else {
		return nil
	}
}

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

func remove(l []string, item string) []string {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}
