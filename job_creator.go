package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type JobCreator struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewJobCreator(namespace string) (*JobCreator, error) {
	log.Println("NewJobCreator called")
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	log.Println("Creating Kubernetes client")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	log.Println("Returning JobCreator")
	return &JobCreator{
		clientset: clientset,
		namespace: namespace,
	}, nil
}

func (jc *JobCreator) CreateRestoreJob(params RestoreParams) error {
	jobName := fmt.Sprintf("restore-job-%d-%d", params.ProjectId, time.Now().Unix())
	log.Printf("Creating restore job: %s", jobName)

	// Check if a job with this name already exists
	_, err := jc.clientset.BatchV1().Jobs(jc.namespace).Get(context.Background(), jobName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("job %s already exists", jobName)
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal restore params: %w", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "restore-worker",
							Image: "guardianmultimedia/pluto-project-restore-worker:DEV", // Update this with your actual image
							Env: []corev1.EnvVar{
								{
									Name:  "RESTORE_PARAMS",
									Value: string(paramsJSON),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	log.Printf("Creating job: %s", job.Name)
	createdJob, err := jc.clientset.BatchV1().Jobs(jc.namespace).Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error creating job: %v", err)
		return fmt.Errorf("failed to create job: %w", err)
	}

	if createdJob == nil {
		log.Printf("Created job is nil, but no error was returned")
		return fmt.Errorf("created job is nil")
	}

	log.Printf("Job created successfully: %s", createdJob.Name)
	log.Printf("Job UID: %s", createdJob.UID)
	log.Printf("Job Status: %+v", createdJob.Status)

	return nil
}
