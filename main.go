package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func print(n *yaml.Node, indent int) {
	fmt.Printf("%s%d %s %s\n", strings.Repeat(" ", indent), n.Kind, n.ShortTag(), n.Value)
	for _, c := range n.Content {
		print(c, indent+2)
	}
}

func findMapNode(n *yaml.Node, key string) (*yaml.Node, int) {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if r, p := findMapNode(c, key); r != nil {
				return r, p
			}
		}
	case yaml.MappingNode:
		for i := 0; i < len(n.Content)/2; i++ {
			if n.Content[i*2].Value == key {
				p := i*2 + 1
				return n.Content[p], p
			}
		}
	}
	return nil, 0
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}
}

func mappingNode(content ...*yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.MappingNode,
		Tag:     "!!map",
		Content: content,
	}
}

func upgradeDeployment(doc *yaml.Node) bool {
	apiVersion, _ := findMapNode(doc, "apiVersion")
	if apiVersion == nil || apiVersion.Value != "extensions/v1beta1" {
		return false
	}
	// spec is where we are going to insert the selector
	spec, _ := findMapNode(doc, "spec")
	if spec == nil {
		return false
	}
	// labels to match come from inside the template
	template, p := findMapNode(spec, "template")
	if template == nil {
		return false
	}
	templateMetadata, _ := findMapNode(template, "metadata")
	if templateMetadata == nil {
		return false
	}
	labels, _ := findMapNode(templateMetadata, "labels")
	if labels == nil {
		return false
	}
	apiVersion.Value = "apps/v1"
	// insert selector ahead of template
	p -= 1
	spec.Content = append(spec.Content, nil, nil)
	copy(spec.Content[p+2:], spec.Content[p:])
	spec.Content[p] = stringNode("selector")
	spec.Content[p+1] = mappingNode(stringNode("matchLabels"), labels)
	return true
}

func main() {
	for _, filename := range os.Args[1:] {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		doc := &yaml.Node{}
		err = yaml.Unmarshal([]byte(data), doc)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if !upgradeDeployment(doc) {
			continue
		}
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0)
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		err = encoder.Encode(doc)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	}
}
