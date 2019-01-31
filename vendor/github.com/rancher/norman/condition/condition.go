package condition

import (
	"reflect"
	"regexp"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"k8s.io/api/core/v1"
	err2 "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Cond string

var temfileRegexp = regexp.MustCompile("/tmp/[-_a-zA-Z0-9]+")

func (c Cond) True(obj runtime.Object) {
	setStatus(obj, string(c), "True")
}

func (c Cond) IsTrue(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "True"
}

func (c Cond) LastUpdated(obj runtime.Object, ts string) {
	setTS(obj, string(c), ts)
}

func (c Cond) GetLastUpdated(obj runtime.Object) string {
	return getTS(obj, string(c))
}

func (c Cond) False(obj runtime.Object) {
	setStatus(obj, string(c), "False")
}

func (c Cond) IsFalse(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "False"
}

func (c Cond) GetStatus(obj runtime.Object) string {
	return getStatus(obj, string(c))
}

func (c Cond) SetStatus(obj runtime.Object, status string) {
	setStatus(obj, string(c), status)
}

func (c Cond) Unknown(obj runtime.Object) {
	setStatus(obj, string(c), "Unknown")
}

func (c Cond) CreateUnknownIfNotExists(obj runtime.Object) {
	condSlice := getValue(obj, "Status", "Conditions")
	cond := findCond(condSlice, string(c))
	if cond == nil {
		c.Unknown(obj)
	}
}

func (c Cond) IsUnknown(obj runtime.Object) bool {
	return getStatus(obj, string(c)) == "Unknown"
}

func (c Cond) Reason(obj runtime.Object, reason string) {
	cond := findOrCreateCond(obj, string(c))
	getFieldValue(cond, "Reason").SetString(reason)
}

func (c Cond) SetMessageIfBlank(obj runtime.Object, message string) {
	if c.GetMessage(obj) == "" {
		c.Message(obj, message)
	}
}

func (c Cond) Message(obj runtime.Object, message string) {
	cond := findOrCreateCond(obj, string(c))
	setValue(cond, "Message", message)
}

func (c Cond) GetMessage(obj runtime.Object) string {
	cond := findOrNotCreateCond(obj, string(c))
	if cond == nil {
		return ""
	}
	return getFieldValue(*cond, "Message").String()
}

func (c Cond) ReasonAndMessageFromError(obj runtime.Object, err error) {
	if err2.IsConflict(err) {
		return
	}
	cond := findOrCreateCond(obj, string(c))
	setValue(cond, "Message", err.Error())
	switch ce := err.(type) {
	case *conditionError:
		setValue(cond, "Reason", ce.reason)
	case *controller.ForgetError:
		if ce.Reason != "" {
			setValue(cond, "Reason", ce.Reason)
		} else {
			setValue(cond, "Reason", "Error")
		}
	default:
		setValue(cond, "Reason", "Error")
	}
}

func (c Cond) GetReason(obj runtime.Object) string {
	cond := findOrNotCreateCond(obj, string(c))
	if cond == nil {
		return ""
	}
	return getFieldValue(*cond, "Reason").String()
}

func (c Cond) Once(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	if c.IsFalse(obj) {
		return obj, &controller.ForgetError{
			Err: errors.New(c.GetReason(obj)),
		}
	}

	return c.DoUntilTrue(obj, f)
}

func (c Cond) DoUntilTrue(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	if c.IsTrue(obj) {
		return obj, nil
	}

	return c.do(obj, f)
}

func (c Cond) Do(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	return c.do(obj, f)
}

type ObjectClientGetter interface {
	ObjectClient() *objectclient.ObjectClient
}

func (c Cond) Track(obj runtime.Object, client ObjectClientGetter, f func() (runtime.Object, error)) (runtime.Object, error) {
	obj = obj.DeepCopyObject()
	retObj, changed, err := c.do2(false, obj, f)
	if !changed {
		return retObj, err
	}

	c.SetStatus(retObj, c.GetStatus(obj))
	c.LastUpdated(retObj, c.GetLastUpdated(obj))
	c.Reason(retObj, c.GetReason(obj))
	c.Message(retObj, c.GetMessage(obj))

	if obj, ok := retObj.(metav1.Object); ok {
		updated, uerr := client.ObjectClient().Update(obj.GetName(), retObj)
		if uerr == nil {
			return updated, err
		}
	}

	return retObj, err
}

func (c Cond) do(obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	obj, _, err := c.do2(true, obj, f)
	return obj, err
}

func (c Cond) do2(setReturned bool, obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, bool, error) {
	status := c.GetStatus(obj)
	ts := c.GetLastUpdated(obj)
	reason := c.GetReason(obj)
	message := c.GetMessage(obj)

	checkObj := obj
	retObj, err := c.doInternal(setReturned, obj, f)
	if setReturned {
		checkObj = retObj
	}

	// This is to prevent non stop flapping of states and update
	if status == c.GetStatus(checkObj) &&
		reason == c.GetReason(checkObj) {
		if message != c.GetMessage(checkObj) {
			replaced := temfileRegexp.ReplaceAllString(c.GetMessage(checkObj), "file_path_redacted")
			c.Message(checkObj, replaced)
		}
		if message == c.GetMessage(checkObj) {
			c.LastUpdated(checkObj, ts)
		}
	}

	changed := status != c.GetStatus(checkObj) ||
		ts != c.GetLastUpdated(checkObj) ||
		reason != c.GetReason(checkObj) ||
		message != c.GetMessage(checkObj)

	return retObj, changed, err
}

func (c Cond) doInternal(setReturned bool, obj runtime.Object, f func() (runtime.Object, error)) (runtime.Object, error) {
	if !c.IsFalse(obj) {
		c.Unknown(obj)
	}

	setObject := obj
	newObj, err := f()
	if newObj != nil && !reflect.ValueOf(newObj).IsNil() {
		obj = newObj
		if setReturned {
			setObject = obj
		}
	}

	if err != nil {
		if _, ok := err.(*controller.ForgetError); ok {
			if c.GetMessage(setObject) == "" {
				c.ReasonAndMessageFromError(setObject, err)
			}
			return obj, err
		}
		c.False(setObject)
		c.ReasonAndMessageFromError(setObject, err)
		return obj, err
	}
	c.True(setObject)
	c.Reason(setObject, "")
	c.Message(setObject, "")
	return obj, nil
}

func touchTS(value reflect.Value) {
	now := time.Now().Format(time.RFC3339)
	getFieldValue(value, "LastUpdateTime").SetString(now)
}

func getStatus(obj interface{}, condName string) string {
	cond := findOrNotCreateCond(obj, condName)
	if cond == nil {
		return ""
	}
	return getFieldValue(*cond, "Status").String()
}

func setTS(obj interface{}, condName, ts string) {
	cond := findOrCreateCond(obj, condName)
	getFieldValue(cond, "LastUpdateTime").SetString(ts)
}

func getTS(obj interface{}, condName string) string {
	cond := findOrNotCreateCond(obj, condName)
	if cond == nil {
		return ""
	}
	return getFieldValue(*cond, "LastUpdateTime").String()
}

func setStatus(obj interface{}, condName, status string) {
	cond := findOrCreateCond(obj, condName)
	setValue(cond, "Status", status)
}

func setValue(cond reflect.Value, fieldName, newValue string) {
	value := getFieldValue(cond, fieldName)
	if value.String() != newValue {
		value.SetString(newValue)
		touchTS(cond)
	}
}

func findOrNotCreateCond(obj interface{}, condName string) *reflect.Value {
	condSlice := getValue(obj, "Status", "Conditions")
	return findCond(condSlice, condName)
}

func findOrCreateCond(obj interface{}, condName string) reflect.Value {
	condSlice := getValue(obj, "Status", "Conditions")
	cond := findCond(condSlice, condName)
	if cond != nil {
		return *cond
	}

	newCond := reflect.New(condSlice.Type().Elem()).Elem()
	newCond.FieldByName("Type").SetString(condName)
	newCond.FieldByName("Status").SetString("Unknown")
	condSlice.Set(reflect.Append(condSlice, newCond))
	return *findCond(condSlice, condName)
}

func findCond(val reflect.Value, name string) *reflect.Value {
	for i := 0; i < val.Len(); i++ {
		cond := val.Index(i)
		typeVal := getFieldValue(cond, "Type")
		if typeVal.String() == name {
			return &cond
		}
	}

	return nil
}

func getValue(obj interface{}, name ...string) reflect.Value {
	if obj == nil {
		return reflect.Value{}
	}
	v := reflect.ValueOf(obj)
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		v = v.Elem()
		t = v.Type()
	}

	field := v.FieldByName(name[0])
	if len(name) == 1 {
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

func Error(reason string, err error) error {
	return &conditionError{
		reason:  reason,
		message: err.Error(),
	}
}

type conditionError struct {
	reason  string
	message string
}

func (e *conditionError) Error() string {
	return e.message
}

type GenericCondition struct {
	// Type of cluster condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}
