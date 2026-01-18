package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	v2 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//pods, err := clientset.CoreV1().Pods(namespace).List(cancelCtx, v1.ListOptions{
	//	LabelSelector: labels,
	//})
	//if err != nil {
	//	panic(err.Error())
	//}
	//if len(pods.Items) == 0 {
	//	log.Println("no pods found")
	//	return
	//}
	logsReq := clientset.CoreV1().Pods(namespace).GetLogs(pod, &v2.PodLogOptions{
		Follow: follow,
	})
	logsResp, err := logsReq.Stream(cancelCtx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Fatalln("pod not found")
		}
		if apierrors.IsBadRequest(err) {
			log.Fatalf("Invalid request: %v", err)
		}
		log.Fatalln(err.Error())
	}
	defer logsResp.Close()

	_, err = io.Copy(os.Stdout, logsResp)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Print("exiting")
			return
		}
		panic(err.Error())
	}
}
