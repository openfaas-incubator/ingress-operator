package main

import (
	"flag"
	"os"
	"time"

	cmclientset "github.com/jetstack/cert-manager/pkg/client/clientset/versioned/typed/certmanager/v1alpha1"

	clientset "github.com/openfaas-incubator/ingress-operator/pkg/client/clientset/versioned"
	informers "github.com/openfaas-incubator/ingress-operator/pkg/client/informers/externalversions"
	"github.com/openfaas-incubator/ingress-operator/pkg/controller"
	"github.com/openfaas-incubator/ingress-operator/pkg/signals"
	"github.com/openfaas-incubator/ingress-operator/pkg/version"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	glog "k8s.io/klog"

	// required to authenticate against GKE clusters
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	masterURL  string
	kubeconfig string
)

var pullPolicyOptions = map[string]bool{
	"Always":       true,
	"IfNotPresent": true,
	"Never":        true,
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	setupLogging()

	sha, release := version.GetReleaseInfo()
	glog.Infof("Starting FunctionIngress controller version: %s commit: %s", release, sha)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building Kubernetes clientset: %s", err.Error())
	}

	faasClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building FunctionIngress clientset: %s", err.Error())
	}

	cmClient, err := cmclientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building cert-manager clientset: %s", err.Error())
	}

	functionNamespace := "openfaas"
	if namespace, exists := os.LookupEnv("function_namespace"); exists {
		functionNamespace = namespace
	}

	defaultResync := time.Second * 30

	kubeInformerOpt := kubeinformers.WithNamespace(functionNamespace)
	kubeInformerFactory := kubeinformers.
		NewSharedInformerFactoryWithOptions(kubeClient, defaultResync, kubeInformerOpt)

	faasInformerOpt := informers.WithNamespace(functionNamespace)
	faasInformerFactory := informers.
		NewSharedInformerFactoryWithOptions(faasClient, defaultResync, faasInformerOpt)

	ctrl := controller.NewController(
		kubeClient,
		faasClient,
		cmClient,
		kubeInformerFactory,
		faasInformerFactory,
	)

	go kubeInformerFactory.Start(stopCh)
	go faasInformerFactory.Start(stopCh)

	if err = ctrl.Run(1, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}
}

func setupLogging() {
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	glog.InitFlags(klogFlags)

	// Sync the glog and klog flags.
	flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)
		if f2 != nil {
			value := f1.Value.String()
			f2.Value.Set(value)
		}
	})
}
