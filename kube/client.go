package kube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/utilitywarehouse/kube-applier/metrics"
	"github.com/utilitywarehouse/kube-applier/sysutil"
)

const (
	// Default location of the service-account token on the cluster
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

	// Location of the kubeconfig template file within the container - see ADD command in Dockerfile
	kubeconfigTemplatePath = "/templates/kubeconfig"

	// Location of the written kubeconfig file within the container
	kubeconfigFilePath = "/etc/kubeconfig"

	enabledAnnotation = "kube-applier.io/enabled"
	dryRunAnnotation  = "kube-applier.io/dry-run"
	pruneAnnotation   = "kube-applier.io/prune"
)

// To make testing possible
var execCommand = exec.Command

//todo(catalin-ilea) Add core/v1/Secret when we plug in strongbox
var pruneWhitelist = []string{
	"apps/v1/DaemonSet",
	"apps/v1/Deployment",
	"apps/v1/StatefulSet",
	"autoscaling/v1/HorizontalPodAutoscaler",
	"batch/v1/Job",
	"core/v1/ConfigMap",
	"core/v1/Pod",
	"core/v1/Service",
	"core/v1/ServiceAccount",
	"networking.k8s.io/v1beta1/Ingress",
	"networking.k8s.io/v1/NetworkPolicy",
}

// KAAnnotations contains the standard set of annotations on the Namespace
// resource defining behaviour for that Namespace
type KAAnnotations struct {
	Enabled string
	DryRun  string
	Prune   string
}

// ClientInterface allows for mocking out the functionality of Client when testing the full process of an apply run.
type ClientInterface interface {
	Apply(path, namespace string, dryRun, prune, kustomize bool) (string, string, error)
	NamespaceAnnotations(namespace string) (KAAnnotations, error)
	NamespaceAnnotationsBatch(namespaces []string) (map[string]KAAnnotations, error)
}

// Client enables communication with the Kubernetes API Server through kubectl commands.
// The Server field enables discovery of the API server when kube-proxy is not configured (see README.md for more information).
type Client struct {
	Server  string
	Label   string
	Metrics metrics.PrometheusInterface
}

// Configure writes the kubeconfig file to be used for authenticating kubectl commands.
func (c *Client) Configure() error {
	// No need to write a kubeconfig file if Server is not specified (API server will be discovered via kube-proxy).
	if c.Server == "" {
		return nil
	}

	f, err := os.Create(kubeconfigFilePath)
	if err != nil {
		return errors.Wrap(err, "creating kubeconfig file failed")
	}
	defer f.Close()

	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return errors.Wrap(err, "cannot access token for kubeconfig file")
	}

	var data struct {
		Token  string
		Server string
	}
	data.Token = string(token)
	data.Server = c.Server

	template, err := sysutil.CreateTemplate(kubeconfigTemplatePath)
	if err != nil {
		return errors.Wrap(err, "parsing kubeconfig template failed")
	}
	if err := template.Execute(f, data); err != nil {
		return errors.Wrap(err, "applying kubeconfig template failed")
	}

	return nil
}

// Apply attempts to "kubectl apply" the files located at path. It returns the
// full apply command and its output.
//
// kustomize - Do a `kubectl apply -k` on the path, set to if there is a
//             `kustomization.yaml` found in the path
func (c *Client) Apply(path, namespace string, dryRun, prune, kustomize bool) (string, string, error) {
	var args []string

	if kustomize {
		args = []string{"kubectl", "apply", fmt.Sprintf("--server-dry-run=%t", dryRun), "-k", path, "-n", namespace}
	} else {
		args = []string{"kubectl", "apply", fmt.Sprintf("--server-dry-run=%t", dryRun), "-R", "-f", path, "-n", namespace}
	}

	if prune {
		args = append(args, "--prune")
		args = append(args, "--all")
		for _, w := range pruneWhitelist {
			args = append(args, "--prune-whitelist="+w)
		}
	}

	if c.Server != "" {
		args = append(args, fmt.Sprintf("--kubeconfig=%s", kubeconfigFilePath))
	}

	kubectlCmd := exec.Command(args[0], args[1:]...)

	cmdStr := strings.Join(args, " ")

	out, err := kubectlCmd.CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
		}
		return cmdStr, string(out), err
	}
	c.Metrics.UpdateKubectlExitCodeCount(path, 0)

	return cmdStr, string(out), err
}

type namespaceResponse struct {
	Metadata struct {
		Name        string
		Annotations map[string]string
	}
}

type namespaceResponseList struct {
	Items []struct {
		Metadata struct {
			Name        string
			Annotations map[string]string
		}
	}
}

// NamespaceAnnotations returns string values of kube-applier annotaions
func (c *Client) NamespaceAnnotations(namespace string) (KAAnnotations, error) {
	kaa := KAAnnotations{}
	args := []string{"kubectl", "get", "namespace", namespace, "-o", "json"}
	if c.Server != "" {
		args = append(args, fmt.Sprintf("--kubeconfig=%s", kubeconfigFilePath))
	}
	stdout, err := execCommand(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			c.Metrics.UpdateKubectlExitCodeCount(namespace, e.ExitCode())
		}
		return kaa, err
	}
	c.Metrics.UpdateKubectlExitCodeCount(namespace, 0)

	nr := namespaceResponse{}
	if err := json.Unmarshal(stdout, &nr); err != nil {
		return kaa, err
	}

	kaa.Enabled = nr.Metadata.Annotations[enabledAnnotation]
	kaa.DryRun = nr.Metadata.Annotations[dryRunAnnotation]
	kaa.Prune = nr.Metadata.Annotations[pruneAnnotation]

	return kaa, nil
}

// NamespaceAnnotationsBatch returns the kube-applier annotations for a list of namespaces as a map
func (c *Client) NamespaceAnnotationsBatch(namespaces []string) (map[string]KAAnnotations, error) {
	kaaMap := map[string]KAAnnotations{}

	if len(namespaces) == 1 {
		kaa, err := c.NamespaceAnnotations(namespaces[0])
		if err != nil {
			return kaaMap, err
		}
		kaaMap[namespaces[0]] = kaa

		return kaaMap, nil
	} else if len(namespaces) > 1 {
		args := []string{"kubectl", "get", "namespace", "-o", "json"}
		args = append(args, namespaces...)
		if c.Server != "" {
			args = append(args, fmt.Sprintf("--kubeconfig=%s", kubeconfigFilePath))
		}

		cmd := execCommand(args[0], args[1:]...)
		stdout, err := cmd.Output()
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				c.Metrics.UpdateKubectlExitCodeCount("", e.ExitCode())
			}
			return kaaMap, err
		}
		c.Metrics.UpdateKubectlExitCodeCount("", 0)

		var nl namespaceResponseList
		if err := json.Unmarshal(stdout, &nl); err != nil {
			return kaaMap, err
		}

		for _, n := range nl.Items {
			kaaMap[n.Metadata.Name] = KAAnnotations{
				Enabled: n.Metadata.Annotations[enabledAnnotation],
				DryRun:  n.Metadata.Annotations[dryRunAnnotation],
				Prune:   n.Metadata.Annotations[pruneAnnotation],
			}
		}
	}

	return kaaMap, nil
}
