package handlers

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	//import kubernetes clientcmdapi
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const role = "edit"
const separationString = "-"
const generatedDefaulfSuffixLength = 5
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// generic parser for json requests with echo context and return a models.Namespace struct
func (c *Container) parseNamespaceRequest(ctx echo.Context) models.Namespace {
	ns := models.Namespace{}
	if err := ctx.Bind(&ns); err != nil {
		log.Errorf("Error parsing namespace request: %s", err)
		c.sendErrorResponse(ctx, "", "Error parsing namespace request", http.StatusBadRequest)
	}
	return ns
}

// parses different errors from kubernetes and returns a custom error message
func (c *Container) NamespaceErrorHandler(ctx echo.Context, err error) error {
	if strings.Contains(err.Error(), "must be no more than 63 characters") {
		c.sendErrorResponse(ctx, "", "Namespace name must be no more than 63 characters", http.StatusBadRequest)
	}

	return c.sendErrorResponse(ctx, "", "Error creating namespace", http.StatusInternalServerError)
}

func (c *Container) send200Reponse(ctx echo.Context, namespace string, message string) error {
	response := models.PostNamespace200Response{
		Message:   message,
		Namespace: namespace,
	}
	return ctx.JSON(http.StatusOK, response)
}

func (c *Container) sendErrorResponse(ctx echo.Context, namespace string, message string, status int) error {
	response := models.PostNamespaceErrorResponse{
		Message:   message,
		Namespace: namespace,
	}
	return ctx.JSON(status, response)
}

// CreateNamespace - Create a new namespace
func (c *Container) CreateNamespace(ctx echo.Context) error {
	namespaceList, _ := getNamespaceList(c.clientset)
	ns := c.parseNamespaceRequest(ctx)
	nsSpec, _ := c.craftNamespaceSpecification(&ns, ctx)
	if !existsNamespace(namespaceList, nsSpec.ObjectMeta.Name) {
		// create namespace
		c.createNamespace(ctx, c.clientset, nsSpec, namespaceList)

		trb := c.craftTenamaRoleBinding(nsSpec.ObjectMeta.Name, "tenama")
		c.createRolebinding(ctx, c.clientset, trb, nsSpec.ObjectMeta.Name)

		quotaSpec := c.craftNamespaceQuotaSpecification(nsSpec.ObjectMeta.Name)
		c.createNamespaceQuota(ctx, c.clientset, quotaSpec, nsSpec.ObjectMeta.Name)

		serviceAccountSpec := c.craftServiceAccountSpecification(nsSpec.ObjectMeta.Name)
		c.createServiceAccount(ctx, c.clientset, serviceAccountSpec, nsSpec.ObjectMeta.Name)

		rbSpec, _ := c.craftUserRolebindings(nsSpec.ObjectMeta.Name, ns.Users, serviceAccountSpec.ObjectMeta.Name)
		c.createRolebinding(ctx, c.clientset, rbSpec, nsSpec.ObjectMeta.Name)

		serviceAccountTokenSecret := c.craftServiceAccountTokenSecretSpecificationn(nsSpec.ObjectMeta.Name)
		secret := c.createSecretForServiceAccountToken(ctx, c.clientset, serviceAccountTokenSecret, nsSpec.ObjectMeta.Name)

		kubeconfig := c.GetKubeconfig(ctx, nsSpec.ObjectMeta.Name, secret)
		//convert kubeconfig to valide yaml configuration and return it as yaml response
		kubeconfigYaml := c.convertKubeconfigToYaml(ctx, nsSpec.ObjectMeta.Name, kubeconfig)

		response := models.PostNamespace200Response{
			Message:    "Namespace created",
			Namespace:  nsSpec.ObjectMeta.Name,
			KubeConfig: kubeconfigYaml,
		}
		return ctx.JSON(http.StatusOK, response)

	}
	return c.sendErrorResponse(ctx, nsSpec.ObjectMeta.Name, "Namespace already exists", http.StatusConflict)
}

