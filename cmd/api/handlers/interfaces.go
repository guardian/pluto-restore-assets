package handlers

import "pluto-restore-assets/internal/types"

type JobCreator interface {
	CreateRestoreJob(params types.RestoreParams) error
	GetJobLogs(jobName string) (string, error)
}
