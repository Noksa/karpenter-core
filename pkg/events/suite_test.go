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

package events_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/util/flowcontrol"

	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/test"
)

var eventRecorder events.Recorder
var internalRecorder *InternalRecorder

type InternalRecorder struct {
	mu    sync.RWMutex
	calls map[string]int
}

func NewInternalRecorder() *InternalRecorder {
	return &InternalRecorder{
		calls: map[string]int{},
	}
}

func (i *InternalRecorder) Event(_ runtime.Object, _, reason, _ string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls[reason]++
}

func (i *InternalRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, _ ...interface{}) {
	i.Event(object, eventtype, reason, messageFmt)
}

func (i *InternalRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventtype, reason, messageFmt string, _ ...interface{}) {
	i.Event(object, eventtype, reason, messageFmt)
}

func (i *InternalRecorder) Calls(reason string) int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.calls[reason]
}

func TestRecorder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventRecorder")
}

var _ = BeforeEach(func() {
	internalRecorder = NewInternalRecorder()
	eventRecorder = events.NewRecorder(internalRecorder)
	events.PodNominationRateLimiter = flowcontrol.NewTokenBucketRateLimiter(5, 10)
})

var _ = Describe("Event Creation", func() {
	It("should create a NominatePod event", func() {
		eventRecorder.Publish(events.NominatePod(PodWithUID(), NodeWithUID()))
		Expect(internalRecorder.Calls(events.NominatePod(PodWithUID(), NodeWithUID()).Reason)).To(Equal(1))
	})
	It("should create a EvictPod event", func() {
		eventRecorder.Publish(events.EvictPod(PodWithUID()))
		Expect(internalRecorder.Calls(events.EvictPod(PodWithUID()).Reason)).To(Equal(1))
	})
	It("should create a PodFailedToSchedule event", func() {
		eventRecorder.Publish(events.PodFailedToSchedule(PodWithUID(), fmt.Errorf("")))
		Expect(internalRecorder.Calls(events.PodFailedToSchedule(PodWithUID(), fmt.Errorf("")).Reason)).To(Equal(1))
	})
	It("should create a NodeFailedToDrain event", func() {
		eventRecorder.Publish(events.NodeFailedToDrain(NodeWithUID(), fmt.Errorf("")))
		Expect(internalRecorder.Calls(events.NodeFailedToDrain(NodeWithUID(), fmt.Errorf("")).Reason)).To(Equal(1))
	})
})

var _ = Describe("Dedupe", func() {
	It("should only create a single event when many events are created quickly", func() {
		pod := PodWithUID()
		for i := 0; i < 100; i++ {
			eventRecorder.Publish(events.EvictPod(pod))
		}
		Expect(internalRecorder.Calls(events.EvictPod(PodWithUID()).Reason)).To(Equal(1))
	})
	It("should allow events with different entities to be created", func() {
		for i := 0; i < 100; i++ {
			eventRecorder.Publish(events.EvictPod(PodWithUID()))
		}
		Expect(internalRecorder.Calls(events.EvictPod(PodWithUID()).Reason)).To(Equal(100))
	})
})

var _ = Describe("Rate Limiting", func() {
	It("should only create max-burst when many events are created quickly", func() {
		for i := 0; i < 100; i++ {
			eventRecorder.Publish(events.NominatePod(PodWithUID(), NodeWithUID()))
		}
		Expect(internalRecorder.Calls(events.NominatePod(PodWithUID(), NodeWithUID()).Reason)).To(Equal(10))
	})
	It("should allow many events over time due to smoothed rate limiting", func() {
		for i := 0; i < 3; i++ {
			for j := 0; j < 5; j++ {
				eventRecorder.Publish(events.NominatePod(PodWithUID(), NodeWithUID()))
			}
			time.Sleep(time.Second)
		}
		Expect(internalRecorder.Calls(events.NominatePod(PodWithUID(), NodeWithUID()).Reason)).To(Equal(15))
	})
})

func PodWithUID() *v1.Pod {
	p := test.Pod()
	p.UID = uuid.NewUUID()
	return p
}

func NodeWithUID() *v1.Node {
	n := test.Node()
	n.UID = uuid.NewUUID()
	return n
}
