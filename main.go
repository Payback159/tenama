package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Payback159/tenama/handlers"
	"github.com/Payback159/tenama/models"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"gopkg.in/yaml.v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// It opens a file, decodes the YAML into a struct, and returns the struct
func newConfig(configPath string) (*models.Config, error) {
	config := &models.Config{}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	d := yaml.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	defer file.Close()
	return config, nil
}

// It returns a list of namespaces that have the label `created-by=tenama`
func getNamespaceList(clientset *kubernetes.Clientset) (*v1.NamespaceList, error) {
	nl, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "created-by=tenama",
	})
	return nl, err
}

// It checks if there are namespaces with the prefix defined in the config file and if the expiration
// date of the namespace is further in the future than the creation date. If the expiration date is
// further in the future than the creation date, it checks if the current date exceeds the expiration
// date. If the current date exceeds the expiration date, the namespace is deleted
func cleanupNamespaces(wg *sync.WaitGroup, clientset *kubernetes.Clientset, pre string, interval string) {
	defer wg.Done()
	cleanupInterval, _ := time.ParseDuration(interval)
	for {
		log.Infof("Check if expired namespaces with the prefix %s exist", pre)
		today := time.Now()
		// get existing ns
		namespaceList, err := getNamespaceList(clientset)
		if err != nil {
			log.Errorf("Could not list namespace: %s", err)
		}
		if len(namespaceList.Items) == 0 {
			log.Warnf("No namespaces with the prefix %s found", pre)
		}

		for _, n := range namespaceList.Items {
			log.Debugf("Iterating over namespaces: current iteration: %s", n.Name)
			if strings.HasPrefix(n.Name, pre) && n.Name != "tenama-system" {
				namespaceDuration, err := time.ParseDuration(n.Labels["tenama/namespace-duration"])
				if err != nil {
					log.Errorf("Error parsing duration of namespace %s: %s", n.Name, err)
				}
				namespaceCreationTimestamp := n.ObjectMeta.CreationTimestamp
				namespaceExpirationTimestamp := namespaceCreationTimestamp.Add(namespaceDuration)
				//Checks if the expiration date of the namespace is further in the future than the creation date.
				if namespaceExpirationTimestamp != namespaceCreationTimestamp.Time {
					//Calculates the lifetime of the namespace based on the namespace creation date + the duration defined for this namespace and checks if the current date exceeds the time.
					log.Debugf("Creation timestamp of the namespace: %s", namespaceCreationTimestamp.String())
					log.Debugf("Expiration timestamp of the namespace: %s", namespaceExpirationTimestamp.String())
					log.Debugf("Current timestamp: %s", today.String())
					if namespaceExpirationTimestamp.Before(today) {
						log.Infof("Delete namespace %s because it has expired.", n.Name)
						err := clientset.CoreV1().Namespaces().Delete(context.TODO(), n.Name, metav1.DeleteOptions{})
						if err != nil {
							log.Errorf("Error deleting namespace: %s", err)
						}
					}
				} else {
					log.Errorf("Looks like the duration label on the namespace %s is set correctly but still the expiration date is not further in the future than the creation date.", n.Name)
				}
			}
		}
		//Put the goroutine to sleep for some time to avoid
		// excessive logging & too many calls against the Kubernetes API.
		time.Sleep(cleanupInterval)
	}
}

func main() {
	// consts
	const cfgPath = "./config/config.yaml"

	var cfg *models.Config
	var clientset *kubernetes.Clientset

	cfg, err := newConfig(cfgPath)
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	if err != nil {
		panic(fmt.Sprintf("Could not parse log level from string: %s", cfg.LogLevel))
	}

	// set log level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		log.SetLevel(log.DEBUG)
	case "INFO":
		log.SetLevel(log.INFO)
	case "WARN":
		log.SetLevel(log.WARN)
	case "ERROR":
		log.SetLevel(log.ERROR)
	default:
		log.SetLevel(log.INFO)
	}

	// prepare kubernetes client configuration
	var kubeconfig *string

	// prepare kubernetes client with in cluster configuration
	var config *rest.Config
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	//use the current context in kubeconfig
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Could not read k8s cluster configuration: %s", err)
		}
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Could not create k8s client: %s", err)
	}

	c, err := handlers.NewContainer(clientset, cfg)
	if err != nil {
		log.Fatalf("Container for the handler could not be initialized: %s", err)
	}
	c.SetBasicAuthUserList(cfg)

	// create new echo instance and register authenticated group
	e := echo.New()
	e.HideBanner = true
	e.HidePort = false
	ag := e.Group("/namespace")

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	ag.Use(middleware.BasicAuth(c.BasicAuthValidator))

	// GetVersion - Outputs the version of tenama
	e.Static("/docs", ".docs/swagger/")
	e.Static("/", ".docs/swagger/")

	// CreateNamespace - Create a new namespace
	ag.POST("", c.CreateNamespace)

	// DeleteNamespace - Deletes a namespace
	ag.DELETE("/:namespace", c.DeleteNamespace)

	// GetNamespaceList - List all namespaces
	ag.GET("", c.GetNamespaces)
	// GetNamespaceByName - Find namespace by name
	ag.GET("/:namespace", c.GetNamespaceByName)

	var wg sync.WaitGroup
	wg.Add(1)
	// start namespace cleanup logic
	go cleanupNamespaces(&wg, clientset, cfg.Namespace.Prefix, cfg.CleanupInterval)

	e.GET("/info", c.GetBuildInfo)
	e.GET("/healthz", c.LivenessProbe)
	e.GET("/readiness", c.ReadinessProbe)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
	wg.Wait()
}
