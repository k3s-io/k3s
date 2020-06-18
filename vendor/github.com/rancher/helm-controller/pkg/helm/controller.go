package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	helmv1 "github.com/rancher/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmcontroller "github.com/rancher/helm-controller/pkg/generated/controllers/helm.cattle.io/v1"
	batchcontroller "github.com/rancher/wrangler-api/pkg/generated/controllers/batch/v1"
	corecontroller "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	rbaccontroller "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/rancher/wrangler/pkg/relatedresource"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	trueVal = true
)

type Controller struct {
	namespace      string
	helmController helmcontroller.HelmChartController
	jobsCache      batchcontroller.JobCache
	apply          apply.Apply
}

const (
	image   = "rancher/klipper-helm:v0.2.7"
	Label   = "helmcharts.helm.cattle.io/chart"
	CRDName = "helmcharts.helm.cattle.io"
	Name    = "helm-controller"
)

func Register(ctx context.Context, apply apply.Apply,
	helms helmcontroller.HelmChartController,
	jobs batchcontroller.JobController,
	crbs rbaccontroller.ClusterRoleBindingController,
	sas corecontroller.ServiceAccountController,
	cm corecontroller.ConfigMapController) {
	apply = apply.WithSetID(Name).
		WithCacheTypes(helms, jobs, crbs, sas, cm).
		WithStrictCaching().WithPatcher(batch.SchemeGroupVersion.WithKind("Job"), func(namespace, name string, pt types.PatchType, data []byte) (runtime.Object, error) {
		err := jobs.Delete(namespace, name, &metav1.DeleteOptions{})
		if err == nil {
			return nil, fmt.Errorf("replace job")
		}
		return nil, err
	})

	relatedresource.Watch(ctx, "helm-pod-watch",
		func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
			if job, ok := obj.(*batch.Job); ok {
				name := job.Labels[Label]
				if name != "" {
					return []relatedresource.Key{
						{
							Name:      name,
							Namespace: namespace,
						},
					}, nil
				}
			}
			return nil, nil
		},
		helms,
		jobs)

	controller := &Controller{
		helmController: helms,
		jobsCache:      jobs.Cache(),
		apply:          apply,
	}

	helms.OnChange(ctx, Name, controller.OnHelmChanged)
	helms.OnRemove(ctx, Name, controller.OnHelmRemove)
}

func (c *Controller) OnHelmChanged(key string, chart *helmv1.HelmChart) (*helmv1.HelmChart, error) {
	if chart == nil {
		return nil, nil
	}

	if chart.Spec.Chart == "" && chart.Spec.ChartContent == "" {
		return chart, nil
	}

	objs := objectset.NewObjectSet()
	job, valuesConfigMap, contentConfigMap := job(chart)
	objs.Add(serviceAccount(chart))
	objs.Add(roleBinding(chart))
	objs.Add(job)
	if valuesConfigMap != nil {
		objs.Add(valuesConfigMap)
	}
	if contentConfigMap != nil {
		objs.Add(contentConfigMap)
	}

	if err := c.apply.WithOwner(chart).Apply(objs); err != nil {
		return chart, err
	}

	chartCopy := chart.DeepCopy()
	chartCopy.Status.JobName = job.Name
	return c.helmController.Update(chartCopy)
}

func (c *Controller) OnHelmRemove(key string, chart *helmv1.HelmChart) (*helmv1.HelmChart, error) {
	if chart.Spec.Chart == "" {
		return chart, nil
	}
	job, _, _ := job(chart)
	job, err := c.jobsCache.Get(chart.Namespace, job.Name)

	if errors.IsNotFound(err) {
		_, err := c.OnHelmChanged(key, chart)
		if err != nil {
			return chart, err
		}
	} else if err != nil {
		return chart, err
	}

	if job.Status.Succeeded <= 0 {
		return chart, fmt.Errorf("waiting for delete of helm chart %s", chart.Name)
	}

	chartCopy := chart.DeepCopy()
	chartCopy.Status.JobName = job.Name
	newChart, err := c.helmController.Update(chartCopy)

	if err != nil {
		return newChart, err
	}

	return newChart, c.apply.WithOwner(newChart).Apply(objectset.NewObjectSet())
}

func job(chart *helmv1.HelmChart) (*batch.Job, *core.ConfigMap, *core.ConfigMap) {
	oneThousand := int32(1000)
	valuesHash := sha256.Sum256([]byte(chart.Spec.ValuesContent))

	action := "install"
	if chart.DeletionTimestamp != nil {
		action = "delete"
	}

	job := &batch.Job{
		TypeMeta: meta.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      fmt.Sprintf("helm-%s-%s", action, chart.Name),
			Namespace: chart.Namespace,
			Labels: map[string]string{
				Label: chart.Name,
			},
		},
		Spec: batch.JobSpec{
			BackoffLimit: &oneThousand,
			Template: core.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: map[string]string{
						Label: chart.Name,
					},
				},
				Spec: core.PodSpec{
					RestartPolicy: core.RestartPolicyOnFailure,
					Containers: []core.Container{
						{
							Name:            "helm",
							Image:           image,
							ImagePullPolicy: core.PullIfNotPresent,
							Args:            args(chart),
							Env: []core.EnvVar{
								{
									Name:  "NAME",
									Value: chart.Name,
								},
								{
									Name:  "VERSION",
									Value: chart.Spec.Version,
								},
								{
									Name:  "REPO",
									Value: chart.Spec.Repo,
								},
								{
									Name:  "VALUES_HASH",
									Value: hex.EncodeToString(valuesHash[:]),
								},
								{
									Name:  "HELM_DRIVER",
									Value: "secret",
								},
								{
									Name:  "CHART_NAMESPACE",
									Value: chart.Namespace,
								},
								{
									Name:  "CHART",
									Value: chart.Spec.Chart,
								},
								{
									Name:  "HELM_VERSION",
									Value: chart.Spec.HelmVersion,
								},
							},
						},
					},
					ServiceAccountName: fmt.Sprintf("helm-%s", chart.Name),
				},
			},
		},
	}

	if chart.Spec.Bootstrap {
		job.Spec.Template.Spec.HostNetwork = true
		job.Spec.Template.Spec.Tolerations = []core.Toleration{
			{
				Key:    "node.kubernetes.io/not-ready",
				Effect: "NoSchedule",
			},
		}
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, []core.EnvVar{
			{
				Name:  "KUBERNETES_SERVICE_HOST",
				Value: "127.0.0.1"},
			{
				Name:  "KUBERNETES_SERVICE_PORT",
				Value: "6443"},
		}...)
		job.Spec.Template.Spec.NodeSelector = make(map[string]string)
		job.Spec.Template.Spec.NodeSelector["node-role.kubernetes.io/master"] = "true"
	}

	setProxyEnv(job)
	valueConfigMap := setValuesConfigMap(job, chart)
	contentConfigMap := setContentConfigMap(job, chart)

	return job, valueConfigMap, contentConfigMap
}

