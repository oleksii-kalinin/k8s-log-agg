package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var labels string
	var namespace string

	flag.StringVar(&labels, "labels", "", "set labels for the selector.")
	flag.StringVar(&namespace, "namespace", "default", "namespace")
	flag.Parse()

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

	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), v1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		panic(err.Error())
	}
	if len(pods.Items) == 0 {
		log.Println("no pods found")
	}
	for _, v := range pods.Items {
		log.Printf("pod: %s, namespace: %s, labels: %s", v.Name, v.Namespace, v.Labels)
	}
}
