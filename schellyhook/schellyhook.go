package schellyhook

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
)

//Options command line options
type Options struct {
	ListenPort        int
	ListenIP          string
	PrePostTimeout    int
	PreBackupCommand  string
	PostBackupCommand string
}

//SchellyResponse schelly webhook response
type SchellyResponse struct {
	ID      string  `json:"id",omitempty`
	DataID  string  `json:"data_id",omitempty`
	Status  string  `json:"status",omitempty`
	Message string  `json:"message",omitempty`
	SizeMB  float64 `json:"size_mb",omitempty`
}

//Backuper interface for who is implementing specific backup operations on backend
type Backuper interface {
	//Init register command line flags here etc
	Init() error
	//CreateNewBackup create a new backup synchronously (return only after complete backup creation). If you set shellContext.CmdRef when calling a Shell Script, the bridge will cancel the process automatically if a DELETE /backup/{id} for the running backup is received
	CreateNewBackup(apiID string, timeout time.Duration, shellContext *ShellContext) error
	//DeleteBackup remove backup data from storage. if backup is still running and set cmdRef on ShellContext of CreateBackup call, cancel it
	DeleteBackup(apiID string) error
	//GetAllBackups returns all tracked backups. this is optional for Schelly
	GetAllBackups() ([]SchellyResponse, error)
	//GetBackup returns a specific backup info. if requested apiID is running, this method is not even called, because schellyhook will do this for you
	GetBackup(apiID string) (*SchellyResponse, error)
}

var options = new(Options)
var currentBackupContext = ShellContext{}
var currentBackuper Backuper

//RunningBackupAPIID current apiID of the currently running backup, if any
var RunningBackupAPIID = ""

//CurrentBackupStartTime start time of currently running backup, if any
var CurrentBackupStartTime time.Time

//Initialize must be invoked to start REST server along with all Backuper hooks
func Initialize(backuper Backuper) {
	if currentBackuper != nil {
		logrus.Infof("Replacing previously existing 'backuper' instance in Schelly-Webhook")
	}
	currentBackuper.Init()
	currentBackuper = backuper
	listenPort := flag.Int("listen-port", 7070, "REST API server listen port")
	listenIP := flag.String("listen-ip", "0.0.0.0", "REST API server listen ip address")
	logLevel := flag.String("log-level", "info", "debug, info, warning or error")
	preBackupCommand := flag.String("pre-backup-command", "", "Command to be executed before running the backup")
	postBackupCommand := flag.String("post-backup-command", "", "Command to be executed after running the backup")
	prePostTimeout := flag.Int("pre-post-timeout", 7200, "Max time for pre or post command to be executing. After that time the process will be killed")
	flag.Parse()

	switch *logLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
		break
	case "warning":
		logrus.SetLevel(logrus.WarnLevel)
		break
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
		break
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	options.ListenPort = *listenPort
	options.ListenIP = *listenIP
	options.PrePostTimeout = *prePostTimeout
	options.PreBackupCommand = *preBackupCommand
	options.PostBackupCommand = *postBackupCommand

	router := mux.NewRouter()
	router.HandleFunc("/backups", getBackups).Methods("GET")
	router.HandleFunc("/backups", createBackup).Methods("POST")
	router.HandleFunc("/backups/{id}", getBackup).Methods("GET")
	router.HandleFunc("/backups/{id}", deleteBackup).Methods("DELETE")
	listen := fmt.Sprintf("%s:%d", options.ListenIP, options.ListenPort)
	logrus.Infof("Listening at %s", listen)
	err := http.ListenAndServe(listen, router)
	if err != nil {
		logrus.Errorf("Error while listening requests: %s", err)
		os.Exit(1)
	}
}

