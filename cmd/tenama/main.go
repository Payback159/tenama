package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Payback159/tenama/internal/handlers"
	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"gopkg.in/yaml.v2"

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
		log.Debugf("Using kubeconfig file: %s", *kubeconfig)
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		log.Debugf("Using in cluster configuration")
	}
	flag.Parse()

	//use the current context in kubeconfig
	config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Debugf("Could not read kubeconfig file: %s", err)
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
	e.Static("/docs", "web/swagger/")
	e.Static("/", "web/swagger/")

	// CreateNamespace - Create a new namespace
	ag.POST("", c.CreateNamespace)

	// DeleteNamespace - Deletes a namespace
	ag.DELETE("/:namespace", c.DeleteNamespace)

	// GetNamespaceList - List all namespaces
	ag.GET("", c.GetNamespaces)
	// GetNamespaceByName - Find namespace by name
	ag.GET("/:namespace", c.GetNamespaceByName)

	// Start event-based namespace watcher for lifecycle management
	namespaceWatcher := handlers.NewNamespaceWatcher(clientset.CoreV1(), cfg.Namespace.Prefix)
	if err := namespaceWatcher.Start(context.Background()); err != nil {
		log.Errorf("Failed to start namespace watcher: %s", err)
	}

	e.GET("/info", c.GetBuildInfo)
	e.GET("/healthz", c.LivenessProbe)
	e.GET("/readiness", c.ReadinessProbe)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Shutdown signal received, stopping namespace watcher...")
		namespaceWatcher.Stop()
		log.Info("Namespace watcher stopped, shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e.Shutdown(shutdownCtx)
	}()

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
