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

package crd

import (
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

// SchemaVisitor walks the nodes of a schema.
type SchemaVisitor interface {
	// Visit is called for each schema node.  If it returns a visitor,
	// the visitor will be called on each direct child node, and then
	// this visitor will be called again with `nil` to indicate that
	// all children have been visited.  If a nil visitor is returned,
	// children are not visited.
	// level is the current depth of the visitor in the tree.
	Visit(schema *apiext.JSONSchemaProps, level int) SchemaVisitor
}

// EditSchema walks the given schema using the given visitor.  Actual
// pointers to each schema node are passed to the visitor, so any changes
// made by the visitor will be reflected to the passed-in schema.
func EditSchema(schema *apiext.JSONSchemaProps, visitor SchemaVisitor) {
	walker := schemaWalker{visitor: visitor}
	walker.walkSchema(schema, 0 /* level at the root is 0 */)
}

// schemaWalker knows how to walk the schema, saving modifications
// made by the given visitor.
type schemaWalker struct {
	visitor SchemaVisitor
}

// walkSchema walks the given schema, saving modifications made by the
// visitor (this is as simple as passing a pointer in most cases,
// but special care needs to be taken to persist with maps).
func (w schemaWalker) walkSchema(schema *apiext.JSONSchemaProps, level int) {
	subVisitor := w.visitor.Visit(schema, level)
	if subVisitor == nil {
		return
	}
	nextLevel := level + 1
	defer subVisitor.Visit(nil, nextLevel)

	subWalker := schemaWalker{visitor: subVisitor}
	if schema.Items != nil {
		subWalker.walkPtr(schema.Items.Schema, nextLevel)
		subWalker.walkSlice(schema.Items.JSONSchemas, nextLevel)
	}
	subWalker.walkSlice(schema.AllOf, nextLevel)
	subWalker.walkSlice(schema.OneOf, nextLevel)
	subWalker.walkSlice(schema.AnyOf, nextLevel)
	subWalker.walkPtr(schema.Not, nextLevel)
	subWalker.walkMap(schema.Properties, nextLevel)
	if schema.AdditionalProperties != nil {
		subWalker.walkPtr(schema.AdditionalProperties.Schema, nextLevel)
	}
	subWalker.walkMap(schema.PatternProperties, nextLevel)
	for name, dep := range schema.Dependencies {
		subWalker.walkPtr(dep.Schema, nextLevel)
		schema.Dependencies[name] = dep
	}
	if schema.AdditionalItems != nil {
		subWalker.walkPtr(schema.AdditionalItems.Schema, nextLevel)
	}
	subWalker.walkMap(schema.Definitions, nextLevel)
}

// walkMap walks over values of the given map, saving changes to them.
func (w schemaWalker) walkMap(defs map[string]apiext.JSONSchemaProps, level int) {
	for name, def := range defs {
		w.walkSchema(&def, level)
		// make sure the edits actually go through since we can't
		// take a reference to the value in the map
		defs[name] = def
	}
}

// walkSlice walks over items of the given slice.
func (w schemaWalker) walkSlice(defs []apiext.JSONSchemaProps, level int) {
	for i := range defs {
		w.walkSchema(&defs[i], level)
	}
}

// walkPtr walks over the contents of the given pointer, if it's not nil.
func (w schemaWalker) walkPtr(def *apiext.JSONSchemaProps, level int) {
	if def == nil {
		return
	}
	w.walkSchema(def, level)
}
