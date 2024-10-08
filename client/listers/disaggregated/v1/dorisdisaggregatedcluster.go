/*
Copyright 2023.

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
// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/apache/doris-operator/api/disaggregated/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// DorisDisaggregatedClusterLister helps list DorisDisaggregatedClusters.
// All objects returned here must be treated as read-only.
type DorisDisaggregatedClusterLister interface {
	// List lists all DorisDisaggregatedClusters in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.DorisDisaggregatedCluster, err error)
	// DorisDisaggregatedClusters returns an object that can list and get DorisDisaggregatedClusters.
	DorisDisaggregatedClusters(namespace string) DorisDisaggregatedClusterNamespaceLister
	DorisDisaggregatedClusterListerExpansion
}

// dorisDisaggregatedClusterLister implements the DorisDisaggregatedClusterLister interface.
type dorisDisaggregatedClusterLister struct {
	indexer cache.Indexer
}

// NewDorisDisaggregatedClusterLister returns a new DorisDisaggregatedClusterLister.
func NewDorisDisaggregatedClusterLister(indexer cache.Indexer) DorisDisaggregatedClusterLister {
	return &dorisDisaggregatedClusterLister{indexer: indexer}
}

// List lists all DorisDisaggregatedClusters in the indexer.
func (s *dorisDisaggregatedClusterLister) List(selector labels.Selector) (ret []*v1.DorisDisaggregatedCluster, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.DorisDisaggregatedCluster))
	})
	return ret, err
}

// DorisDisaggregatedClusters returns an object that can list and get DorisDisaggregatedClusters.
func (s *dorisDisaggregatedClusterLister) DorisDisaggregatedClusters(namespace string) DorisDisaggregatedClusterNamespaceLister {
	return dorisDisaggregatedClusterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// DorisDisaggregatedClusterNamespaceLister helps list and get DorisDisaggregatedClusters.
// All objects returned here must be treated as read-only.
type DorisDisaggregatedClusterNamespaceLister interface {
	// List lists all DorisDisaggregatedClusters in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.DorisDisaggregatedCluster, err error)
	// Get retrieves the DorisDisaggregatedCluster from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.DorisDisaggregatedCluster, error)
	DorisDisaggregatedClusterNamespaceListerExpansion
}

// dorisDisaggregatedClusterNamespaceLister implements the DorisDisaggregatedClusterNamespaceLister
// interface.
type dorisDisaggregatedClusterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all DorisDisaggregatedClusters in the indexer for a given namespace.
func (s dorisDisaggregatedClusterNamespaceLister) List(selector labels.Selector) (ret []*v1.DorisDisaggregatedCluster, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.DorisDisaggregatedCluster))
	})
	return ret, err
}

// Get retrieves the DorisDisaggregatedCluster from the indexer for a given namespace and name.
func (s dorisDisaggregatedClusterNamespaceLister) Get(name string) (*v1.DorisDisaggregatedCluster, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("dorisdisaggregatedcluster"), name)
	}
	return obj.(*v1.DorisDisaggregatedCluster), nil
}
