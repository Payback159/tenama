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

	"github.com/Payback159/tenama/models"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const role = "edit"
const separationString = "-"
const generatedDefaulfSuffixLength = 5
const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// generic parser for json requests with echo context and return a models.Namespace struct
func parseNamespaceRequest(ctx echo.Context) (models.Namespace, error) {
	ns := models.Namespace{}
	if err := ctx.Bind(&ns); err != nil {
		return ns, err
	}
	return ns, nil
}

// CreateNamespace - Create a new namespace
// TODO: reduce complexity
func (c *Container) CreateNamespace(ctx echo.Context) error {
	namespaceList, _ := getNamespaceList(c.clientset)
	ns, _ := parseNamespaceRequest(ctx)
	nsSpec, _ := c.craftNamespaceSpecification(&ns, ctx)
	if !existsNamespace(namespaceList, nsSpec.ObjectMeta.Name) {
		createNamespace(c.clientset, nsSpec, namespaceList)
		rbSpec, _ := c.craftRolebindingSpecification(nsSpec.ObjectMeta.Name, ns.Users)
		_, err := createRolebinding(c.clientset, rbSpec, nsSpec.ObjectMeta.Name)
		if err != nil {
			log.Errorf("Error creating rolebinding: %s", err)
			errorResponse := models.Response{
				Message:   "Error creating rolebinding",
				Namespace: nsSpec.ObjectMeta.Name,
			}
			return ctx.JSON(http.StatusInternalServerError, errorResponse)
		}
		quotaSpec := c.craftNamespaceQuotaSpecification(nsSpec.ObjectMeta.Name)
		_, err = createNamespaceQuota(c.clientset, quotaSpec, nsSpec.ObjectMeta.Name)
		if err != nil {
			log.Errorf("Error creating namespace quota: %s", err)
			errorResponse := models.Response{
				Message:   "Error creating namespace quota",
				Namespace: nsSpec.ObjectMeta.Name,
			}
			return ctx.JSON(http.StatusInternalServerError, errorResponse)
		}
		serviceAccountSpec := c.craftServiceAccountSpecification(nsSpec.ObjectMeta.Name)
		_, err = c.createServiceAccount(c.clientset, serviceAccountSpec, nsSpec.ObjectMeta.Name)
		if err != nil {
			log.Errorf("Error creating service account: %s", err)
			errorResponse := models.Response{
				Message:   "Error creating service account",
				Namespace: nsSpec.ObjectMeta.Name,
			}
			return ctx.JSON(http.StatusInternalServerError, errorResponse)
		}
		serviceAccountTokenSecret := c.craftServiceAccountTokenSecretSpecificationn(nsSpec.ObjectMeta.Name)
		accessToken, err := c.createSecretForServiceAccountToken(c.clientset, serviceAccountTokenSecret, nsSpec.ObjectMeta.Name)
		if err != nil {
			log.Errorf("Error creating service account token secret: %s", err)
			errorResponse := models.Response{
				Message:   "Error creating service account token secret",
				Namespace: nsSpec.ObjectMeta.Name,
			}
			return ctx.JSON(http.StatusInternalServerError, errorResponse)
		}

		successResponse := models.Response{
			Message:     "Namespace successfully created",
			Namespace:   nsSpec.ObjectMeta.Name,
			AccessToken: string(accessToken.Name),
		}

		return ctx.JSON(http.StatusOK, successResponse)
	} else {
		errorResponse := models.Response{
			Message:   "Namespace already exists",
			Namespace: nsSpec.ObjectMeta.Name,
		}
		return ctx.JSON(http.StatusConflict, errorResponse)
	}
}

// DeleteNamespace - Deletes a namespace
func (c *Container) DeleteNamespace(ctx echo.Context) error {
	// get existing ns
	namespace := strings.Trim(ctx.Param("namespace"), "/")

	if strings.HasPrefix(namespace, c.config.Namespace.Prefix) {
		log.Infof("Delete namespace %s through an API call.", namespace)
		err := c.clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		if err != nil {
			log.Errorf("Error deleting namespace: %s", err)
			errorResponse := models.Response{
				Message:   "Namespace not found",
				Namespace: namespace,
			}
			return ctx.JSON(http.StatusNotFound, errorResponse)
		}
	} else {
		log.Errorf("Namespace %s not found", namespace)
		errorResponse := models.Response{
			Message:   "Namespace not found",
			Namespace: namespace,
		}
		return ctx.JSON(http.StatusNotFound, errorResponse)
	}

	successResponse := models.Response{
		Message:   "Namespace successfully deleted",
		Namespace: namespace,
	}
	return ctx.JSON(http.StatusOK, successResponse)
}

// GetNamespaceByName - Find namespace by name
func (c *Container) GetNamespaceByName(ctx echo.Context) error {
	// get existing ns
	namespace := strings.Trim(ctx.Param("namespace"), "/")

	if !strings.HasPrefix(namespace, c.config.Namespace.Prefix) {
		log.Errorf("Namespace %s not found", namespace)
		errorResponse := models.Response{
			Message:   "Namespace not found",
			Namespace: namespace,
		}
		return ctx.JSON(http.StatusNotFound, errorResponse)
	} else {
		successReponse := models.Response{
			Message:   "Namespace successfully found",
			Namespace: namespace,
		}
		return ctx.JSON(http.StatusOK, successReponse)
	}
}

