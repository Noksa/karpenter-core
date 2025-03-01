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

package deprovisioning

import (
	"context"
	"fmt"

	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	"github.com/aws/karpenter-core/pkg/controllers/state"
)

// SingleNodeConsolidation is the consolidation controller that performs single node consolidation.
type SingleNodeConsolidation struct {
	consolidation
}

func NewSingleNodeConsolidation(clk clock.Clock, cluster *state.Cluster, kubeClient client.Client, provisioner *provisioning.Provisioner,
	cp cloudprovider.CloudProvider, reporter *Reporter) *SingleNodeConsolidation {
	return &SingleNodeConsolidation{consolidation: makeConsolidation(clk, cluster, kubeClient, provisioner, cp, reporter)}
}

// ComputeCommand generates a deprovisioning command given deprovisionable nodes
//
//nolint:gocyclo
func (c *SingleNodeConsolidation) ComputeCommand(ctx context.Context, candidates ...CandidateNode) (Command, error) {
	if c.cluster.Consolidated() {
		return Command{action: actionDoNothing}, nil
	}
	candidates, err := c.sortAndFilterCandidates(ctx, candidates)
	if err != nil {
		return Command{}, fmt.Errorf("sorting candidates, %w", err)
	}

	v := NewValidation(consolidationTTL, c.clock, c.cluster, c.kubeClient, c.provisioner, c.cloudProvider)
	var failedValidation bool
	for _, node := range candidates {
		// compute a possible consolidation option
		cmd, err := c.computeConsolidation(ctx, node)
		if err != nil {
			logging.FromContext(ctx).Errorf("computing consolidation %s", err)
			continue
		}
		if cmd.action == actionDoNothing || cmd.action == actionRetry {
			continue
		}

		isValid, err := v.IsValid(ctx, cmd)
		if err != nil {
			logging.FromContext(ctx).Errorf("validating consolidation %s", err)
			continue
		}
		if !isValid {
			failedValidation = true
			continue
		}

		if cmd.action == actionReplace || cmd.action == actionDelete {
			return cmd, nil
		}
	}

	// we failed validation, so we need to retry
	if failedValidation {
		return Command{action: actionRetry}, nil
	}
	return Command{action: actionDoNothing}, nil
}