//GetBackups - get backups from Backuper
func getBackups(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("GetBackups r=%s", r)
	w.Header().Set("Content-Type", "application/json")
	gab, err := currentBackuper.GetAllBackups()
	if err != nil {
		logrus.Warnf("Error calling getAllBackups(). err=%s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(gab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//GetBackup - get specific backup from Backuper
func getBackup(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("GetBackup r=%s", r)
	params := mux.Vars(r)

	apiID := params["id"]

	if RunningBackupAPIID == apiID {
		sendSchellyResponse(apiID, "running", "backup is running", -1, http.StatusOK, w)
		return
	}

	resp, err := currentBackuper.GetBackup(apiID)
	if err != nil {
		logrus.Warnf("Error calling getBackup() for id %s. err=%s", apiID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if resp == nil {
		logrus.Debugf("Backup %s not found", apiID)
		http.Error(w, fmt.Sprintf("Backup %s not found", apiID), http.StatusNotFound)
		return
	}

	sendSchellyResponse(apiID, resp.Status, resp.Message, resp.SizeMB, http.StatusOK, w)
}

//CreateBackup - trigger new backup
func createBackup(w http.ResponseWriter, r *http.Request) {
	logrus.Infof(">>>>CreateBackup r=%s", r)

	if RunningBackupAPIID != "" {
		logrus.Infof("Another backup id %s is already running. Aborting.", RunningBackupAPIID)
		http.Error(w, fmt.Sprintf("Another backup id %s is already running. Aborting.", RunningBackupAPIID), http.StatusConflict)
		return
	}

	RunningBackupAPIID = createAPIID()
	CurrentBackupStartTime = time.Now()

	//run backup assyncronouslly
	go runBackup(RunningBackupAPIID)

	sendSchellyResponse(RunningBackupAPIID, "running", "backup triggered", -1, http.StatusAccepted, w)
}

//DeleteBackup - delete backup from Backuper
func deleteBackup(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("DeleteBackup r=%s", r)
	params := mux.Vars(r)

	apiID := params["id"]

	if RunningBackupAPIID == apiID {
		if currentBackupContext.CmdRef != nil {
			logrus.Debugf("Canceling currently running backup %s", RunningBackupAPIID)
			err := (*currentBackupContext.CmdRef).Stop()
			if err != nil {
				sendSchellyResponse(apiID, "running", "Couldn't cancel current running backup task. err="+err.Error(), -1, http.StatusInternalServerError, w)
			} else {
				sendSchellyResponse(apiID, "deleted", "Running backup task was cancelled successfuly", -1, http.StatusOK, w)
			}
		}
		return
	}

	err := currentBackuper.DeleteBackup(apiID)
	if err != nil {
		logrus.Warnf("Error calling deleteBackup() with id %s", apiID)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logrus.Debugf("Backup %s deleted", apiID)

	sendSchellyResponse(apiID, "deleted", "backup deleted successfuly", -1, http.StatusOK, w)
}

func sendSchellyResponse(id string, status string, message string, size float64, httpStatus int, w http.ResponseWriter) {
	resp := SchellyResponse{
		ID:      id,
		Status:  status,
		Message: message,
		SizeMB:  size,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		logrus.Errorf("Error encoding response. err=%s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		logrus.Debugf("SchellyResponse sent %s", resp)
	}
}

func runBackup(apiID string) {
	logrus.Debugf("Backup request arrived apiID=%s", RunningBackupAPIID)
	RunningBackupAPIID = apiID

	//process pre backup command before calling backup
	if options.PreBackupCommand != "" {
		logrus.Infof("Running pre-backup command '%s'", options.PreBackupCommand)
		out, err := ExecShellTimeout(options.PreBackupCommand, time.Duration(options.PrePostTimeout)*time.Second, &currentBackupContext)
		if err != nil {
			status := currentBackupContext.CmdRef.Status()
			if status.Exit == -1 {
				logrus.Warnf("Pre-backup command timeout enforced (%d seconds)", (status.StopTs-status.StartTs)/1000000000)
			}
			logrus.Debugf("Pre-backup command error. out=%s; err=%s", out, err.Error())
			RunningBackupAPIID = ""
			return
		} else {
			logrus.Debug("Pre-backup command success")
		}
	}

	//run backup
	logrus.Infof("Running backup")
	err := currentBackuper.CreateNewBackup(RunningBackupAPIID, time.Duration(options.PrePostTimeout)*time.Second, &currentBackupContext)
	if err != nil {
		status := currentBackupContext.CmdRef.Status()
		if status.Exit == -1 {
			logrus.Warnf("Backup command timeout enforced (%d seconds)", (status.StopTs-status.StartTs)/1000000000)
		}
		logrus.Debugf("Backup error. Will retry. err=%s", err.Error())
		RunningBackupAPIID = ""
		return
	} else {
		logrus.Debug("Backup creation success on Backuper. backup id %s", RunningBackupAPIID)
	}

	//process post backup command after finished
	if options.PostBackupCommand != "" {
		logrus.Infof("Running post-backup command '%s'", options.PostBackupCommand)
		out, err := ExecShellTimeout(options.PostBackupCommand, time.Duration(options.PrePostTimeout)*time.Second, &currentBackupContext)
		if err != nil {
			status := currentBackupContext.CmdRef.Status()
			if status.Exit == -1 {
				logrus.Warnf("Post-backup command timeout enforced (%d seconds)", (status.StopTs-status.StartTs)/1000000000)
			}
			logrus.Debugf("Post-backup command error. out=%s; err=%s", out, err.Error())
			RunningBackupAPIID = ""
			return
		} else {
			logrus.Debug("Post-backup command success")
		}
	}
	logrus.Infof("Backup finished")

	//now we can accept another POST /backups call...
	RunningBackupAPIID = ""
}

func createAPIID() string {
	uuid, _ := uuid.NewV4()
	return uuid.String()
}