func (c *Container) craftRolebindingSpecification(namespace string, users []string) (*rbacv1.RoleBinding, error) {
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

	return rb, nil
}

func createRolebinding(clientset *kubernetes.Clientset, rb *rbacv1.RoleBinding, ns string) (*rbacv1.RoleBinding, error) {
	rb, err := clientset.RbacV1().RoleBindings(ns).Create(context.TODO(), rb, metav1.CreateOptions{})
	return rb, err
}

func (c *Container) craftNamespaceQuotaSpecification(namespace string) *v1.ResourceQuota {
	return &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.config.Namespace.Prefix + separationString + "quota",
			Namespace: namespace,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: v1.ResourceList{
				v1.ResourceLimitsCPU:       resource.MustParse(c.config.Namespace.Resources.Limits.CPU),
				v1.ResourceLimitsMemory:    resource.MustParse(c.config.Namespace.Resources.Limits.Memory),
				v1.ResourceRequestsCPU:     resource.MustParse(c.config.Namespace.Resources.Requests.CPU),
				v1.ResourceRequestsMemory:  resource.MustParse(c.config.Namespace.Resources.Requests.Memory),
				v1.ResourceRequestsStorage: resource.MustParse(c.config.Namespace.Resources.Requests.Storage),
			},
		},
	}
}

// craft ServiceAccount to give access to the newly generated namespace
func (c *Container) craftServiceAccountSpecification(namespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.config.Namespace.Prefix + separationString + "sa",
			Namespace: namespace,
		},
	}
}

func (c *Container) createServiceAccount(clientset *kubernetes.Clientset, sa *v1.ServiceAccount, ns string) (*v1.ServiceAccount, error) {
	sa, err := clientset.CoreV1().ServiceAccounts(ns).Create(context.TODO(), sa, metav1.CreateOptions{})
	return sa, err
}

// craft secret for service account token for the crafted ServiceAccount
func (c *Container) craftServiceAccountTokenSecretSpecificationn(namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.config.Namespace.Prefix + separationString + "sa-token",
			Namespace:   namespace,
			Annotations: map[string]string{"kubernetes.io/service-account.name": c.config.Namespace.Prefix + separationString + "sa"},
		},
		Type: "kubernetes.io/service-account-token",
	}
}

func (c *Container) createSecretForServiceAccountToken(clientset *kubernetes.Clientset, secret *v1.Secret, ns string) (*v1.Secret, error) {
	secret, err := clientset.CoreV1().Secrets(ns).Create(context.TODO(), secret, metav1.CreateOptions{})
	return secret, err
}

func createNamespaceQuota(clientset *kubernetes.Clientset, quota *v1.ResourceQuota, ns string) (*v1.ResourceQuota, error) {
	quota, err := clientset.CoreV1().ResourceQuotas(ns).Create(context.TODO(), quota, metav1.CreateOptions{})
	return quota, err
}

func (c *Container) craftNamespaceSpecification(ns *models.Namespace, ctx echo.Context) (*v1.Namespace, error) {

	var nsn string

	if c.config.Namespace.Prefix == "" {
		log.Errorf("Prefix is not set in config file")
	} else {
		nsn = c.config.Namespace.Prefix + separationString
	}
	if ns.Infix == "" {
		log.Errorf("Infix is not set in request")
	} else {
		nsn = nsn + ns.Infix + separationString
	}
	nsn, err := validateAndTransformToK8sName(nsn, []rune(separationString)[0])
	if err != nil {
		log.Fatalf("Error parsing namespace name: %s", nsn)
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
		log.Infof("Namespace duration is not set, using default value %s", c.config.Namespace.Duration)
		ns.Duration = c.config.Namespace.Duration
	} else {
		ns.Duration = fmt.Sprint(namespaceDuration)
	}

	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsn,
			Labels: map[string]string{
				"created-by":                "tenama",
				"tenama/namespace-duration": ns.Duration,
			},
		},
	}

	return nsSpec, err
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

func createNamespace(clientset *kubernetes.Clientset, nsSpec *v1.Namespace, namespaceList *v1.NamespaceList) *v1.Namespace {
	log.Info("Considering to create namespace " + nsSpec.Name)
	if !existsNamespaceWithPrefix(namespaceList, nsSpec.Name) {
		ns, err := clientset.CoreV1().Namespaces().Create(context.TODO(), nsSpec, metav1.CreateOptions{})
		if err != nil {
			log.Fatalf("Error creating namespace %s, error was: %s", nsSpec, err)
		}
		log.Infof("Created Namespace %s", nsSpec.Name)
		return ns
	} else {
		log.Infof("Namespace matching %s already exists!", nsSpec.Name)
		return nsSpec
	}
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
		} else {
			normalizedNameRunes = append(normalizedNameRunes, ch)
		}
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

func chompEndingCharacter(runearr []rune, runechar rune) []rune {
	if len(runearr) == 0 {
		return []rune{}
	}
	if runearr[len(runearr)-1] == runechar {
		return chompEndingCharacter(runearr[:len(runearr)-1], runechar)
	} else {
		return runearr
	}
}

func StringWithCharset(length int, charset string) string {
	randombytes := make([]byte, length)
	for i := range randombytes {
		randombytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randombytes)
}
