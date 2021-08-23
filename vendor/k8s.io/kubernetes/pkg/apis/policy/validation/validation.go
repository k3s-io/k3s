/*
Copyright 2016 The Kubernetes Authors.

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

package validation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"k8s.io/api/core/v1"
	policyapiv1beta1 "k8s.io/api/policy/v1beta1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	appsvalidation "k8s.io/kubernetes/pkg/apis/apps/validation"
	core "k8s.io/kubernetes/pkg/apis/core"
	apivalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/security/apparmor"
	"k8s.io/kubernetes/pkg/security/podsecuritypolicy/seccomp"
	psputil "k8s.io/kubernetes/pkg/security/podsecuritypolicy/util"
)

// ValidatePodDisruptionBudget validates a PodDisruptionBudget and returns an ErrorList
// with any errors.
func ValidatePodDisruptionBudget(pdb *policy.PodDisruptionBudget) field.ErrorList {
	allErrs := ValidatePodDisruptionBudgetSpec(pdb.Spec, field.NewPath("spec"))
	return allErrs
}

// ValidatePodDisruptionBudgetSpec validates a PodDisruptionBudgetSpec and returns an ErrorList
// with any errors.
func ValidatePodDisruptionBudgetSpec(spec policy.PodDisruptionBudgetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.MinAvailable != nil && spec.MaxUnavailable != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec, "minAvailable and maxUnavailable cannot be both set"))
	}

	if spec.MinAvailable != nil {
		allErrs = append(allErrs, appsvalidation.ValidatePositiveIntOrPercent(*spec.MinAvailable, fldPath.Child("minAvailable"))...)
		allErrs = append(allErrs, appsvalidation.IsNotMoreThan100Percent(*spec.MinAvailable, fldPath.Child("minAvailable"))...)
	}

	if spec.MaxUnavailable != nil {
		allErrs = append(allErrs, appsvalidation.ValidatePositiveIntOrPercent(*spec.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
		allErrs = append(allErrs, appsvalidation.IsNotMoreThan100Percent(*spec.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	}

	allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)

	return allErrs
}

// ValidatePodDisruptionBudgetStatusUpdate validates a PodDisruptionBudgetStatus and returns an ErrorList
// with any errors.
func ValidatePodDisruptionBudgetStatusUpdate(status, oldStatus policy.PodDisruptionBudgetStatus, fldPath *field.Path, apiVersion schema.GroupVersion) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, unversionedvalidation.ValidateConditions(status.Conditions, fldPath.Child("conditions"))...)
	// Don't run other validations for v1beta1 since we don't want to introduce
	// new validations retroactively.
	if apiVersion == policyapiv1beta1.SchemeGroupVersion {
		return allErrs
	}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.DisruptionsAllowed), fldPath.Child("disruptionsAllowed"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.CurrentHealthy), fldPath.Child("currentHealthy"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.DesiredHealthy), fldPath.Child("desiredHealthy"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.ExpectedPods), fldPath.Child("expectedPods"))...)
	return allErrs
}

// ValidatePodSecurityPolicyName can be used to check whether the given
// pod security policy name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
var ValidatePodSecurityPolicyName = apimachineryvalidation.NameIsDNSSubdomain

// PodSecurityPolicyValidationOptions contains additional parameters for ValidatePodSecurityPolicy.
type PodSecurityPolicyValidationOptions struct {
	// AllowEphemeralVolumeType determines whether Ephemeral is a valid entry
	// in PodSecurityPolicySpec.Volumes.
	AllowEphemeralVolumeType bool
}

// ValidatePodSecurityPolicy validates a PodSecurityPolicy and returns an ErrorList
// with any errors.
func ValidatePodSecurityPolicy(psp *policy.PodSecurityPolicy, opts PodSecurityPolicyValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&psp.ObjectMeta, false, ValidatePodSecurityPolicyName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpecificAnnotations(psp.Annotations, field.NewPath("metadata").Child("annotations"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpec(&psp.Spec, opts, field.NewPath("spec"))...)
	return allErrs
}

// ValidatePodSecurityPolicySpec validates a PodSecurityPolicySpec and returns an ErrorList
// with any errors.
func ValidatePodSecurityPolicySpec(spec *policy.PodSecurityPolicySpec, opts PodSecurityPolicyValidationOptions, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePSPRunAsUser(fldPath.Child("runAsUser"), &spec.RunAsUser)...)
	allErrs = append(allErrs, validatePSPRunAsGroup(fldPath.Child("runAsGroup"), spec.RunAsGroup)...)
	allErrs = append(allErrs, validatePSPSELinux(fldPath.Child("seLinux"), &spec.SELinux)...)
	allErrs = append(allErrs, validatePSPSupplementalGroup(fldPath.Child("supplementalGroups"), &spec.SupplementalGroups)...)
	allErrs = append(allErrs, validatePSPFSGroup(fldPath.Child("fsGroup"), &spec.FSGroup)...)
	allErrs = append(allErrs, validatePodSecurityPolicyVolumes(opts, fldPath, spec.Volumes)...)
	if len(spec.RequiredDropCapabilities) > 0 && hasCap(policy.AllowAllCapabilities, spec.AllowedCapabilities) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("requiredDropCapabilities"), spec.RequiredDropCapabilities,
			"must be empty when all capabilities are allowed by a wildcard"))
	}
	allErrs = append(allErrs, validatePSPCapsAgainstDrops(spec.RequiredDropCapabilities, spec.DefaultAddCapabilities, field.NewPath("defaultAddCapabilities"))...)
	allErrs = append(allErrs, validatePSPCapsAgainstDrops(spec.RequiredDropCapabilities, spec.AllowedCapabilities, field.NewPath("allowedCapabilities"))...)
	allErrs = append(allErrs, validatePSPDefaultAllowPrivilegeEscalation(fldPath.Child("defaultAllowPrivilegeEscalation"), spec.DefaultAllowPrivilegeEscalation, spec.AllowPrivilegeEscalation)...)
	allErrs = append(allErrs, validatePSPAllowedProcMountTypes(fldPath.Child("allowedProcMountTypes"), spec.AllowedProcMountTypes)...)
	allErrs = append(allErrs, validatePSPAllowedHostPaths(fldPath.Child("allowedHostPaths"), spec.AllowedHostPaths)...)
	allErrs = append(allErrs, validatePSPAllowedFlexVolumes(fldPath.Child("allowedFlexVolumes"), spec.AllowedFlexVolumes)...)
	allErrs = append(allErrs, validatePSPAllowedCSIDrivers(fldPath.Child("allowedCSIDrivers"), spec.AllowedCSIDrivers)...)
	allErrs = append(allErrs, validatePodSecurityPolicySysctls(fldPath.Child("allowedUnsafeSysctls"), spec.AllowedUnsafeSysctls)...)
	allErrs = append(allErrs, validatePodSecurityPolicySysctls(fldPath.Child("forbiddenSysctls"), spec.ForbiddenSysctls)...)
	allErrs = append(allErrs, validatePodSecurityPolicySysctlListsDoNotOverlap(fldPath.Child("allowedUnsafeSysctls"), fldPath.Child("forbiddenSysctls"), spec.AllowedUnsafeSysctls, spec.ForbiddenSysctls)...)
	allErrs = append(allErrs, validateRuntimeClassStrategy(fldPath.Child("runtimeClass"), spec.RuntimeClass)...)

	return allErrs
}

// ValidatePodSecurityPolicySpecificAnnotations validates annotations and returns an ErrorList
// with any errors.
func ValidatePodSecurityPolicySpecificAnnotations(annotations map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if p := annotations[v1.AppArmorBetaDefaultProfileAnnotationKey]; p != "" {
		if err := apparmor.ValidateProfileFormat(p); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Key(v1.AppArmorBetaDefaultProfileAnnotationKey), p, err.Error()))
		}
	}
	if allowed := annotations[v1.AppArmorBetaAllowedProfilesAnnotationKey]; allowed != "" {
		for _, p := range strings.Split(allowed, ",") {
			if err := apparmor.ValidateProfileFormat(p); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Key(v1.AppArmorBetaAllowedProfilesAnnotationKey), allowed, err.Error()))
			}
		}
	}

	if p := annotations[seccomp.DefaultProfileAnnotationKey]; p != "" {
		allErrs = append(allErrs, apivalidation.ValidateSeccompProfile(p, fldPath.Key(seccomp.DefaultProfileAnnotationKey))...)
	}
	if allowed := annotations[seccomp.AllowedProfilesAnnotationKey]; allowed != "" {
		for _, p := range strings.Split(allowed, ",") {
			if p == seccomp.AllowAny {
				continue
			}
			allErrs = append(allErrs, apivalidation.ValidateSeccompProfile(p, fldPath.Key(seccomp.AllowedProfilesAnnotationKey))...)
		}
	}
	return allErrs
}

// validatePSPAllowedHostPaths makes sure all allowed host paths follow:
// 1. path prefix is required
// 2. path prefix does not have any element which is ".."
func validatePSPAllowedHostPaths(fldPath *field.Path, allowedHostPaths []policy.AllowedHostPath) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, target := range allowedHostPaths {
		if target.PathPrefix == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i), "is required"))
			break
		}
		parts := strings.Split(filepath.ToSlash(target.PathPrefix), "/")
		for _, item := range parts {
			if item == ".." {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i), target.PathPrefix, "must not contain '..'"))
				break // even for `../../..`, one error is sufficient to make the point
			}
		}
	}

	return allErrs
}

func validatePSPAllowedFlexVolumes(fldPath *field.Path, flexVolumes []policy.AllowedFlexVolume) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(flexVolumes) > 0 {
		for idx, fv := range flexVolumes {
			if len(fv.Driver) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("allowedFlexVolumes").Index(idx).Child("driver"),
					"must specify a driver"))
			}
		}
	}
	return allErrs
}

func validatePSPAllowedCSIDrivers(fldPath *field.Path, csiDrivers []policy.AllowedCSIDriver) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(csiDrivers) > 0 {
		for idx, csiDriver := range csiDrivers {
			fieldPath := fldPath.Child("allowedCSIDriver").Index(idx).Child("name")
			allErrs = append(allErrs, apivalidation.ValidateCSIDriverName(csiDriver.Name, fieldPath)...)
		}
	}
	return allErrs
}

// validatePSPSELinux validates the SELinux fields of PodSecurityPolicy.
func validatePSPSELinux(fldPath *field.Path, seLinux *policy.SELinuxStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// ensure the selinux strategy has a valid rule
	supportedSELinuxRules := sets.NewString(
		string(policy.SELinuxStrategyMustRunAs),
		string(policy.SELinuxStrategyRunAsAny),
	)
	if !supportedSELinuxRules.Has(string(seLinux.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), seLinux.Rule, supportedSELinuxRules.List()))
	}

	return allErrs
}

// validatePSPRunAsUser validates the RunAsUser fields of PodSecurityPolicy.
func validatePSPRunAsUser(fldPath *field.Path, runAsUser *policy.RunAsUserStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// ensure the user strategy has a valid rule
	supportedRunAsUserRules := sets.NewString(
		string(policy.RunAsUserStrategyMustRunAs),
		string(policy.RunAsUserStrategyMustRunAsNonRoot),
		string(policy.RunAsUserStrategyRunAsAny),
	)
	if !supportedRunAsUserRules.Has(string(runAsUser.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), runAsUser.Rule, supportedRunAsUserRules.List()))
	}

	// validate range settings
	for idx, rng := range runAsUser.Ranges {
		allErrs = append(allErrs, validateUserIDRange(fldPath.Child("ranges").Index(idx), rng)...)
	}

	return allErrs
}

// validatePSPRunAsGroup validates the RunAsGroup fields of PodSecurityPolicy.
func validatePSPRunAsGroup(fldPath *field.Path, runAsGroup *policy.RunAsGroupStrategyOptions) field.ErrorList {
	var allErrs field.ErrorList

	if runAsGroup == nil {
		return allErrs
	}

	switch runAsGroup.Rule {
	case policy.RunAsGroupStrategyRunAsAny:
		if len(runAsGroup.Ranges) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ranges"), runAsGroup.Ranges, "Ranges must be empty"))
		}
	case policy.RunAsGroupStrategyMustRunAs, policy.RunAsGroupStrategyMayRunAs:
		if len(runAsGroup.Ranges) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ranges"), runAsGroup.Ranges, "must provide at least one range"))
		}
		// validate range settings
		for idx, rng := range runAsGroup.Ranges {
			allErrs = append(allErrs, validateGroupIDRange(fldPath.Child("ranges").Index(idx), rng)...)
		}
	default:
		supportedRunAsGroupRules := []string{
			string(policy.RunAsGroupStrategyMustRunAs),
			string(policy.RunAsGroupStrategyRunAsAny),
			string(policy.RunAsGroupStrategyMayRunAs),
		}
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), runAsGroup.Rule, supportedRunAsGroupRules))
	}
	return allErrs
}

// validatePSPFSGroup validates the FSGroupStrategyOptions fields of the PodSecurityPolicy.
func validatePSPFSGroup(fldPath *field.Path, groupOptions *policy.FSGroupStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	supportedRules := sets.NewString(
		string(policy.FSGroupStrategyMustRunAs),
		string(policy.FSGroupStrategyMayRunAs),
		string(policy.FSGroupStrategyRunAsAny),
	)
	if !supportedRules.Has(string(groupOptions.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), groupOptions.Rule, supportedRules.List()))
	}

	for idx, rng := range groupOptions.Ranges {
		allErrs = append(allErrs, validateGroupIDRange(fldPath.Child("ranges").Index(idx), rng)...)
	}
	return allErrs
}

// validatePSPSupplementalGroup validates the SupplementalGroupsStrategyOptions fields of the PodSecurityPolicy.
func validatePSPSupplementalGroup(fldPath *field.Path, groupOptions *policy.SupplementalGroupsStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	supportedRules := sets.NewString(
		string(policy.SupplementalGroupsStrategyRunAsAny),
		string(policy.SupplementalGroupsStrategyMayRunAs),
		string(policy.SupplementalGroupsStrategyMustRunAs),
	)
	if !supportedRules.Has(string(groupOptions.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), groupOptions.Rule, supportedRules.List()))
	}

	for idx, rng := range groupOptions.Ranges {
		allErrs = append(allErrs, validateGroupIDRange(fldPath.Child("ranges").Index(idx), rng)...)
	}
	return allErrs
}

// validatePodSecurityPolicyVolumes validates the volume fields of PodSecurityPolicy.
func validatePodSecurityPolicyVolumes(opts PodSecurityPolicyValidationOptions, fldPath *field.Path, volumes []policy.FSType) field.ErrorList {
	allErrs := field.ErrorList{}
	allowed := psputil.GetAllFSTypesAsSet()
	// add in the * value since that is a pseudo type that is not included by default
	allowed.Insert(string(policy.All))
	// Ephemeral may or may not be allowed.
	if !opts.AllowEphemeralVolumeType {
		allowed.Delete(string(policy.Ephemeral))
	}
	for _, v := range volumes {
		if !allowed.Has(string(v)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("volumes"), v, allowed.List()))
		}
	}
	return allErrs
}

// validatePSPDefaultAllowPrivilegeEscalation validates the DefaultAllowPrivilegeEscalation field against the AllowPrivilegeEscalation field of a PodSecurityPolicy.
func validatePSPDefaultAllowPrivilegeEscalation(fldPath *field.Path, defaultAllowPrivilegeEscalation *bool, allowPrivilegeEscalation bool) field.ErrorList {
	allErrs := field.ErrorList{}
	if defaultAllowPrivilegeEscalation != nil && *defaultAllowPrivilegeEscalation && !allowPrivilegeEscalation {
		allErrs = append(allErrs, field.Invalid(fldPath, defaultAllowPrivilegeEscalation, "Cannot set DefaultAllowPrivilegeEscalation to true without also setting AllowPrivilegeEscalation to true"))
	}

	return allErrs
}

// validatePSPAllowedProcMountTypes validates the DefaultAllowPrivilegeEscalation field against the AllowPrivilegeEscalation field of a PodSecurityPolicy.
func validatePSPAllowedProcMountTypes(fldPath *field.Path, allowedProcMountTypes []core.ProcMountType) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, procMountType := range allowedProcMountTypes {
		if err := apivalidation.ValidateProcMountType(fldPath.Index(i), procMountType); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return allErrs
}

const sysctlPatternSegmentFmt string = "([a-z0-9][-_a-z0-9]*)?[a-z0-9*]"

// SysctlPatternFmt is a regex used for matching valid sysctl patterns.
const SysctlPatternFmt string = "(" + apivalidation.SysctlSegmentFmt + "\\.)*" + sysctlPatternSegmentFmt

var sysctlPatternRegexp = regexp.MustCompile("^" + SysctlPatternFmt + "$")

// IsValidSysctlPattern checks if name is a valid sysctl pattern.
func IsValidSysctlPattern(name string) bool {
	if len(name) > apivalidation.SysctlMaxLength {
		return false
	}
	return sysctlPatternRegexp.MatchString(name)
}

func validatePodSecurityPolicySysctlListsDoNotOverlap(allowedSysctlsFldPath, forbiddenSysctlsFldPath *field.Path, allowedUnsafeSysctls, forbiddenSysctls []string) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, allowedSysctl := range allowedUnsafeSysctls {
		isAllowedSysctlPattern := false
		allowedSysctlPrefix := ""
		if strings.HasSuffix(allowedSysctl, "*") {
			isAllowedSysctlPattern = true
			allowedSysctlPrefix = strings.TrimSuffix(allowedSysctl, "*")
		}
		for j, forbiddenSysctl := range forbiddenSysctls {
			isForbiddenSysctlPattern := false
			forbiddenSysctlPrefix := ""
			if strings.HasSuffix(forbiddenSysctl, "*") {
				isForbiddenSysctlPattern = true
				forbiddenSysctlPrefix = strings.TrimSuffix(forbiddenSysctl, "*")
			}
			switch {
			case isAllowedSysctlPattern && isForbiddenSysctlPattern:
				if strings.HasPrefix(allowedSysctlPrefix, forbiddenSysctlPrefix) {
					allErrs = append(allErrs, field.Invalid(allowedSysctlsFldPath.Index(i), allowedUnsafeSysctls[i], fmt.Sprintf("sysctl overlaps with %v", forbiddenSysctl)))
				} else if strings.HasPrefix(forbiddenSysctlPrefix, allowedSysctlPrefix) {
					allErrs = append(allErrs, field.Invalid(forbiddenSysctlsFldPath.Index(j), forbiddenSysctls[j], fmt.Sprintf("sysctl overlaps with %v", allowedSysctl)))
				}
			case isAllowedSysctlPattern:
				if strings.HasPrefix(forbiddenSysctl, allowedSysctlPrefix) {
					allErrs = append(allErrs, field.Invalid(forbiddenSysctlsFldPath.Index(j), forbiddenSysctls[j], fmt.Sprintf("sysctl overlaps with %v", allowedSysctl)))
				}
			case isForbiddenSysctlPattern:
				if strings.HasPrefix(allowedSysctl, forbiddenSysctlPrefix) {
					allErrs = append(allErrs, field.Invalid(allowedSysctlsFldPath.Index(i), allowedUnsafeSysctls[i], fmt.Sprintf("sysctl overlaps with %v", forbiddenSysctl)))
				}
			default:
				if allowedSysctl == forbiddenSysctl {
					allErrs = append(allErrs, field.Invalid(allowedSysctlsFldPath.Index(i), allowedUnsafeSysctls[i], fmt.Sprintf("sysctl overlaps with %v", forbiddenSysctl)))
				}
			}
		}
	}
	return allErrs
}

// validatePodSecurityPolicySysctls validates the sysctls fields of PodSecurityPolicy.
func validatePodSecurityPolicySysctls(fldPath *field.Path, sysctls []string) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(sysctls) == 0 {
		return allErrs
	}

	coversAll := false
	for i, s := range sysctls {
		if len(s) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), sysctls[i], "empty sysctl not allowed"))
		} else if !IsValidSysctlPattern(string(s)) {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath.Index(i), sysctls[i], fmt.Sprintf("must have at most %d characters and match regex %s",
					apivalidation.SysctlMaxLength,
					SysctlPatternFmt,
				)),
			)
		} else if s[0] == '*' {
			coversAll = true
		}
	}

	if coversAll && len(sysctls) > 1 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("items"), "if '*' is present, must not specify other sysctls"))
	}

	return allErrs
}

func validateUserIDRange(fldPath *field.Path, rng policy.IDRange) field.ErrorList {
	return validateIDRanges(fldPath, rng.Min, rng.Max)
}

func validateGroupIDRange(fldPath *field.Path, rng policy.IDRange) field.ErrorList {
	return validateIDRanges(fldPath, rng.Min, rng.Max)
}

// validateIDRanges ensures the range is valid.
func validateIDRanges(fldPath *field.Path, min, max int64) field.ErrorList {
	allErrs := field.ErrorList{}

	// if 0 <= Min <= Max then we do not need to validate max.  It is always greater than or
	// equal to 0 and Min.
	if min < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("min"), min, "min cannot be negative"))
	}
	if max < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("max"), max, "max cannot be negative"))
	}
	if min > max {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("min"), min, "min cannot be greater than max"))
	}

	return allErrs
}

// validatePSPCapsAgainstDrops ensures an allowed cap is not listed in the required drops.
func validatePSPCapsAgainstDrops(requiredDrops []core.Capability, capsToCheck []core.Capability, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if requiredDrops == nil {
		return allErrs
	}
	for _, cap := range capsToCheck {
		if hasCap(cap, requiredDrops) {
			allErrs = append(allErrs, field.Invalid(fldPath, cap,
				fmt.Sprintf("capability is listed in %s and requiredDropCapabilities", fldPath.String())))
		}
	}
	return allErrs
}

// validateRuntimeClassStrategy ensures all the RuntimeClass restrictions are valid.
func validateRuntimeClassStrategy(fldPath *field.Path, rc *policy.RuntimeClassStrategyOptions) field.ErrorList {
	if rc == nil {
		return nil
	}

	var allErrs field.ErrorList

	allowed := map[string]bool{}
	for i, name := range rc.AllowedRuntimeClassNames {
		if name != policy.AllowAllRuntimeClassNames {
			allErrs = append(allErrs, apivalidation.ValidateRuntimeClassName(name, fldPath.Child("allowedRuntimeClassNames").Index(i))...)
		}
		if allowed[name] {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("allowedRuntimeClassNames").Index(i), name))
		}
		allowed[name] = true
	}

	if rc.DefaultRuntimeClassName != nil {
		allErrs = append(allErrs, apivalidation.ValidateRuntimeClassName(*rc.DefaultRuntimeClassName, fldPath.Child("defaultRuntimeClassName"))...)
		if !allowed[*rc.DefaultRuntimeClassName] && !allowed[policy.AllowAllRuntimeClassNames] {
			allErrs = append(allErrs, field.Required(fldPath.Child("allowedRuntimeClassNames"),
				fmt.Sprintf("default %q must be allowed", *rc.DefaultRuntimeClassName)))
		}
	}

	if allowed[policy.AllowAllRuntimeClassNames] && len(rc.AllowedRuntimeClassNames) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("allowedRuntimeClassNames"), rc.AllowedRuntimeClassNames, "if '*' is present, must not specify other RuntimeClass names"))
	}

	return allErrs
}

// ValidatePodSecurityPolicyUpdate validates a PSP for updates.
func ValidatePodSecurityPolicyUpdate(old *policy.PodSecurityPolicy, new *policy.PodSecurityPolicy, opts PodSecurityPolicyValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpecificAnnotations(new.Annotations, field.NewPath("metadata").Child("annotations"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpec(&new.Spec, opts, field.NewPath("spec"))...)
	return allErrs
}

// hasCap checks for needle in haystack.
func hasCap(needle core.Capability, haystack []core.Capability) bool {
	for _, c := range haystack {
		if needle == c {
			return true
		}
	}
	return false
}
