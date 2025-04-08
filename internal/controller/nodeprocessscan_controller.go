/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	// "github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/zensey/nps/api/v1"
)

type NodeProcessScanReconciler struct {
	client.Client
	// Log       logr.Logger
	Scheme    *runtime.Scheme
	ClientSet *kubernetes.Clientset
	Config    *rest.Config
}

// +kubebuilder:rbac:groups=monitoring.example.com,resources=nodeprocessscans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.example.com,resources=nodeprocessscans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.example.com,resources=nodeprocessscans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NodeProcessScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile

func (r *NodeProcessScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("NodeProcessScanReconciler Reconcile!")

	// Fetch the custom resource
	var scan monitoringv1.NodeProcessScan

	if err := r.Get(ctx, req.NamespacedName, &scan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if scan.Status.Results == nil {
		scan.Status.Results = make(map[string][]monitoringv1.ProcessDetection)
	}

	// Get all pods in all namespaces
	var pods corev1.PodList
	if err := r.List(ctx, &pods); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list pods: %v", err)
	}

	// Scan each pod's containers
	scan.Status.ScanningNodes = 0
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.Namespace != "default" {
			continue
		}

		for _, container := range pod.Spec.Containers {
			// Execute "ps aux" in the container
			output, err := r.execInContainer(ctx, &pod, container.Name, []string{"ps", "aux"})
			if err != nil {
				log.Info("Failed to exec in container",
					"pod", pod.Name,
					"container", container.Name,
					"err", err.Error(),
				)
				continue
			}

			// Parse and store results
			detections := r.parseProcessOutput(output, scan.Spec.ProcessNames)
			if len(detections) > 0 {
				key := fmt.Sprintf("%s/%s/%s", pod.Spec.NodeName, pod.Name, container.Name)
				scan.Status.Results[key] = detections
			}
		}
		scan.Status.ScanningNodes++
	}

	scan.Status.LastScanTime = metav1.Now()
	if err := r.Status().Update(ctx, &scan); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %v", err)
	}

	// Requeue after specified interval
	interval := time.Duration(scan.Spec.Interval) * time.Second
	if interval == 0 {
		interval = 30 * time.Second // Default
	}
	return ctrl.Result{RequeueAfter: interval}, nil
}

// execInContainer executes a command in a container using Kubernetes API
func (r *NodeProcessScanReconciler) execInContainer(
	ctx context.Context,
	pod *corev1.Pod,
	containerName string,
	command []string,
) (string, error) {
	req := r.ClientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("exec").
		Param("container", containerName).
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, runtime.NewParameterCodec(r.Scheme))

	executor, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	err = executor.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", fmt.Errorf("exec failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// parseProcessOutput finds target processes in ps aux output
func (r *NodeProcessScanReconciler) parseProcessOutput(
	output string,
	targets []string,
) []monitoringv1.ProcessDetection {
	var detections []monitoringv1.ProcessDetection
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		for _, target := range targets {
			if strings.Contains(line, target) {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					pid, _ := strconv.Atoi(fields[1])
					detections = append(detections, monitoringv1.ProcessDetection{
						ProcessName: target,
						PID:         pid,
						Command:     strings.Join(fields[10:], " "),
					})
				}
			}
		}
	}
	return detections
}

func (r *NodeProcessScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize Kubernetes clientset
	config := mgr.GetConfig()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	r.ClientSet = clientset
	r.Config = config

	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.NodeProcessScan{}).
		Complete(r)
}
