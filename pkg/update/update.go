package update

import (
	"github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// update any mention of an image with the canonical name
// canonicalName, with the latestRef. TODO: other kinds.
func UpdateImageEverywhere(inpath, outpath, imageName, latestRef string) error {
	updateImages := makeUpdateImagesFilter(imageName, latestRef)

	reader := &kio.LocalPackageReader{
		PackagePath:        inpath,
		IncludeSubpackages: true,
	}
	writer := &kio.LocalPackageWriter{
		PackagePath: outpath,
	}

	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{reader},
		Outputs: []kio.Writer{writer},
		Filters: []kio.Filter{updateImages},
	}
	return pipeline.Execute()
}

func makeUpdateImagesFilter(originalRepo, replacement string) kio.Filter {
	originalRef, err := name.ParseReference(originalRepo)
	if err != nil {
		return kio.FilterFunc(func([]*yaml.RNode) ([]*yaml.RNode, error) {
			return nil, err
		})
	}

	canonName := originalRef.Context().String()
	replacementNode := yaml.NewScalarRNode(replacement)

	replaceContainerImage := func(container *yaml.RNode) error {
		if imageField := container.Field("image"); imageField != nil {
			ref, err := name.ParseReference(imageField.Value.YNode().Value)
			if err != nil {
				return err
			}
			if ref.Context().String() == canonName {
				imageField.Value.SetYNode(replacementNode.YNode())
			}
		}
		return nil
	}

	replaceImageInEachContainer := yaml.FilterFunc(func(containers *yaml.RNode) (*yaml.RNode, error) {
		return containers, containers.VisitElements(replaceContainerImage)
	})

	return kio.FilterFunc(func(objs []*yaml.RNode) ([]*yaml.RNode, error) {
		for _, obj := range objs {
			if err := obj.PipeE(
				yaml.Lookup("spec", "template", "spec"),
				yaml.Tee(
					yaml.Lookup("initContainers"),
					replaceImageInEachContainer,
				),
				yaml.Tee(
					yaml.Lookup("containers"),
					replaceImageInEachContainer,
				),
			); err != nil {
				return nil, err
			}
		}
		return objs, nil
	})
}
