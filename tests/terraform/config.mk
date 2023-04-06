SHELL := /bin/bash

LOCAL_TFVARS_PATH := modules/k3scluster/config/local.tfvars

ifeq ($(wildcard ${LOCAL_TFVARS_PATH}),)
  RESOURCE_NAME :=
else
  export RESOURCE_NAME := $(shell sed -n 's/resource_name *= *"\([^"]*\)"/\1/p' ${LOCAL_TFVARS_PATH})
endif

export ACCESS_KEY_LOCAL
export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY