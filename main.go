package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var labels string
	var namespace string
	var follow bool
	var pod string

	flag.StringVar(&labels, "labels", "", "set labels for the selector")
	flag.StringVar(&namespace, "namespace", "default", "namespace name")
	flag.BoolVar(&follow, "follow", false, "follow logs from the pod or exit")
	flag.StringVar(&pod, "pod", "", "pod name for logs streaming")
	flag.Parse()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx := context.Background()
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-sigChan
		cancel()
	}()

	kubeconfigPath := os.Getenv("KUBECONFIG")
	home := homedir.HomeDir()
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	if pod == "" && labels == "" {
		log.Fatalln("No labels provided (required when --pod is omitted)")
		return
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatalf("Error reading logs: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Get pods list for given labels

	if pod == "" {
		var wg sync.WaitGroup

		podList, err := clientset.CoreV1().Pods(namespace).List(cancelCtx, v1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			log.Fatalf("Error listing pods: %v", err)
		}

		if len(podList.Items) == 0 {
			log.Fatalln("No pod found")
		}
		log.Printf("Found %d pods", len(podList.Items))

		for _, p := range podList.Items {
			wg.Add(1)
			go func(pod v2.Pod) {
				defer wg.Done()
				log.Printf("Starting stream for pod %s", pod.Name)
				if err = streamPodLogs(cancelCtx, clientset, pod.Namespace, pod.Name, follow); err != nil {
					log.Printf("Error streaming pod %s: %v", pod.Name, err)
				}
			}(p)
		}
		wg.Wait()

	} else {
		err = streamPodLogs(cancelCtx, clientset, namespace, pod, follow)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Print("exiting")
				return
			}
			log.Fatalf("Error reading logs: %v", err)
		}
	}
	if !follow {
		return
	}
	log.Println("Streaming logs, press Ctrl+C to exit")
	<-cancelCtx.Done()
	log.Println("Shutting down...")
}

func streamPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, pod string, follow bool) error {
	logsReq := clientset.CoreV1().Pods(namespace).GetLogs(pod, &v2.PodLogOptions{
		Follow: follow,
	})
	logsResp, err := logsReq.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream pod '%s': %w", pod, err)
	}
	defer logsResp.Close()

	_, err = io.Copy(os.Stdout, logsResp)

	return err
}
