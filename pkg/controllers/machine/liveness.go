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

package machine

import (
	"context"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
)

type Liveness struct {
	clock      clock.Clock
	kubeClient client.Client
}

func (l *Liveness) Reconcile(ctx context.Context, machine *v1alpha5.Machine) (reconcile.Result, error) {
	if settings.FromContext(ctx).TTLAfterNotRegistered == nil {
		return reconcile.Result{}, nil
	}
	// Delete the machine if we believe the machine won't register since we haven't seen the node
	if !machine.StatusConditions().GetCondition(v1alpha5.MachineRegistered).IsTrue() {
		if !machine.CreationTimestamp.IsZero() && machine.CreationTimestamp.Add(settings.FromContext(ctx).TTLAfterNotRegistered.Duration).Before(l.clock.Now()) {
			return reconcile.Result{}, client.IgnoreNotFound(l.kubeClient.Delete(ctx, machine))
		}
		return reconcile.Result{RequeueAfter: settings.FromContext(ctx).TTLAfterNotRegistered.Duration}, nil
	}
	return reconcile.Result{}, nil
}
