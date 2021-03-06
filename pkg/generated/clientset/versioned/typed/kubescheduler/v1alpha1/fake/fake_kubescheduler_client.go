// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/openshift/cluster-kube-scheduler-operator/pkg/generated/clientset/versioned/typed/kubescheduler/v1alpha1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeKubeschedulerV1alpha1 struct {
	*testing.Fake
}

func (c *FakeKubeschedulerV1alpha1) KubeSchedulerOperatorConfigs() v1alpha1.KubeSchedulerOperatorConfigInterface {
	return &FakeKubeSchedulerOperatorConfigs{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeKubeschedulerV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
