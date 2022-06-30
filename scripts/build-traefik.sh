#!/bin/bash

. ./scripts/env-common.sh
. ./scripts/env-traefik.sh

echo "Uncompress ${CHARTS_DIR}/${TRAEFIK_FILE}"
TMP_DIR=${CHARTS_DIR}/traefik-build/
mkdir -p $TMP_DIR
tar xf ${CHARTS_DIR}/${TRAEFIK_FILE} -C ${TMP_DIR}

echo "Prepare traefik CRD"
TRAEFIK_TMP_CHART=${TMP_DIR}/traefik
TRAEFIK_TMP_CRD=${TRAEFIK_TMP_CHART}-crd

# Collect information on CRDs
crd_apis=()
for crd_yaml in $(find ${TRAEFIK_TMP_CHART}/crds -type f | sort); do
echo "Processing CRD at ${crd_yaml}"
crd_group=$(yq e '.spec.group' ${crd_yaml})
crd_kind=$(yq e '.spec.names.kind' ${crd_yaml})
crd_version=$(yq e '.spec.version' ${crd_yaml})
if [[ -z "$crd_version" ]] || [[ "$crd_version" == "null" ]]; then
    crd_version=$(yq e '.spec.versions[0].name' ${crd_yaml})
fi
echo "Found CRD with GVK ${crd_group}/${crd_version}/${crd_kind}"
crd_apis+=("${crd_group}/${crd_version}/${crd_kind}")
done

# Copy base template and apply variables to the template
mkdir -p ${TRAEFIK_TMP_CRD}
cp -R ./scripts/chart-templates/crd-base/* ${TRAEFIK_TMP_CRD}
for template_file in $(find ${TRAEFIK_TMP_CRD} -type f | sort); do
# Applies any environment variables currently set onto your template file
echo "Templating ${template_file}"
eval "echo \"$(sed 's/"/\\"/g' ${template_file})\"" > ${template_file}
done

# Move anything from ${f}/charts-crd/overlay-upstream to the main chart
cp -R ${TRAEFIK_TMP_CRD}/overlay-upstream/* ${TRAEFIK_TMP_CHART}
rm -rf ${TRAEFIK_TMP_CRD}/overlay-upstream

# Modify charts to support system-default-registry
echo -e 'global:\n  systemDefaultRegistry: ""' >> ${TRAEFIK_TMP_CHART}/values.yaml
find ${TRAEFIK_TMP_CHART} -type f | xargs sed -i 's/{{ .Values.image.name }}/{{ template "system_default_registry" .}}&/g'

# Modify chart version to append package version
# If we alter our repackaging of the helm chart without also bumping the version of the
# chart, the package version portion (final two digits) of the version string in the
# traefik HelmChart manifest should be bumped accordingly.
sed -i "s/version: .*/&${TRAEFIK_PACKAGE_VERSION}/" ${TRAEFIK_TMP_CHART}/Chart.yaml

# Add dashboard annotations to main chart
cat <<EOF >>${TRAEFIK_TMP_CHART}/Chart.yaml
annotations:
fleet.cattle.io/bundle-id: k3s
EOF

# Move CRDs from main chart to CRD chart
mkdir -p ${TRAEFIK_TMP_CRD}/templates
mv ${TRAEFIK_TMP_CHART}/crds/* ${TRAEFIK_TMP_CRD}/templates
rm -rf ${TRAEFIK_TMP_CHART}/crds

# Package charts
OPTS="--format=gnu --sort=name --owner=0 --group=0 --mode=gou-s --numeric-owner --no-acls --no-selinux --no-xattrs"
tar ${OPTS} --mtime='2021-01-01 00:00:00Z' -cf - -C ${TMP_DIR} $(basename ${TRAEFIK_TMP_CHART}) | gzip -n > ${CHARTS_DIR}/${TRAEFIK_FILE}
tar ${OPTS} --mtime='2021-01-01 00:00:00Z' -cf - -C ${TMP_DIR} $(basename ${TRAEFIK_TMP_CRD}) | gzip -n > ${CHARTS_DIR}/${TRAEFIK_CRD_FILE}
for TAR in ${CHARTS_DIR}/${TRAEFIK_FILE} ${CHARTS_DIR}/${TRAEFIK_CRD_FILE}; do
sha256sum ${TAR}
stat ${TAR}
tar -vtf ${TAR}
done