// DeleteNamespace - Deletes a namespace
func (c *Container) DeleteNamespace(ctx echo.Context) error {
	// get existing ns
	namespace := strings.Trim(ctx.Param("namespace"), "/")

	if !strings.HasPrefix(namespace, c.config.Namespace.Prefix) {
		log.Infof("Namespace %s does not start with prefix %s", namespace, c.config.Namespace.Prefix)
		c.sendErrorResponse(ctx, namespace, "Namespace does not start with prefix "+c.config.Namespace.Prefix, http.StatusBadRequest)
	}

	log.Infof("Delete namespace %s through an API call.", namespace)
	err := c.clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Error deleting namespace: %s", err)
		c.sendErrorResponse(ctx, namespace, "Namespace not found", http.StatusInternalServerError)
	}

	return c.sendErrorResponse(ctx, namespace, "Namespace successfully deleted", http.StatusOK)
}

// GetNamespaces - Get all namespaces
func (c *Container) GetNamespaces(ctx echo.Context) error {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "created-by=tenama",
	})
	if err != nil {
		log.Errorf("Error getting namespaces: %s", err)
		c.sendErrorResponse(ctx, "", "Error getting namespaces", http.StatusInternalServerError)
	}

	// convert namespaces to a list of strings
	var nsList []string
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.ObjectMeta.Name)
	}

	successResponse := models.GetNamespaces200Response{
		Message:    "Namespaces successfully retrieved",
		Namespaces: nsList,
	}

	return ctx.JSON(http.StatusOK, successResponse)
}

// GetNamespaceByName - Find namespace by name
func (c *Container) GetNamespaceByName(ctx echo.Context) error {
	// get existing ns
	namespace := strings.Trim(ctx.Param("namespace"), "/")

	//Check if namespace is valid and starts with the prefix from the config file (e.g. tenama)
	if !strings.HasPrefix(namespace, c.config.Namespace.Prefix) {
		log.Warnf("SearchingNamespace %s is invalid", namespace)
		c.sendErrorResponse(ctx, namespace, "Namespace is invalid", http.StatusBadRequest)
	}

	ns, err := c.clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting namespace: %s", err)
		c.sendErrorResponse(ctx, namespace, "Namespace not found", http.StatusInternalServerError)
	}

	//check if namespace is found
	if ns == nil {
		log.Warnf("Namespace %s not found", namespace)
		c.sendErrorResponse(ctx, namespace, "Namespace not found", http.StatusNotFound)
	}

	return c.sendErrorResponse(ctx, namespace, "Namespace successfully found", http.StatusOK)
}

// convertKubeconfigToYaml
func (c *Container) convertKubeconfigToYaml(ctx echo.Context, namespace string, kubeconfig *clientcmdapi.Config) []byte {
	var kubeconfigYaml []byte
	var err error
	if kubeconfigYaml, err = clientcmd.Write(*kubeconfig); err != nil {
		log.Errorf("Error converting kubeconfig to yaml: %s", err)
		c.sendErrorResponse(ctx, namespace, "Error converting kubeconfig to yaml", http.StatusInternalServerError)
	}
	return kubeconfigYaml
}

// get secret name with service account token for a given namespace and generate a kubeconfigiuration
func (c *Container) GetKubeconfig(ctx echo.Context, namespace string, secret *v1.Secret) *clientcmdapi.Config {
	serviceAccountSecret, err := c.clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error getting service account token secret: %s", err)
		c.sendErrorResponse(ctx, namespace, "Error getting service account token secret", http.StatusInternalServerError)
		return nil
	}
	kubeconfig := c.craftKubeconfig(ctx, namespace, serviceAccountSecret)
	if err != nil {
		log.Errorf("Error crafting kubeconfig: %s", err)
		c.sendErrorResponse(ctx, namespace, "Error crafting kubeconfig", http.StatusInternalServerError)
		return nil
	}
	return kubeconfig
}

// get namespace and service account token secret name for a given namespace
// craft a kubeconfig and return it
func (c *Container) craftKubeconfig(ctx echo.Context, namespace string, secret *v1.Secret) *clientcmdapi.Config {
	clusterName := "default"
	// get cluster endpoint
	clusterEndpoint := c.clientset.CoreV1().RESTClient().Get().URL().Host
	// get cluster certificate authority data
	clusterCertificateAuthorityData := secret.Data["ca.crt"]
	// get service account token
	serviceAccountToken := secret.Data["token"]
	// get service account name
	serviceAccountName := secret.Annotations["kubernetes.io/service-account.name"]
	// get service account namespace
	serviceAccountNamespace := secret.Namespace

	// create a kubeconfig
	kubeconfig := clientcmdapi.NewConfig()
	// set cluster
	kubeconfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   clusterEndpoint,
		CertificateAuthorityData: clusterCertificateAuthorityData,
	}
	// set auth info
	kubeconfig.AuthInfos[serviceAccountName] = &clientcmdapi.AuthInfo{
		Token: string(serviceAccountToken),
	}
	// set context
	kubeconfig.Contexts[serviceAccountName] = &clientcmdapi.Context{
		Cluster:   clusterName,
		AuthInfo:  serviceAccountName,
		Namespace: serviceAccountNamespace,
	}
	// set current context
	kubeconfig.CurrentContext = serviceAccountName

	return kubeconfig
}

