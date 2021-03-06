/*
Copyright 2019 The Tekton Authors

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

package v1alpha2

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType string

var (
	AllowedOutputResources = map[PipelineResourceType]bool{
		PipelineResourceTypeStorage: true,
		PipelineResourceTypeGit:     true,
	}
)

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = "git"

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = "storage"

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = "image"

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = "cluster"

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = "pullRequest"

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = "cloudEvent"
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = []PipelineResourceType{PipelineResourceTypeGit, PipelineResourceTypeStorage, PipelineResourceTypeImage, PipelineResourceTypeCluster, PipelineResourceTypePullRequest, PipelineResourceTypeCloudEvent}

// TaskResources allows a Pipeline to declare how its DeclaredPipelineResources
// should be provided to a Task as its inputs and outputs.
type TaskResources struct {
	// Inputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	Inputs []TaskResource `json:"inputs,omitempty"`
	// Outputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	Outputs []TaskResource `json:"outputs,omitempty"`
}

// TaskResource defines an input or output Resource declared as a requirement
// by a Task. The Name field will be used to refer to these Resources within
// the Task definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this Resource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type TaskResource struct {
	ResourceDeclaration `json:",inline"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration struct {
	// Name declares the name by which a resource is referenced in the
	// definition. Resources may be referenced by name in the definition of a
	// Task's steps.
	Name string `json:"name"`
	// Type is the type of this resource;
	Type PipelineResourceType `json:"type"`
	// TargetPath is the path in workspace directory where the resource
	// will be copied.
	// +optional
	TargetPath string `json:"targetPath,omitempty"`
}

// TaskModifier is an interface to be implemented by different PipelineResources
type TaskModifier interface {
	GetStepsToPrepend() []Step
	GetStepsToAppend() []Step
	GetVolumes() []v1.Volume
}

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
type InternalTaskModifier struct {
	StepsToPrepend []Step
	StepsToAppend  []Step
	Volumes        []v1.Volume
}

// GetStepsToPrepend returns a set of Steps to prepend to the Task.
func (tm *InternalTaskModifier) GetStepsToPrepend() []Step {
	return tm.StepsToPrepend
}

// GetStepsToAppend returns a set of Steps to append to the Task.
func (tm *InternalTaskModifier) GetStepsToAppend() []Step {
	return tm.StepsToAppend
}

// GetVolumes returns a set of Volumes to prepend to the Task pod.
func (tm *InternalTaskModifier) GetVolumes() []v1.Volume {
	return tm.Volumes
}

// ApplyTaskModifier applies a modifier to the task by appending and prepending steps and volumes.
// If steps with the same name exist in ts an error will be returned. If identical Volumes have
// been added, they will not be added again. If Volumes with the same name but different contents
// have been added, an error will be returned.
func ApplyTaskModifier(ts *TaskSpec, tm TaskModifier) error {
	steps := tm.GetStepsToPrepend()
	for _, step := range steps {
		if err := checkStepNotAlreadyAdded(step, ts.Steps); err != nil {
			return err
		}
	}
	ts.Steps = append(steps, ts.Steps...)

	steps = tm.GetStepsToAppend()
	for _, step := range steps {
		if err := checkStepNotAlreadyAdded(step, ts.Steps); err != nil {
			return err
		}
	}
	ts.Steps = append(ts.Steps, steps...)

	volumes := tm.GetVolumes()
	for _, volume := range volumes {
		var alreadyAdded bool
		for _, v := range ts.Volumes {
			if volume.Name == v.Name {
				// If a Volume with the same name but different contents has already been added, we can't add both
				if d := cmp.Diff(volume, v); d != "" {
					return fmt.Errorf("tried to add volume %s already added but with different contents", volume.Name)
				}
				// If an identical Volume has already been added, don't add it again
				alreadyAdded = true
			}
		}
		if !alreadyAdded {
			ts.Volumes = append(ts.Volumes, volume)
		}
	}

	return nil
}

func checkStepNotAlreadyAdded(s Step, steps []Step) error {
	for _, step := range steps {
		if s.Name == step.Name {
			return fmt.Errorf("Step %s cannot be added again", step.Name)
		}
	}
	return nil
}
