/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/controllers/counter"
	"github.com/aws/karpenter-core/pkg/controllers/deprovisioning"
	"github.com/aws/karpenter-core/pkg/controllers/inflightchecks"
	"github.com/aws/karpenter-core/pkg/controllers/machine/terminator"
	metricspod "github.com/aws/karpenter-core/pkg/controllers/metrics/pod"
	metricsprovisioner "github.com/aws/karpenter-core/pkg/controllers/metrics/provisioner"
	metricsstate "github.com/aws/karpenter-core/pkg/controllers/metrics/state"
	"github.com/aws/karpenter-core/pkg/controllers/node"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/controllers/state/informer"
	"github.com/aws/karpenter-core/pkg/controllers/termination"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/metrics"
	"github.com/aws/karpenter-core/pkg/operator/controller"
)

func init() {
	metrics.MustRegister() // Registers cross-controller metrics
}

func NewControllers(
	ctx context.Context,
	clock clock.Clock,
	kubeClient client.Client,
	kubernetesInterface kubernetes.Interface,
	cluster *state.Cluster,
	recorder events.Recorder,
	cloudProvider cloudprovider.CloudProvider,
) []controller.Controller {

	provisioner := provisioning.NewProvisioner(ctx, kubeClient, kubernetesInterface.CoreV1(), recorder, cloudProvider, cluster)
	terminator := terminator.NewTerminator(clock, kubeClient, cloudProvider, terminator.NewEvictionQueue(ctx, kubernetesInterface.CoreV1(), recorder))
	return []controller.Controller{
		provisioner,
		metricsstate.NewController(cluster),
		deprovisioning.NewController(clock, kubeClient, provisioner, cloudProvider, recorder, cluster),
		provisioning.NewController(kubeClient, provisioner, recorder),
		informer.NewNodeController(kubeClient, cluster),
		informer.NewPodController(kubeClient, cluster),
		informer.NewProvisionerController(kubeClient, cluster),
		node.NewController(clock, kubeClient, cloudProvider, cluster),
		termination.NewController(kubeClient, terminator, recorder),
		metricspod.NewController(kubeClient),
		metricsprovisioner.NewController(kubeClient),
		counter.NewController(kubeClient, cluster),
		inflightchecks.NewController(clock, kubeClient, recorder, cloudProvider),
	}
}