// craft rolebinding for service account tenama from tenama-system namespace and bind clusterrole admin
func (c *Container) craftTenamaRoleBinding(namespace string, serviceAccountName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenama-admin",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: "tenama-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func (c *Container) craftUserRolebindings(namespace string, users []string, serviceAccountName string) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace + "troubleshooters",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     role,
		},
	}

	for _, user := range users {
		rb.Subjects = append(rb.Subjects, rbacv1.Subject{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     user,
		})
	}

	// add ServiceAccount that is returned to the caller so that it can access the namespace
	rb.Subjects = append(rb.Subjects, rbacv1.Subject{
		Kind: rbacv1.ServiceAccountKind,
		Name: serviceAccountName,
	})

	return rb, nil
}

func (c *Container) createRolebinding(ctx echo.Context, clientset *kubernetes.Clientset, rb *rbacv1.RoleBinding, ns string) {
	log.Debugf("creating binding: %s for service account %s in namespace %s for users", rb.Name, rb.Subjects[:len(rb.Subjects)-1], ns)
	rb, err := clientset.RbacV1().RoleBindings(ns).Create(context.TODO(), rb, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating rolebinding: %s", err)
		c.sendErrorResponse(ctx, ns, "Error creating rolebinding", http.StatusInternalServerError)
	}
}

// Checks if resource values are set in the config file and
// crafts a ResourceQuota for the namespace
func (c *Container) craftNamespaceQuotaSpecification(namespace string) *v1.ResourceQuota {
	log.Debugf("crafting quota for the namespace %s", namespace)

	quota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.config.Namespace.Prefix + separationString + "quota",
			Namespace: namespace,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: make(v1.ResourceList),
		},
	}

	if c.config.Namespace.Resources.Limits.CPU != "" {
		namespaceResourcesCPULimit, err := resource.ParseQuantity(c.config.Namespace.Resources.Limits.CPU)
		if err == nil {
			quota.Spec.Hard[v1.ResourceLimitsCPU] = namespaceResourcesCPULimit
		}
	}
	if c.config.Namespace.Resources.Limits.Memory != "" {
		namespaceResourcesMemoryLimit, err := resource.ParseQuantity(c.config.Namespace.Resources.Limits.Memory)
		if err == nil {
			quota.Spec.Hard[v1.ResourceLimitsMemory] = namespaceResourcesMemoryLimit
		}
	}
	if c.config.Namespace.Resources.Requests.CPU != "" {
		namespaceResourcesCPURequest, err := resource.ParseQuantity(c.config.Namespace.Resources.Requests.CPU)
		if err == nil {
			quota.Spec.Hard[v1.ResourceRequestsCPU] = namespaceResourcesCPURequest
		}
	}
	if c.config.Namespace.Resources.Requests.Memory != "" {
		namespaceResourcesMemoryRequest, err := resource.ParseQuantity(c.config.Namespace.Resources.Requests.Memory)
		if err == nil {
			quota.Spec.Hard[v1.ResourceRequestsMemory] = namespaceResourcesMemoryRequest
		}
	}
	if c.config.Namespace.Resources.Requests.Storage != "" {
		namespaceResourcesStorageRequest, err := resource.ParseQuantity(c.config.Namespace.Resources.Requests.Storage)
		if err == nil {
			quota.Spec.Hard[v1.ResourceRequestsStorage] = namespaceResourcesStorageRequest
		}
	}

	return quota
}

// craft ServiceAccount to give access to the newly generated namespace
func (c *Container) craftServiceAccountSpecification(namespace string) *v1.ServiceAccount {
	log.Debugf("crafting service account for the namespace %s", namespace)
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.config.Namespace.Prefix + separationString + "sa",
			Namespace: namespace,
		},
	}
}

