// Package api contains Go structs for all LXD API objects
//
// Overview
//
// This package has Go structs for every API object, all the various
// structs are named after the object they represent and some variations of
// those structs exist for initial object creation, object update and
// object retrieval.
//
// A few convenience functions are also tied to those structs which let
// you convert between the various strucs for a given object and also query
// some of the more complex metadata that LXD can export.
package api
