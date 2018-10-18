package operator

import (
	"fmt"

	"github.com/openshift/cluster-kube-scheduler-operator/pkg/apis/kubescheduler/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/cluster-kube-scheduler-operator/pkg/operator/v311_00_assets"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// syncKubeScheduler_v311_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func syncKubeScheduler_v311_00_to_latest(c KubeSchedulerOperator, operatorConfig *v1alpha1.KubeSchedulerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailability) (operatorsv1alpha1.VersionAvailability, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailability{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/ns.yaml"))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, requiredNamespace)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "ns", err))
	}

	requiredPublicRole := resourceread.ReadClusterRoleV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/public-info-role.yaml"))
	_, _, err = resourceapply.ApplyClusterRole(c.rbacv1Client, requiredPublicRole)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "clusterrole", err))
	}

	requiredPublicRoleBinding := resourceread.ReadClusterRoleBindingV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/public-info-rolebinding.yaml"))
	_, _, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, requiredPublicRoleBinding)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "clusterrolebinding1", err))
	}

	requiredPublicRoleBinding2 := resourceread.ReadClusterRoleBindingV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/scheduler-clusterrolebinding.yaml"))
	_, _, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, requiredPublicRoleBinding2)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "clusterrolebinding2", err))
	}

	requiredService := resourceread.ReadServiceV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/svc.yaml"))
	_, _, err = resourceapply.ApplyService(c.corev1Client, requiredService)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "svc", err))
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/sa.yaml"))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, requiredSA)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "sa", err))
	}

	schedulerConfig, configMapModified, err := manageKubeSchedulerConfigMap_v311_00_to_latest(c.corev1Client, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "configmap", err))
	}

	forceDeployment := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if configMapModified {
		forceDeployment = true
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := manageKubeSchedulerDeployment_v311_00_to_latest(c.appsv1Client, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "deployment", err))
	}

	configData := ""
	if schedulerConfig != nil {
		configData = schedulerConfig.Data["config.yaml"]
	}
	_, _, err = manageKubeSchedulerPublicConfigMap_v311_00_to_latest(c.corev1Client, configData, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "configmap/public-info", err))
	}

	return resourcemerge.ApplyDeploymentGenerationAvailability(versionAvailability, actualDeployment, errors...), errors
}

func manageKubeSchedulerConfigMap_v311_00_to_latest(client coreclientv1.ConfigMapsGetter, operatorConfig *v1alpha1.KubeSchedulerOperatorConfig) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/cm.yaml"))
	defaultConfig := v311_00_assets.MustAsset("v3.11.0/kube-scheduler/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "config.yaml", nil, defaultConfig, operatorConfig.Spec.KubeSchedulerConfig.Raw)
	if err != nil {
		return nil, false, err
	}
	return resourceapply.ApplyConfigMap(client, requiredConfigMap)
}

func manageKubeSchedulerDeployment_v311_00_to_latest(client appsclientv1.DeploymentsGetter, options *v1alpha1.KubeSchedulerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailability, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.Spec.Logging.Level))

	return resourceapply.ApplyDeployment(client, required, resourcemerge.ExpectedDeploymentGeneration(required, previousAvailability), forceDeployment)
}

func manageKubeSchedulerPublicConfigMap_v311_00_to_latest(client coreclientv1.ConfigMapsGetter, schedulerConfigString string, operatorConfig *v1alpha1.KubeSchedulerOperatorConfig) (*corev1.ConfigMap, bool, error) {
	uncastUnstructured, err := runtime.Decode(unstructured.UnstructuredJSONScheme, []byte(schedulerConfigString))
	if err != nil {
		return nil, false, err
	}
	schedulerConfig := uncastUnstructured.(runtime.Unstructured)

	configMap := resourceread.ReadConfigMapV1OrDie(v311_00_assets.MustAsset("v3.11.0/kube-scheduler/public-info.yaml"))
	if operatorConfig.Status.CurrentAvailability != nil {
		configMap.Data["version"] = operatorConfig.Status.CurrentAvailability.Version
	} else {
		configMap.Data["version"] = ""
	}
	configMap.Data["imagePolicyConfig.internalRegistryHostname"], _, err = unstructured.NestedString(schedulerConfig.UnstructuredContent(), "imagePolicyConfig", "internalRegistryHostname")
	if err != nil {
		return nil, false, err
	}
	configMap.Data["imagePolicyConfig.externalRegistryHostname"], _, err = unstructured.NestedString(schedulerConfig.UnstructuredContent(), "imagePolicyConfig", "externalRegistryHostname")
	if err != nil {
		return nil, false, err
	}
	configMap.Data["projectConfig.defaultNodeSelector"], _, err = unstructured.NestedString(schedulerConfig.UnstructuredContent(), "projectConfig", "defaultNodeSelector")
	if err != nil {
		return nil, false, err
	}

	return resourceapply.ApplyConfigMap(client, configMap)
}