func (c *Container) createServiceAccount(ctx echo.Context, clientset *kubernetes.Clientset, sa *v1.ServiceAccount, ns string) {
	log.Debugf("creating ServiceAccount %s in namespace %s", sa.Name, ns)
	sa, err := clientset.CoreV1().ServiceAccounts(ns).Create(context.TODO(), sa, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating service account: %s", err)
		c.sendErrorResponse(ctx, ns, "Error creating service account", http.StatusInternalServerError)
	}
}

// craft secret for service account token for the crafted ServiceAccount
func (c *Container) craftServiceAccountTokenSecretSpecificationn(namespace string) *v1.Secret {
	log.Debugf("crafting secret for the service account in the namespace %s", namespace)
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.config.Namespace.Prefix + separationString + "sa-token",
			Namespace:   namespace,
			Annotations: map[string]string{"kubernetes.io/service-account.name": c.config.Namespace.Prefix + separationString + "sa"},
		},
		Type: "kubernetes.io/service-account-token",
	}
}

func (c *Container) createSecretForServiceAccountToken(ctx echo.Context, clientset *kubernetes.Clientset, secret *v1.Secret, ns string) *v1.Secret {
	log.Debugf("creating Secret %s in namespace %s", secret.Name, ns)
	//Create Token Secret, wait for it to be created and then return it

	secret, err := clientset.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating secret: %s", err)
		c.sendErrorResponse(ctx, ns, "Error creating ServiceAccount secret", http.StatusInternalServerError)
	}
	//loop until secret has a data field with a token in it
	// or until timeout is reached (10 seconds) and then return it
	// or error if timeout is reached before token is created in secret data field
	timeout := time.After(10 * time.Second)
	//use ticker to check every 500ms if secret has token in data field
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-timeout:
			log.Errorf("timeout reached before token was created in secret data field")
			c.sendErrorResponse(ctx, ns, "timeout reached before token was created in secret data field", http.StatusInternalServerError)
		case <-ticker.C:
			secret, err := clientset.CoreV1().Secrets(ns).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				log.Errorf("Error getting secret: %s", err)
				c.sendErrorResponse(ctx, ns, "Error getting ServiceAccount secret", http.StatusInternalServerError)
				return nil
			}
			if secret.Data["token"] != nil {
				return secret
			}
		}
	}
}

func (c *Container) createNamespaceQuota(ctx echo.Context, clientset *kubernetes.Clientset, quota *v1.ResourceQuota, ns string) {
	log.Debugf("creating quota %s in namespace %s", quota.Name, ns)
	quota, err := clientset.CoreV1().ResourceQuotas(ns).Create(context.TODO(), quota, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating namespace quota: %s", err)
		c.sendErrorResponse(ctx, ns, "Error creating namespace quota", http.StatusInternalServerError)
	}
}

func (c *Container) craftNamespaceSpecification(ns *models.Namespace, ctx echo.Context) (*v1.Namespace, error) {
	var nsn string

	if c.config.Namespace.Prefix == "" {
		log.Errorf("Prefix is not set in config file")
		return nil, errors.New("prefix is not set in config file")
	}

	nsn = c.config.Namespace.Prefix + separationString

	if ns.Infix == "" {
		log.Errorf("Infix is not set in request")
		return nil, errors.New("infix is not set in request")
	}

	nsn = nsn + ns.Infix + separationString

	nsn, err := validateAndTransformToK8sName(nsn, []rune(separationString)[0])
	if err != nil {
		log.Errorf("Error parsing namespace name: %s", nsn)
	}

	if ns.Suffix != "" {
		nsn = nsn + separationString + ns.Suffix
	} else {
		// generate randomstring for namespace postfix if buildhash is unset, avoiding collisions
		nsn = nsn + separationString + StringWithCharset(generatedDefaulfSuffixLength, charset)
	}

	namespaceDuration, err := time.ParseDuration(ns.Duration)
	if err != nil {
		log.Warnf("Error parsing duration: %s", ns.Duration)
		c.sendErrorResponse(ctx, nsn, "Error parsing duration", http.StatusBadRequest)
	}

	ns.Duration = fmt.Sprint(namespaceDuration)

	podSecurityStandardVersion, err := getK8sServerVersion(c.clientset)
	if err != nil {
		log.Warnf("Error getting kubernetes server version: %s", err)
	}

	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsn,
			Labels: map[string]string{
				"created-by":                                 "tenama",
				"tenama/namespace-duration":                  ns.Duration,
				"pod-security.kubernetes.io/enforce":         "baseline",
				"pod-security.kubernetes.io/enforce-version": podSecurityStandardVersion,
			},
		},
	}

	return nsSpec, err
}

