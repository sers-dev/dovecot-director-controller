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
	"strconv"
	"time"
)

var initialDovecotPodCount int

type EnvVariables struct {
	DovecotLabels                string
	DovecotDirectorLabels        string
	DovecotDirectorContainerName string
	Namespace                    string
	SyncFrequencyDuration        int
}

type K3sVars struct {
	Kubeconf  *rest.Config
	Clientset *kubernetes.Clientset
}

func main() {
	clientset, kubeconfig, err := InClusterAuth()

	if clientset == nil {
		clientset, kubeconfig, err = OutOfClusterAuth()
	}
	if err != nil {
		panic(err.Error())
	}

	syncFrequencyDurationEnv := os.Getenv("SYNC_FREQUENCY_DURATION")

	syncFrequencyDuration := 70
	if syncFrequencyDurationEnv != "" {
		syncFrequencyDuration, err = strconv.Atoi(syncFrequencyDurationEnv)
		if err != nil {
			syncFrequencyDuration = 70
		}
	}
	envVariables := EnvVariables{
		DovecotLabels:                os.Getenv("DOVECOT_LABELS"),
		DovecotDirectorLabels:        os.Getenv("DOVECOT_DIRECTOR_LABELS"),
		DovecotDirectorContainerName: os.Getenv("DOVECOT_DIRECTOR_CONTAINER_NAME"),
		Namespace:                    os.Getenv("DOVECOT_NAMESPACE"),
		SyncFrequencyDuration:        syncFrequencyDuration,
	}

	dovecotPods := GetPodsByLabel(clientset, envVariables.Namespace, envVariables.DovecotLabels)
	initialDovecotPodCount = len(dovecotPods.Items)

	k3sVars := K3sVars{
		Kubeconf:  kubeconfig,
		Clientset: clientset,
	}
	StartWatchers(envVariables, k3sVars)
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

func ExecuteCommand(command string, podname string, namespace string, containerName string, k3sVars K3sVars) error {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := k3sVars.Clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podname).Namespace(namespace).SubResource("exec")
	// THE FOLLOWING EXPECTS THE POD TO HAVE ONLY ONE CONTAINER IN WHICH THE COMMAND IS GOING TO BE EXECUTED
	option := &v1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(k3sVars.Kubeconf, "POST", req.URL())
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

func handleEvent(pod *v1.Pod, envVars EnvVariables, k3sVars K3sVars) {
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
				ExecuteDoveAdm(envVars, k3sVars, "pod", 0)
			}
		}
	}
}

func ExecuteDoveAdm(envVars EnvVariables, k3sVars K3sVars, trigger string, sleepTime int) {
	if sleepTime != 0 {
		time.Sleep(time.Second * time.Duration(int64(sleepTime)))
	}
	podlist := GetPodsByLabel(k3sVars.Clientset, envVars.Namespace, envVars.DovecotDirectorLabels)

	for _, dovecotDirectorPod := range podlist.Items {
		curTime := time.Now()
		logLevel := "info"
		logMessage := "success"
		formattedTime := curTime.Format("2006-01-02 15:04:05 MST")

		err := ExecuteCommand(
			"doveadm reload",
			dovecotDirectorPod.ObjectMeta.Name,
			envVars.Namespace,
			envVars.DovecotDirectorContainerName,
			k3sVars)

		if err != nil {
			logLevel = "error"
			logMessage = err.Error()
		}

		log := fmt.Sprintf("{ \"level\": \"%s\", \"timestamp\": \"%s\", \"pod\": \"%s\", \"command\": \"doveadm reload\", \"triggered-by\": \"%s\", \"message\": \"%s\" }",
			logLevel,
			formattedTime,
			dovecotDirectorPod.ObjectMeta.Name,
			trigger,
			logMessage,
		)
		fmt.Println(log)
	}
}

func StartWatchers(envVars EnvVariables, k3sVars K3sVars) {
	watchlistSecrets := cache.NewFilteredListWatchFromClient(
		k3sVars.Clientset.CoreV1().RESTClient(),
		"secrets",
		envVars.Namespace,
		func(options *metav1.ListOptions) {},
	)
	_, controllerSecrets := cache.NewInformer(
		watchlistSecrets,
		&v1.Secret{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				secret := obj.(*v1.Secret)
				if secret.Type == "kubernetes.io/tls" {
					go ExecuteDoveAdm(envVars, k3sVars, "secret", envVars.SyncFrequencyDuration)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				secret := newObj.(*v1.Secret)
				if secret.Type == "kubernetes.io/tls" {
					go ExecuteDoveAdm(envVars, k3sVars, "secret", envVars.SyncFrequencyDuration)
				}
			},
		},
	)

	watchlistPods := cache.NewFilteredListWatchFromClient(
		k3sVars.Clientset.CoreV1().RESTClient(),
		"pods",
		envVars.Namespace,
		func(options *metav1.ListOptions) { options.LabelSelector = envVars.DovecotLabels },
	)

	_, controllerPods := cache.NewInformer(
		watchlistPods,
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				handleEvent(obj.(*v1.Pod), envVars, k3sVars)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				handleEvent(newObj.(*v1.Pod), envVars, k3sVars)
			},
		},
	)

	go controllerPods.Run(make(chan struct{}))
	go controllerSecrets.Run(make(chan struct{}))

	for {
		time.Sleep(time.Second)
	}
}

func InClusterAuth() (*kubernetes.Clientset, *rest.Config, error) {
	var err error
	kubeconf, err := rest.InClusterConfig()

	if err != nil {
		return nil, nil, nil
	}

	clientset, err := kubernetes.NewForConfig(kubeconf)
	if err != nil {
		panic(err.Error())
	}

	return clientset, kubeconf, nil
}

func OutOfClusterAuth() (*kubernetes.Clientset, *rest.Config, error) {
	var configPath *string
	if home := homedir.HomeDir(); home != "" {
		configPath = flag.String("c", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the configPath file")
	} else {
		configPath = flag.String("c", "", "absolute path to the configPath file")
	}
	flag.Parse()

	var err error
	kubeconf, err := clientcmd.BuildConfigFromFlags("", *configPath)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(kubeconf)
	if err != nil {
		panic(err.Error())
	}

	return clientset, kubeconf, nil
}
