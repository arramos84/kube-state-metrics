/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kube-event-exporter/pkg/metric"
	generator "k8s.io/kube-event-exporter/pkg/metric_generator"

	v1 "k8s.io/api/core/v1"
)

var (
	configMapMetricFamilies = []generator.FamilyGenerator{
		{
			Name: "kube_configmap_events",
			Type: metric.Gauge,
			Help: "ConfigMap events.",
			GenerateFunc: wrapConfigMapEventFunc(func(e *v1.Event) *metric.Family {
				m := metric.Metric{
					LabelKeys:   []string{"namespace", "configmap", "reason", "type", "message"},
					LabelValues: []string{e.InvolvedObject.Namespace, e.InvolvedObject.Name, e.Reason, e.Type, e.Message},
					Value:       1,
				}

				return &metric.Family{
					Metrics: []*metric.Metric{&m},
				}
			}),
		},
	}
)

func wrapConfigMapEventFunc(f func(e *v1.Event) *metric.Family) func(interface{}) *metric.Family {
	return func(obj interface{}) *metric.Family {
		event := obj.(*v1.Event)
		metricFamily := f(event)

		return metricFamily
	}
}

func createConfigMapEventListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			opts = metav1.ListOptions{
				FieldSelector: "involvedObject.kind=ConfigMap",
			}
			return kubeClient.CoreV1().Events(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			opts = metav1.ListOptions{
				FieldSelector: "involvedObject.kind=ConfigMap",
			}
			return kubeClient.CoreV1().Events(ns).Watch(opts)
		},
	}
}


