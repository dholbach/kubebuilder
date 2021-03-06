/*
Copyright 2019 The Kubernetes Authors.

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

package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kubebuilder/internal/config"
	"sigs.k8s.io/kubebuilder/pkg/model"
	"sigs.k8s.io/kubebuilder/pkg/scaffold/input"
	"sigs.k8s.io/kubebuilder/pkg/scaffold/resource"
	controllerv1 "sigs.k8s.io/kubebuilder/pkg/scaffold/v1/controller"
	crdv1 "sigs.k8s.io/kubebuilder/pkg/scaffold/v1/crd"
	scaffoldv2 "sigs.k8s.io/kubebuilder/pkg/scaffold/v2"
	controllerv2 "sigs.k8s.io/kubebuilder/pkg/scaffold/v2/controller"
	crdv2 "sigs.k8s.io/kubebuilder/pkg/scaffold/v2/crd"
)

// apiScaffolder contains configuration for generating scaffolding for Go type
// representing the API and controller that implements the behavior for the API.
type apiScaffolder struct {
	config   *config.Config
	resource *resource.Resource
	// plugins is the list of plugins we should allow to transform our generated scaffolding
	plugins []Plugin
	// doResource indicates whether to scaffold API Resource or not
	doResource bool
	// doController indicates whether to scaffold controller files or not
	doController bool
}

func NewAPIScaffolder(
	config *config.Config,
	res *resource.Resource,
	doResource, doController bool,
	plugins []Plugin,
) Scaffolder {
	return &apiScaffolder{
		plugins:      plugins,
		resource:     res,
		config:       config,
		doResource:   doResource,
		doController: doController,
	}
}

func (s *apiScaffolder) Scaffold() error {
	fmt.Println("Writing scaffold for you to edit...")

	switch {
	case s.config.IsV1():
		return s.scaffoldV1()
	case s.config.IsV2():
		return s.scaffoldV2()
	default:
		return fmt.Errorf("unknown project version %v", s.config.Version)
	}
}

func (s *apiScaffolder) buildUniverse() (*model.Universe, error) {
	return model.NewUniverse(
		model.WithConfig(&s.config.Config),
		// TODO: missing model.WithBoilerplate[From], needs boilerplate or path
		model.WithResource(s.resource, &s.config.Config),
	)
}

func (s *apiScaffolder) scaffoldV1() error {
	if s.doResource {
		fmt.Println(filepath.Join("pkg", "apis", s.resource.Group, s.resource.Version,
			fmt.Sprintf("%s_types.go", strings.ToLower(s.resource.Kind))))
		fmt.Println(filepath.Join("pkg", "apis", s.resource.Group, s.resource.Version,
			fmt.Sprintf("%s_types_test.go", strings.ToLower(s.resource.Kind))))

		universe, err := s.buildUniverse()
		if err != nil {
			return fmt.Errorf("error building API scaffold: %v", err)
		}

		if err := (&Scaffold{}).Execute(
			universe,
			input.Options{},
			&crdv1.Register{Resource: s.resource},
			&crdv1.Types{Resource: s.resource},
			&crdv1.VersionSuiteTest{Resource: s.resource},
			&crdv1.TypesTest{Resource: s.resource},
			&crdv1.Doc{Resource: s.resource},
			&crdv1.Group{Resource: s.resource},
			&crdv1.AddToScheme{Resource: s.resource},
			&crdv1.CRDSample{Resource: s.resource},
		); err != nil {
			return fmt.Errorf("error scaffolding APIs: %v", err)
		}
	} else {
		// disable generation of example reconcile body if not scaffolding resource
		// because this could result in a fork-bomb of k8s resources where watching a
		// deployment, replicaset etc. results in generating deployment which
		// end up generating replicaset, pod etc recursively.
		s.resource.CreateExampleReconcileBody = false
	}

	if s.doController {
		fmt.Println(filepath.Join("pkg", "controller", strings.ToLower(s.resource.Kind),
			fmt.Sprintf("%s_controller.go", strings.ToLower(s.resource.Kind))))
		fmt.Println(filepath.Join("pkg", "controller", strings.ToLower(s.resource.Kind),
			fmt.Sprintf("%s_controller_test.go", strings.ToLower(s.resource.Kind))))

		universe, err := s.buildUniverse()
		if err != nil {
			return fmt.Errorf("error building controller scaffold: %v", err)
		}

		if err := (&Scaffold{}).Execute(
			universe,
			input.Options{},
			&controllerv1.Controller{Resource: s.resource},
			&controllerv1.AddController{Resource: s.resource},
			&controllerv1.Test{Resource: s.resource},
			&controllerv1.SuiteTest{Resource: s.resource},
		); err != nil {
			return fmt.Errorf("error scaffolding controller: %v", err)
		}
	}

	return nil
}

func (s *apiScaffolder) scaffoldV2() error {
	if s.doResource {
		// Only save the resource in the config file if it didn't exist
		if s.config.AddResource(s.resource) {
			if err := s.config.Save(); err != nil {
				return fmt.Errorf("error updating project file with resource information : %v", err)
			}
		}

		var path string
		if s.config.MultiGroup {
			path = filepath.Join("apis", s.resource.Group, s.resource.Version,
				fmt.Sprintf("%s_types.go", strings.ToLower(s.resource.Kind)))
		} else {
			path = filepath.Join("api", s.resource.Version,
				fmt.Sprintf("%s_types.go", strings.ToLower(s.resource.Kind)))
		}
		fmt.Println(path)

		universe, err := s.buildUniverse()
		if err != nil {
			return fmt.Errorf("error building API scaffold: %v", err)
		}

		if err := (&Scaffold{Plugins: s.plugins}).Execute(
			universe,
			input.Options{},
			&scaffoldv2.Types{Input: input.Input{Path: path}, Resource: s.resource},
			&scaffoldv2.Group{Resource: s.resource},
			&scaffoldv2.CRDSample{Resource: s.resource},
			&scaffoldv2.CRDEditorRole{Resource: s.resource},
			&scaffoldv2.CRDViewerRole{Resource: s.resource},
			&crdv2.EnableWebhookPatch{Resource: s.resource},
			&crdv2.EnableCAInjectionPatch{Resource: s.resource},
		); err != nil {
			return fmt.Errorf("error scaffolding APIs: %v", err)
		}

		universe, err = s.buildUniverse()
		if err != nil {
			return fmt.Errorf("error building kustomization scaffold: %v", err)
		}

		kustomizationFile := &crdv2.Kustomization{Resource: s.resource}
		if err := (&Scaffold{}).Execute(
			universe,
			input.Options{},
			kustomizationFile,
			&crdv2.KustomizeConfig{},
		); err != nil {
			return fmt.Errorf("error scaffolding kustomization: %v", err)
		}

		if err := kustomizationFile.Update(); err != nil {
			return fmt.Errorf("error updating kustomization.yaml: %v", err)
		}

	} else {
		// disable generation of example reconcile body if not scaffolding resource
		// because this could result in a fork-bomb of k8s resources where watching a
		// deployment, replicaset etc. results in generating deployment which
		// end up generating replicaset, pod etc recursively.
		s.resource.CreateExampleReconcileBody = false
	}

	if s.doController {
		if s.config.MultiGroup {
			fmt.Println(filepath.Join("controllers", s.resource.Group,
				fmt.Sprintf("%s_controller.go", strings.ToLower(s.resource.Kind))))
		} else {
			fmt.Println(filepath.Join("controllers",
				fmt.Sprintf("%s_controller.go", strings.ToLower(s.resource.Kind))))
		}

		universe, err := s.buildUniverse()
		if err != nil {
			return fmt.Errorf("error building controller scaffold: %v", err)
		}

		suiteTestFile := &controllerv2.SuiteTest{Resource: s.resource}
		if err := (&Scaffold{Plugins: s.plugins}).Execute(
			universe,
			input.Options{},
			suiteTestFile,
			&controllerv2.Controller{Resource: s.resource},
		); err != nil {
			return fmt.Errorf("error scaffolding controller: %v", err)
		}

		if err := suiteTestFile.Update(); err != nil {
			return fmt.Errorf("error updating suite_test.go under controllers pkg: %v", err)
		}
	}

	if err := (&scaffoldv2.Main{}).Update(
		&scaffoldv2.MainUpdateOptions{
			Config:         &s.config.Config,
			WireResource:   s.doResource,
			WireController: s.doController,
			Resource:       s.resource,
		},
	); err != nil {
		return fmt.Errorf("error updating main.go: %v", err)
	}

	return nil
}
