package config

import (
	"fmt"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"kusionstack.io/kpt-kcl-sdk/pkg/edit"
)

const (
	// KCLRunGroup represents the API group for the KCLRun resource.
	KCLRunGroup = "fn.kpt.dev"

	// KCLRunVersion represents the API version for the KCLRun resource.
	KCLRunVersion = "v1alpha1"

	// KCLRunAPIVersion is a combination of the API group and version for the KCLRun resource.
	KCLRunAPIVersion = KCLRunGroup + "/" + KCLRunVersion

	// KCLRunKind represents the kind of resource for the KCLRun resource.
	KCLRunKind = "KCLRun"

	// ConfigMapAPIVersion represents the API version for the ConfigMap resource.
	ConfigMapAPIVersion = "v1"

	// ConfigMapKind represents the kind of resource for the ConfigMap resource.
	ConfigMapKind = "ConfigMap"

	// SourceKey is the key for the source field in a ConfigMap.
	SourceKey = "source"

	// defaultProgramName is the default name for the KCL function program.
	defaultProgramName = "kcl-function-run"
)

// KCLRun is a custom resource to provider KPT `functionConfig`, KCL source and params.
type KCLRun struct {
	yaml.ResourceMeta `json:",inline" yaml:",inline"`
	// Source is a required field for providing a KCL script inline.
	Source string `json:"source" yaml:"source"`
	// Params are the parameters in key-value pairs format.
	Params map[string]interface{} `json:"params,omitempty" yaml:"params,omitempty"`
}

func (r *KCLRun) Config(fnCfg *fn.KubeObject) error {
	fnCfgKind := fnCfg.GetKind()
	fnCfgAPIVersion := fnCfg.GetAPIVersion()
	switch {
	case fnCfg.IsEmpty():
		return fmt.Errorf("FunctionConfig is missing. Expect `ConfigMap` or `KCLRun`")
	case fnCfgAPIVersion == ConfigMapAPIVersion && fnCfgKind == ConfigMapKind:
		cm := &corev1.ConfigMap{}
		if err := fnCfg.As(cm); err != nil {
			return err
		}
		// Convert ConfigMap to KCLRun
		r.Name = cm.Name
		r.Namespace = cm.Namespace
		r.Params = map[string]interface{}{}
		for k, v := range cm.Data {
			if k == SourceKey {
				r.Source = v
			}
			r.Params[k] = v
		}
	case fnCfgAPIVersion == KCLRunAPIVersion && fnCfgKind == KCLRunKind:
		if err := fnCfg.As(r); err != nil {
			return err
		}
	default:
		return fmt.Errorf("`functionConfig` must be either %v or %v, but we got: %v",
			schema.FromAPIVersionAndKind(ConfigMapAPIVersion, ConfigMapKind).String(),
			schema.FromAPIVersionAndKind(KCLRunAPIVersion, KCLRunKind).String(),
			schema.FromAPIVersionAndKind(fnCfg.GetAPIVersion(), fnCfg.GetKind()).String())
	}

	// Defaulting
	if r.Name == "" {
		r.Name = defaultProgramName
	}
	// Validation
	if r.Source == "" {
		return fmt.Errorf("`source` must not be empty")
	}
	return nil
}

func (r *KCLRun) Transform(rl *fn.ResourceList) error {
	var transformedObjects []*fn.KubeObject
	var nodes []*yaml.RNode

	fcRN, err := yaml.Parse(rl.FunctionConfig.String())
	if err != nil {
		return err
	}
	for _, obj := range rl.Items {
		objRN, err := yaml.Parse(obj.String())
		if err != nil {
			return err
		}
		nodes = append(nodes, objRN)
	}

	st := &edit.SimpleTransformer{
		Name:           r.Name,
		Source:         r.Source,
		FunctionConfig: fcRN,
	}
	transformedNodes, err := st.Transform(nodes)
	if err != nil {
		return err
	}
	for _, n := range transformedNodes {
		obj, err := fn.ParseKubeObject([]byte(n.MustString()))
		if err != nil {
			return err
		}
		transformedObjects = append(transformedObjects, obj)
	}
	rl.Items = transformedObjects
	return nil
}
