/*
Copyright 2018 The Kubernetes Authors All rights reserved.

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

package store

import (
	"context"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	vpaclientset "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	ksmtypes "k8s.io/kube-event-exporter/pkg/builder/types"
	"k8s.io/kube-event-exporter/pkg/listwatch"
	generator "k8s.io/kube-event-exporter/pkg/metric_generator"
	metricsstore "k8s.io/kube-event-exporter/pkg/metrics_store"
	"k8s.io/kube-event-exporter/pkg/options"
	"k8s.io/kube-event-exporter/pkg/sharding"
	"k8s.io/kube-event-exporter/pkg/watch"
)

// Builder helps to build store. It follows the builder pattern
// (https://en.wikipedia.org/wiki/Builder_pattern).
type Builder struct {
	kubeClient       clientset.Interface
	vpaClient        vpaclientset.Interface
	namespaces       options.NamespaceList
	ctx              context.Context
	enabledResources []string
	allowDenyList    ksmtypes.AllowDenyLister
	metrics          *watch.ListWatchMetrics
	shard            int32
	totalShards      int
	buildStoreFunc   ksmtypes.BuildStoreFunc
}

// NewBuilder returns a new builder.
func NewBuilder() *Builder {
	b := &Builder{}
	return b
}

// WithMetrics sets the metrics property of a Builder.
func (b *Builder) WithMetrics(r *prometheus.Registry) {
	b.metrics = watch.NewListWatchMetrics(r)
}

// WithEnabledResources sets the enabledResources property of a Builder.
func (b *Builder) WithEnabledResources(r []string) error {
	for _, col := range r {
		if !resourceExists(col) {
			return errors.Errorf("resource %s does not exist. Available resources: %s", col, strings.Join(availableResources(), ","))
		}
	}

	var copy []string
	copy = append(copy, r...)

	sort.Strings(copy)

	b.enabledResources = copy
	return nil
}

// WithNamespaces sets the namespaces property of a Builder.
func (b *Builder) WithNamespaces(n options.NamespaceList) {
	b.namespaces = n
}

// WithSharding sets the shard and totalShards property of a Builder.
func (b *Builder) WithSharding(shard int32, totalShards int) {
	b.shard = shard
	b.totalShards = totalShards
}

// WithContext sets the ctx property of a Builder.
func (b *Builder) WithContext(ctx context.Context) {
	b.ctx = ctx
}

// WithKubeClient sets the kubeClient property of a Builder.
func (b *Builder) WithKubeClient(c clientset.Interface) {
	b.kubeClient = c
}

// WithVPAClient sets the vpaClient property of a Builder so that the verticalpodautoscaler collector can query VPA objects.
func (b *Builder) WithVPAClient(c vpaclientset.Interface) {
	b.vpaClient = c
}

// WithAllowDenyList configures the allow or denylisted metric to be exposed
// by the store build by the Builder.
func (b *Builder) WithAllowDenyList(l ksmtypes.AllowDenyLister) {
	b.allowDenyList = l
}

// WithGenerateStoreFunc configures a constom generate store function
func (b *Builder) WithGenerateStoreFunc(f ksmtypes.BuildStoreFunc) {
	b.buildStoreFunc = f
}

// DefaultGenerateStoreFunc returns default buildStore function
func (b *Builder) DefaultGenerateStoreFunc() ksmtypes.BuildStoreFunc {
	return b.buildStore
}

// Build initializes and registers all enabled stores.
func (b *Builder) Build() []cache.Store {
	if b.allowDenyList == nil {
		panic("allowDenyList should not be nil")
	}

	stores := []cache.Store{}
	activeStoreNames := []string{}

	for _, c := range b.enabledResources {
		constructor, ok := availableStores[c]
		if ok {
			store := constructor(b)
			activeStoreNames = append(activeStoreNames, c)
			stores = append(stores, store)
		}
	}

	klog.Infof("Active resources: %s", strings.Join(activeStoreNames, ","))

	return stores
}

var availableStores = map[string]func(f *Builder) cache.Store{
	"configmaps":                      func(b *Builder) cache.Store { return b.buildConfigMapStore() },
	"daemonsets":                      func(b *Builder) cache.Store { return b.buildDaemonSetStore() },
	"deployments":                     func(b *Builder) cache.Store { return b.buildDeploymentStore() },
	"replicasets":                     func(b *Builder) cache.Store { return b.buildReplicaSetStore() },
	//"namespaces":                      func(b *Builder) cache.Store { return b.buildNamespaceStore() },
	"nodes":                           func(b *Builder) cache.Store { return b.buildNodeStore() },
	"persistentvolumeclaims":          func(b *Builder) cache.Store { return b.buildPersistentVolumeClaimStore() },
	"pods":                            func(b *Builder) cache.Store { return b.buildPodStore() },
	"statefulsets":                    func(b *Builder) cache.Store { return b.buildStatefulSetStore() },
	"services":                        func(b *Builder) cache.Store { return b.buildServiceStore() },
}

func resourceExists(name string) bool {
	_, ok := availableStores[name]
	return ok
}

func availableResources() []string {
	c := []string{}
	for name := range availableStores {
		c = append(c, name)
	}
	return c
}

func (b *Builder) buildPodStore() cache.Store {
	return b.buildStoreFunc(podMetricFamilies, &v1.Event{}, createPodEventListWatch)
}

func (b *Builder) buildNodeStore() cache.Store {
	return b.buildStoreFunc(nodeMetricFamilies, &v1.Event{}, createNodeEventListWatch)
}

func (b *Builder) buildConfigMapStore() cache.Store {
	return b.buildStoreFunc(configMapMetricFamilies, &v1.Event{}, createConfigMapEventListWatch)
}

func (b *Builder) buildDeploymentStore() cache.Store {
	return b.buildStoreFunc(deploymentMetricFamilies, &v1.Event{}, createDeploymentEventListWatch)
}

func (b *Builder) buildDaemonSetStore() cache.Store {
	return b.buildStoreFunc(daemonSetMetricFamilies, &v1.Event{}, createDaemonSetEventListWatch)
}

func (b *Builder) buildReplicaSetStore() cache.Store {
	return b.buildStoreFunc(replicaSetMetricFamilies, &v1.Event{}, createReplicaSetEventListWatch)
}

func (b *Builder) buildServiceStore() cache.Store {
	return b.buildStoreFunc(serviceMetricFamilies, &v1.Event{}, createServiceEventListWatch)
}

func (b *Builder) buildStatefulSetStore() cache.Store {
	return b.buildStoreFunc(statefulSetMetricFamilies, &v1.Event{}, createStatefulSetEventListWatch)
}

func (b *Builder) buildPersistentVolumeClaimStore() cache.Store {
	return b.buildStoreFunc(persistentVolumeClaimMetricFamilies, &v1.Event{}, createPersistentVolumeClaimEventListWatch)
}

func (b *Builder) buildStore(
	metricFamilies []generator.FamilyGenerator,
	expectedType interface{},
	listWatchFunc func(kubeClient clientset.Interface, ns string) cache.ListerWatcher,
) cache.Store {
	filteredMetricFamilies := generator.FilterMetricFamilies(b.allowDenyList, metricFamilies)
	composedMetricGenFuncs := generator.ComposeMetricGenFuncs(filteredMetricFamilies)

	familyHeaders := generator.ExtractMetricFamilyHeaders(filteredMetricFamilies)

	store := metricsstore.NewMetricsStore(
		familyHeaders,
		composedMetricGenFuncs,
	)
	b.reflectorPerNamespace(expectedType, store, listWatchFunc)

	return store
}

// reflectorPerNamespace creates a Kubernetes client-go reflector with the given
// listWatchFunc for each given namespace and registers it with the given store.
func (b *Builder) reflectorPerNamespace(
	expectedType interface{},
	store cache.Store,
	listWatchFunc func(kubeClient clientset.Interface, ns string) cache.ListerWatcher,
) {
	lwf := func(ns string) cache.ListerWatcher { return listWatchFunc(b.kubeClient, ns) }
	lw := listwatch.MultiNamespaceListerWatcher(b.namespaces, nil, lwf)
	instrumentedListWatch := watch.NewInstrumentedListerWatcher(lw, b.metrics, reflect.TypeOf(expectedType).String())
	reflector := cache.NewReflector(sharding.NewShardedListWatch(b.shard, b.totalShards, instrumentedListWatch), expectedType, store, 0)
	go reflector.Run(b.ctx.Done())
}