func getK8sServerVersion(clientset *kubernetes.Clientset) (string, error) {
	information, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return "latest", err
	}
	return "v" + information.Major + "." + information.Minor, nil
}

func existsNamespace(namespaceList *v1.NamespaceList, namespace string) bool {
	for _, ns := range namespaceList.Items {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}

func existsNamespaceWithPrefix(namespaceList *v1.NamespaceList, namespacePrefix string) bool {
	for _, ns := range namespaceList.Items {
		if strings.Contains(ns.Name, namespacePrefix) {
			return true
		}
	}
	return false
}

func getNamespaceList(clientset *kubernetes.Clientset) (*v1.NamespaceList, error) {
	nl, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	return nl, err
}

func (c *Container) createNamespace(ctx echo.Context, clientset *kubernetes.Clientset, nsSpec *v1.Namespace, namespaceList *v1.NamespaceList) {
	log.Infof("Considering to create namespace %s", nsSpec.Name)
	if !existsNamespaceWithPrefix(namespaceList, nsSpec.Name) {
		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), nsSpec, metav1.CreateOptions{})
		if err != nil {
			log.Errorf("Error creating namespace %s: %s", nsSpec.Name, err)
			c.sendErrorResponse(ctx, nsSpec.ObjectMeta.Name, "Error creating namespace", http.StatusInternalServerError)
		}
		log.Infof("Created Namespace %s", nsSpec.Name)
	}
	log.Warnf("Namespace matching %s already exists!", nsSpec.Name)
}

// replaces k8s invalid chars (separationRune) in inputString
func validateAndTransformToK8sName(inputString string, separationRune rune) (string, error) {
	// init
	var err error

	// pre-validate
	if inputString == "" {
		return "", errors.New("parameter namespace is required")
	}

	// lowercase
	inputStringLowerCase := strings.ToLower(inputString)

	// replace invalid characters
	r, _ := regexp.Compile(`[-a-z\\d]`)
	normalizedNameRunes := []rune("")
	for _, ch := range inputStringLowerCase {
		chs := string(ch)
		if !r.MatchString(chs) {
			log.Debugf("namespace '%s' contains invalid character: %s,"+
				"allowed are only ones that match the regex: %s, appending a '%s' instead of this character!",
				inputStringLowerCase, chs, r, string(separationRune))
			normalizedNameRunes = append(normalizedNameRunes, separationRune)
		}
		normalizedNameRunes = append(normalizedNameRunes, ch)
	}

	// truncate too long name
	// RFC 1123 Label Names
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	if len(normalizedNameRunes) > 63 {
		normalizedNameRunes = normalizedNameRunes[:62]
	}

	// chomp minuses at beginning
	normalizedNameRunes = chompBeginningCharacter(normalizedNameRunes, separationRune)

	// chomp minuses at end
	normalizedNameRunes = chompEndingCharacter(normalizedNameRunes, separationRune)

	// convert rune array to string
	normalizedNameString := string(normalizedNameRunes)

	// post-validate
	if normalizedNameString == "" {
		return "",
			fmt.Errorf("namespace empty after matching all characters against regex: '%s'", r)
	}

	return normalizedNameString, err
}

func chompBeginningCharacter(runearr []rune, runechar rune) []rune {
	chomping := true
	var chompedRune []rune
	for _, cr := range runearr {
		if chomping && cr == runechar {
			log.Debugf("chomping character %s from string %s", string(cr), string(runechar))
		} else {
			chompedRune = append(chompedRune, cr)
			chomping = false
		}
	}
	return chompedRune
}

// chompEndingCharacter removes the ending character from the given rune array recursively.
// If the rune array is empty, it returns an empty rune array.
// If the last element of the rune array is equal to the specified rune character,
// it recursively removes the last element until it finds a different character.
// It returns the modified rune array.
func chompEndingCharacter(runearr []rune, runechar rune) []rune {
	if len(runearr) == 0 {
		return []rune{}
	}
	if runearr[len(runearr)-1] == runechar {
		return chompEndingCharacter(runearr[:len(runearr)-1], runechar)
	}

	return runearr
}

// StringWithCharset generates a random string of the specified length using the characters from the given charset.
// It returns the generated random string.
func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		randombytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randombytes)
}
