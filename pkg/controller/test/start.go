/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"context"
	"strings"

	"github.com/citrusframework/yaks/pkg/apis/yaks/v1alpha1"
	"github.com/citrusframework/yaks/pkg/config"
	"github.com/citrusframework/yaks/pkg/install"
	"github.com/citrusframework/yaks/pkg/util/kubernetes"
	snap "github.com/container-tools/snap/pkg/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewStartAction creates a new start action
func NewStartAction() Action {
	return &startAction{}
}

type startAction struct {
	baseAction
}

// Name returns a common name of the action
func (action *startAction) Name() string {
	return "start"
}

// CanHandle tells whether this action can handle the test
func (action *startAction) CanHandle(test *v1alpha1.Test) bool {
	return test.Status.Phase == v1alpha1.TestPhasePending
}

// Handle handles the test
func (action *startAction) Handle(ctx context.Context, test *v1alpha1.Test) (*v1alpha1.Test, error) {
	// Create the viewer service account
	if err := action.ensureServiceAccountRoles(ctx, test.Namespace); err != nil {
		return nil, err
	}

	cm := action.newTestingConfigMap(ctx, test)
	pod, err := action.newTestingPod(ctx, test, cm)
	if err != nil {
		return nil, err
	}
	resources := []runtime.Object{cm, pod}
	if err := kubernetes.ReplaceResources(ctx, action.client, resources); err != nil {
		return nil, err
	}

	test.Status.Phase = v1alpha1.TestPhaseRunning
	return test, nil
}

func (action *startAction) newTestingPod(ctx context.Context, test *v1alpha1.Test, cm *v1.ConfigMap) (*v1.Pod, error) {
	controller := true
	blockOwnerDeletion := true
	pod := v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: test.Namespace,
			Name:      TestPodNameFor(test),
			Labels: map[string]string{
				"org.citrusframework.yaks/app":     "yaks",
				"org.citrusframework.yaks/test":    test.Name,
				"org.citrusframework.yaks/test-id": test.Status.TestID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         test.APIVersion,
					Kind:               test.Kind,
					Name:               test.Name,
					UID:                test.UID,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Spec: v1.PodSpec{
			ServiceAccountName: "yaks-viewer",
			Containers: []v1.Container{
				{
					Name:  "test",
					Image: config.GetTestBaseImage(),
					Command: []string{
						"mvn",
						"-f",
						"/deployments/data/yaks-runtime-maven",
						"-Dremoteresources.skip=true",
						"-Dmaven.repo.local=/deployments/artifacts/m2",
						"-s",
						"/deployments/artifacts/settings.xml",
						"test",
					},
					TerminationMessagePolicy: "FallbackToLogsOnError",
					TerminationMessagePath:   "/dev/termination-log",
					ImagePullPolicy:          v1.PullIfNotPresent,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "tests",
							MountPath: "/etc/yaks/tests",
						},
					},
					Env: []v1.EnvVar{
						{
							Name:  "YAKS_TERMINATION_LOG",
							Value: "/dev/termination-log",
						},
						{
							Name:  "YAKS_TESTS_PATH",
							Value: "/etc/yaks/tests",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "tests",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: cm.Name,
							},
						},
					},
				},
			},
		},
	}

	for _, value := range test.Spec.Env {
		pair := strings.SplitN(value, "=", 2)
		if len(pair) == 2 {
			k := strings.TrimSpace(pair[0])
			v := strings.TrimSpace(pair[1])

			if len(k) > 0 && len(v) > 0 {
				pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
					Name:  k,
					Value: v,
				})
			}
		}
	}

	if test.Spec.Settings.Name != "" {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "YAKS_SETTINGS_FILE",
			Value: "/etc/yaks/tests/" + test.Spec.Settings.Name,
		},
		)
	} else if test.Spec.Settings.Content != "" {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "YAKS_DEPENDENCIES",
			Value: test.Spec.Settings.Content,
		},
		)
	}

	if err := action.injectSnap(ctx, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (action *startAction) newTestingConfigMap(ctx context.Context, test *v1alpha1.Test) *v1.ConfigMap {
	controller := true
	blockOwnerDeletion := true

	sources := make(map[string]string)
	sources[test.Spec.Source.Name] = test.Spec.Source.Content

	if test.Spec.Settings.Name != "" {
		sources[test.Spec.Settings.Name] = test.Spec.Settings.Content
	}

	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: test.Namespace,
			Name:      TestResourceNameFor(test),
			Labels: map[string]string{
				"org.citrusframework.yaks/app":     "yaks",
				"org.citrusframework.yaks/test":    test.Name,
				"org.citrusframework.yaks/test-id": test.Status.TestID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         test.APIVersion,
					Kind:               test.Kind,
					Name:               test.Name,
					UID:                test.UID,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
		Data: sources,
	}
	return &cm
}

func (action *startAction) ensureServiceAccountRoles(ctx context.Context, namespace string) error {
	rb := v1beta1.RoleBinding{}
	rbKey := client.ObjectKey{
		Name:      "yaks-viewer",
		Namespace: namespace,
	}

	err := action.client.Get(ctx, rbKey, &rb)
	if err != nil && k8serrors.IsNotFound(err) {
		// Create proper service account and roles
		return install.ViewerServiceAccountRoles(ctx, action.client, namespace)
	}
	return err
}

func (action *startAction) injectSnap(ctx context.Context, pod *v1.Pod) error {
	bucket := "yaks"
	options := snap.SnapOptions{
		Bucket: bucket,
	}
	s3, err := snap.NewSnap(action.config, pod.Namespace, true, options)
	if err != nil {
		return err
	}
	installed, err := s3.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		// Adding env var to enable the S3 service
		url, err := s3.GetEndpoint(ctx)
		if err != nil {
			return err
		}
		creds, err := s3.GetCredentials(ctx)
		if err != nil {
			return err
		}

		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "YAKS_S3_REPOSITORY_URL",
			Value: url,
		})
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "YAKS_S3_REPOSITORY_BUCKET",
			Value: bucket,
		})
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name: "YAKS_S3_REPOSITORY_ACCESS_KEY",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: creds.SecretName,
					},
					Key: creds.AccessKeyEntry,
				},
			},
		})
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{
			Name: "YAKS_S3_REPOSITORY_SECRET_KEY",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: creds.SecretName,
					},
					Key: creds.SecretKeyEntry,
				},
			},
		})
	}
	return nil
}
