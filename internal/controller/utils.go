package controller

import (
	"fmt"
	"reflect"
	"strings"

	hapi "github.com/hstreamdb/hstream-operator/api/v1alpha2"
	"github.com/hstreamdb/hstream-operator/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// structAssign copy the value of struct from src to dist.
func structAssign(dist, src interface{}) {
	distVal := reflect.ValueOf(dist).Elem()
	srcVal := reflect.ValueOf(src).Elem()
	srcType := srcVal.Type()

	for i := 0; i < srcVal.NumField(); i++ {
		if srcVal.Field(i).IsZero() {
			continue
		}

		// Check if the dist has the same field.
		name := srcType.Field(i).Name
		distValField := distVal.FieldByName(name)
		if ok := distValField.IsValid(); !ok {
			continue
		}

		distValField.Set(srcVal.Field(i))
	}
}

// mergeLabels merges the labels specified by the operator into
// on object's metadata.
//
// This will return whether the target's labels have changed.
func mergeLabelsInMetadata(target *metav1.ObjectMeta, desired metav1.ObjectMeta) bool {
	return mergeMap(target.Labels, desired.Labels)
}

// mergeAnnotations merges the annotations specified by the operator into
// on object's metadata.
//
// This will return whether the target's annotations have changed.
func mergeAnnotations(target *metav1.ObjectMeta, desired metav1.ObjectMeta) bool {
	return mergeMap(target.Annotations, desired.Annotations)
}

// mergeMap merges a map into another map.
//
// This will return whether the target's values have changed.
func mergeMap(target map[string]string, desired map[string]string) bool {
	changed := false
	for key, value := range desired {
		if target[key] != value {
			target[key] = value
			changed = true
		}
	}
	return changed
}

// extendEnv adds environment variables to an existing environment, unless
// environment variables with the same name are already present.
func extendEnvs(envs []corev1.EnvVar, externalEnvs ...corev1.EnvVar) []corev1.EnvVar {
	existingVars := make(map[string]struct{}, len(envs))

	for _, envVar := range envs {
		existingVars[envVar.Name] = struct{}{}
	}

	for _, envVar := range externalEnvs {
		if _, ok := existingVars[envVar.Name]; !ok {
			envs = append(envs, envVar)
		}
	}
	return envs
}

func extendArgs(args []string, externalArgs ...string) ([]string, error) {
	containerArgFlags := internal.FlagSet{}
	err := containerArgFlags.Parse(args)
	if err != nil {
		return nil, err
	}

	existKeys := make(map[string]struct{}, len(containerArgFlags.Flags()))
	for flag := range containerArgFlags.Flags() {
		key := strings.TrimLeft(flag, "-")
		if flag == "-p" {
			key = "port"
		}
		existKeys[key] = struct{}{}
	}

	externalArgFlags := internal.FlagSet{}
	_ = externalArgFlags.Parse(externalArgs)

	for flag, value := range externalArgFlags.Flags() {
		key := strings.TrimLeft(flag, "-")
		if _, ok := existKeys[key]; !ok {
			containerArgFlags.Flags()[flag] = value
		}
	}

	mergedArgs := make([]string, 0, len(existKeys)*2)
	containerArgFlags.Visit(func(flag, value string) {
		mergedArgs = append(mergedArgs, flag, value)
	})
	return mergedArgs, nil
}

func extendPorts(ports []corev1.ContainerPort, externalPorts ...corev1.ContainerPort) []corev1.ContainerPort {
	for i := range externalPorts {
		found := false
		for j := range ports {
			if (&ports[j]).Name == (&externalPorts[i]).Name {
				found = true
				break
			}
		}
		if !found {
			if externalPorts[i].Name == "" {
				(&externalPorts[i]).Name = fmt.Sprintf("unset-%d", (&externalPorts[i]).ContainerPort)
			}
			ports = append(ports, externalPorts[i])
		}
	}
	return ports
}

// coverPortsFromArgs use the port in user-defined args to cover the default port
func coverPortsFromArgs(args []string, ports []corev1.ContainerPort) []corev1.ContainerPort {
	newPorts := make([]corev1.ContainerPort, len(ports))
	copy(newPorts, ports)

	flags := internal.FlagSet{}
	_ = flags.Parse(args)
	parsedArgs := flags.Flags()

	for i := range ports {
		name := ports[i].Name

		if port, ok := parsedArgs["--"+name]; ok {
			newPorts[i].ContainerPort = intstr.Parse(port).IntVal
		}
	}

	return newPorts
}

func isHashChanged(obj1, obj2 *metav1.ObjectMeta) bool {
	return obj1.Annotations[hapi.LastSpecKey] != obj2.Annotations[hapi.LastSpecKey]
}
