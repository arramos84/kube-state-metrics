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
	persistentVolumeClaimMetricFamilies = []generator.FamilyGenerator{
		{
			Name: "kube_persistentvolumeclaim_events",
			Type: metric.Gauge,
			Help: "PersistentVolumeClaim events.",
			GenerateFunc: wrapPersistentVolumeClaimEventFunc(func(e *v1.Event) *metric.Family {
				alertCategory := "amend"
				if e.Type != "Normal" {
					alertCategory = "failure"
				}

				m := metric.Metric{
					LabelKeys:   []string{"namespace", "persistentvolumeclaim", "reason", "type", "message", "asserts_entity_type", "asserts_alert_type", "asserts_alert_category"},
					LabelValues: []string{e.InvolvedObject.Namespace, e.InvolvedObject.Name, e.Reason, e.Type, e.Message, "PersistentVolumeClaim", "cause", alertCategory},
					Value:       1,
				}

				return &metric.Family{
					Metrics: []*metric.Metric{&m},
				}
			}),
		},
	}
)

func wrapPersistentVolumeClaimEventFunc(f func(e *v1.Event) *metric.Family) func(interface{}) *metric.Family {
	return func(obj interface{}) *metric.Family {
		event := obj.(*v1.Event)
		metricFamily := f(event)

		return metricFamily
	}
}

func createPersistentVolumeClaimEventListWatch(kubeClient clientset.Interface, ns string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			opts = metav1.ListOptions{
				FieldSelector: "involvedObject.kind=PersistentVolumeClaim",
			}
			return kubeClient.CoreV1().Events(ns).List(opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			opts = metav1.ListOptions{
				FieldSelector: "involvedObject.kind=PersistentVolumeClaim",
			}
			return kubeClient.CoreV1().Events(ns).Watch(opts)
		},
	}
}


