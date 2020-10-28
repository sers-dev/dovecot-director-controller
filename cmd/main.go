package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"time"
)

// variables: namespace, labels
var dovecotLabels string
var dovecotDirectorLabels string

var namespace string
var kubeconf *rest.Config

var initialDovecotPodCount int

func main() {
	clientset, err := InClusterAuth()

	if clientset == nil {
		clientset, err = OutOfClusterAuth()
	}
	if err != nil {
		panic(err.Error())
	}
	dovecotDirectorLabels = os.Getenv("DOVECOT_DIRECTOR_LABELS")
	dovecotLabels = os.Getenv("DOVECOT_LABELS")
	namespace = os.Getenv("DOVECOT_NAMESPACE")

	dovecotPods := GetPodsByLabel(clientset, namespace, dovecotLabels)
	initialDovecotPodCount = len(dovecotPods.Items)

	StartWatcher(clientset, namespace)
}

func GetPodsByLabel(clientset *kubernetes.Clientset, namespace string, labels string) *v1.PodList {
	listOptions := metav1.ListOptions{
		LabelSelector: labels,
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		panic(err.Error())
	}

	return pods
}

func ExecuteCommand(command string, podname string, namespace string, clientset *kubernetes.Clientset) error {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podname).Namespace(namespace).SubResource("exec")
	// THE FOLLOWING EXPECTS THE POD TO HAVE ONLY ONE CONTAINER IN WHICH THE COMMAND IS GOING TO BE EXECUTED
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(kubeconf, "POST", req.URL())
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    true,
	})
	if err != nil {
		return err
	}

	return nil
}

func handleEvent(pod *v1.Pod, clientset *kubernetes.Clientset) {
	if initialDovecotPodCount > 1 {
		initialDovecotPodCount--
		return
	}

	switch pod.Status.Phase {
	case v1.PodFailed, v1.PodSucceeded:
	case v1.PodRunning:
		containerStatusSlice := pod.Status.ContainerStatuses

		for _, containerStatus := range containerStatusSlice {

			if containerStatus.Ready {
				podlist := GetPodsByLabel(clientset, namespace, dovecotDirectorLabels)

				for _, dovecotDirectorPod := range podlist.Items {
                    time := time.Now()
                    logLevel := "info"
                    logMessage := "success"
                    formattedTime := time.Format("2006-01-02 15:04:05 MST")

					err := ExecuteCommand(
						"doveadm reload",
						dovecotDirectorPod.ObjectMeta.Name,
						namespace,
						clientset)

					if err != nil {
					    logLevel = "error"
					    logMessage = err.Error()
					}

					log := fmt.Sprintf("{ \"level\": \"%s\", \"timestamp\": \"%s\", \"pod\": \"%s\", \"command\": \"doveadm reload\", \"message\": \"%s\" }", logLevel, formattedTime, dovecotDirectorPod.ObjectMeta.Name, logMessage)
					fmt.Println(log)
				}
			}
		}
	}

}

func StartWatcher(clientset *kubernetes.Clientset, namespace string) () {
	optionsModifierFunc := func(options *metav1.ListOptions) {
		options.LabelSelector = dovecotLabels
	}
	watchlist := cache.NewFilteredListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"pods",
		namespace,
		optionsModifierFunc)

	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				handleEvent(pod, clientset)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				pod := newObj.(*v1.Pod)
				handleEvent(pod, clientset)
			},
		},
	)

	go controller.Run(make(chan struct{}))
	for {
		time.Sleep(time.Second)
	}
}


func InClusterAuth() (*kubernetes.Clientset, error) {
	var err error
	kubeconf, err = rest.InClusterConfig()

	if err != nil {
		return nil, nil
	}

	clientset, err := kubernetes.NewForConfig(kubeconf)
	if err != nil {
		panic(err.Error())
	}

	return clientset, nil
}

func OutOfClusterAuth() (*kubernetes.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("c", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("c", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	var err error
	kubeconf, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(kubeconf)
	if err != nil {
		panic(err.Error())
	}

	return clientset, nil
}