func valuesConfigMap(chart *helmv1.HelmChart) *core.ConfigMap {
	if chart.Spec.ValuesContent == "" {
		return nil
	}

	return &core.ConfigMap{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      fmt.Sprintf("chart-values-%s", chart.Name),
			Namespace: chart.Namespace,
		},
		Data: map[string]string{
			"values.yaml": chart.Spec.ValuesContent,
		},
	}
}

func roleBinding(chart *helmv1.HelmChart) *rbac.ClusterRoleBinding {
	return &rbac.ClusterRoleBinding{
		TypeMeta: meta.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: meta.ObjectMeta{
			Name: fmt.Sprintf("helm-%s-%s", chart.Namespace, chart.Name),
		},
		RoleRef: rbac.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "cluster-admin",
		},
		Subjects: []rbac.Subject{
			{
				Name:      fmt.Sprintf("helm-%s", chart.Name),
				Kind:      "ServiceAccount",
				Namespace: chart.Namespace,
			},
		},
	}
}

func serviceAccount(chart *helmv1.HelmChart) *core.ServiceAccount {
	return &core.ServiceAccount{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      fmt.Sprintf("helm-%s", chart.Name),
			Namespace: chart.Namespace,
		},
		AutomountServiceAccountToken: &trueVal,
	}
}

func args(chart *helmv1.HelmChart) []string {
	if chart.DeletionTimestamp != nil {
		return []string{
			"delete",
		}
	}

	spec := chart.Spec
	args := []string{
		"install",
	}
	if spec.TargetNamespace != "" {
		args = append(args, "--namespace", spec.TargetNamespace)
	}
	if spec.Repo != "" {
		args = append(args, "--repo", spec.Repo)
	}
	if spec.Version != "" {
		args = append(args, "--version", spec.Version)
	}

	for _, k := range keys(spec.Set) {
		val := spec.Set[k]
		if val.StrVal == "false" || val.StrVal == "true" {
			args = append(args, "--set", fmt.Sprintf("%s=%s", k, val.StrVal))
		} else if val.StrVal != "" {
			args = append(args, "--set-string", fmt.Sprintf("%s=%s", k, val.StrVal))
		} else {
			args = append(args, "--set", fmt.Sprintf("%s=%d", k, val.IntVal))
		}
	}

	return args
}

func keys(val map[string]intstr.IntOrString) []string {
	var keys []string
	for k := range val {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func setProxyEnv(job *batch.Job) {
	proxySysEnv := []string{
		"all_proxy",
		"ALL_PROXY",
		"http_proxy",
		"HTTP_PROXY",
		"https_proxy",
		"HTTPS_PROXY",
		"no_proxy",
		"NO_PROXY",
	}
	for _, proxyEnv := range proxySysEnv {
		proxyEnvValue := os.Getenv(proxyEnv)
		if len(proxyEnvValue) == 0 {
			continue
		}
		envar := core.EnvVar{
			Name:  proxyEnv,
			Value: proxyEnvValue,
		}
		job.Spec.Template.Spec.Containers[0].Env = append(
			job.Spec.Template.Spec.Containers[0].Env,
			envar)
	}
}

func contentConfigMap(chart *helmv1.HelmChart) *core.ConfigMap {
	if chart.Spec.ChartContent == "" {
		return nil
	}
	return &core.ConfigMap{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      fmt.Sprintf("chart-content-%s", chart.Name),
			Namespace: chart.Namespace,
		},
		Data: map[string]string{
			fmt.Sprintf("%s.tgz.base64", chart.Name): chart.Spec.ChartContent,
		},
	}
}

func setValuesConfigMap(job *batch.Job, chart *helmv1.HelmChart) *core.ConfigMap {
	configMap := valuesConfigMap(chart)
	if configMap == nil {
		return nil
	}

	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, []core.Volume{
		{
			Name: "values",
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
		},
	}...)

	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, []core.VolumeMount{
		{
			MountPath: "/config",
			Name:      "values",
		},
	}...)

	return configMap
}

func setContentConfigMap(job *batch.Job, chart *helmv1.HelmChart) *core.ConfigMap {
	configMap := contentConfigMap(chart)
	if configMap == nil {
		return nil
	}

	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, []core.Volume{
		{
			Name: "content",
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
		},
	}...)

	job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, []core.VolumeMount{
		{
			MountPath: "/chart",
			Name:      "content",
		},
	}...)

	return configMap
}
