package controller

import (
	"reflect"
	"strings"
)

func ObjectInCluster(cluster string, obj interface{}) bool {
	var clusterName string
	if c := getValue(obj, "ClusterName"); c.IsValid() {
		clusterName = c.String()
	}
	if clusterName == "" {
		if c := getValue(obj, "Spec", "ClusterName"); c.IsValid() {
			clusterName = c.String()
		}

	}
	if clusterName == "" {
		if c := getValue(obj, "ProjectName"); c.IsValid() {
			if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
				clusterName = parts[0]
			}
		}
	}
	if clusterName == "" {
		if c := getValue(obj, "Spec", "ProjectName"); c.IsValid() {
			if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
				clusterName = parts[0]
			}
		}
	}
	if clusterName == "" {
		if a := getValue(obj, "Annotations"); a.IsValid() {
			if c := a.MapIndex(reflect.ValueOf("field.cattle.io/projectId")); c.IsValid() {
				if parts := strings.SplitN(c.String(), ":", 2); len(parts) == 2 {
					clusterName = parts[0]
				}
			}
		}
	}
	if clusterName == "" {
		if c := getValue(obj, "Namespace"); c.IsValid() {
			clusterName = c.String()
		}
	}

	return clusterName == cluster
}

func getValue(obj interface{}, name ...string) reflect.Value {
	v := reflect.ValueOf(obj)
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = v.Type()
	}

	field := v.FieldByName(name[0])
	if !field.IsValid() || len(name) == 1 {
		return field
	}

	return getFieldValue(field, name[1:]...)
}

func getFieldValue(v reflect.Value, name ...string) reflect.Value {
	field := v.FieldByName(name[0])
	if len(name) == 1 {
		return field
	}
	return getFieldValue(field, name[1:]...)
}
