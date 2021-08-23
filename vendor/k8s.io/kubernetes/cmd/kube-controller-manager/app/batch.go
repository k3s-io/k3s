/*
Copyright 2016 The Kubernetes Authors.

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

// Package app implements a server that runs a set of active
// components.  This includes replication controllers, service endpoints and
// nodes.
//
package app

import (
	"fmt"
	"net/http"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/controller/cronjob"
	"k8s.io/kubernetes/pkg/controller/job"
	kubefeatures "k8s.io/kubernetes/pkg/features"
)

func startJobController(ctx ControllerContext) (http.Handler, bool, error) {
	go job.NewController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.InformerFactory.Batch().V1().Jobs(),
		ctx.ClientBuilder.ClientOrDie("job-controller"),
	).Run(int(ctx.ComponentConfig.JobController.ConcurrentJobSyncs), ctx.Stop)
	return nil, true, nil
}

func startCronJobController(ctx ControllerContext) (http.Handler, bool, error) {
	if utilfeature.DefaultFeatureGate.Enabled(kubefeatures.CronJobControllerV2) {
		cj2c, err := cronjob.NewControllerV2(ctx.InformerFactory.Batch().V1().Jobs(),
			ctx.InformerFactory.Batch().V1().CronJobs(),
			ctx.ClientBuilder.ClientOrDie("cronjob-controller"),
		)
		if err != nil {
			return nil, true, fmt.Errorf("error creating CronJob controller V2: %v", err)
		}
		go cj2c.Run(int(ctx.ComponentConfig.CronJobController.ConcurrentCronJobSyncs), ctx.Stop)
		return nil, true, nil
	}
	cjc, err := cronjob.NewController(
		ctx.ClientBuilder.ClientOrDie("cronjob-controller"),
	)
	if err != nil {
		return nil, true, fmt.Errorf("error creating CronJob controller: %v", err)
	}
	go cjc.Run(ctx.Stop)
	return nil, true, nil
}
