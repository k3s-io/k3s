
##========================= Terraform Tests =========================#
include ./config.mk

TAGNAME ?= default
test-env-up:
	@cd ../.. && docker build . -q -f ./tests/acceptance/Dockerfile.build -t k3s-automated-${TAGNAME}

.PHONY: test-run
test-run:
	@docker run -d --name k3s-automated-test${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/k3s-io/k3s/tests/acceptance/modules/k3scluster/config/.ssh/aws_key.pem \
      k3s-automated-${TAGNAME} sh -c 'cd ./tests/acceptance/entrypoint ; \
      				  if [ -n "${ARGVALUE}" ]; then \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/... \
                           - "${ARGVALUE}"; \
                      elif [ -z "${TESTDIR}" ]; then \
                         go test -v -timeout=45m \
                           ./createcluster/... ./version/... ./upgradecluster/... \
                            - "${ARGVALUE}"  ; \
                      else \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/...; \
                      fi'

.PHONY: test-logs
test-logs:
	@docker logs -f k3s-automated-test${IMGNAME}

.PHONY: test-env-down
test-env-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=k3s-automated*")
	@docker rm $$(docker ps -a -q --filter="name=k3s-automated*") ; \
	 docker rmi --force $$(docker images -q --filter="reference=k3s-automated*")

test-env-clean:
	@./shared/scripts/delete_resources.sh

.PHONY: test-complete
test-complete: test-env-clean test-env-down remove-tf-state test-env-up test-run

.PHONY: remove-tf-state
remove-tf-state:
	@rm -rf ./modules/k3scluster/.terraform
	@rm -rf ./modules/k3scluster/.terraform.lock.hcl ./modules/k3scluster/terraform.tfstate ./modules/k3scluster/terraform.tfstate.backup


#========================= Run acceptance tests locally =========================#

.PHONY: test-create
test-create:
	@go test -timeout=45m -v ./entrypoint/createcluster/...

.PHONY: test-upgrade
test-upgrade:
	@go test -timeout=45m -v ./entrypoint/upgradecluster/... -upgradeVersion ${ARGVALUE}

.PHONY: test-version-local-path
test-version-local-path:
	@go test -timeout=45m -v -tags=localpath ./entrypoint/versionbump/...  -expectedValueNode ${VALUENODE} \
		-expectedValueUpgradedNode ${VALUENODEUPGRADED} -expectedValueHost ${VALUEHOST} -expectedValueUpgradedHost ${VALUEHOSTUPGRADED} -installUpgradeFlag ${INSTALLTYPE}

.PHONY: test-version-bump
test-version-local-bump:
	@go test -timeout=45m -v -tags=versionbump ./entrypoint/versionbump/... -cmdNode ${CMDNODE} -expectedValueNode ${VALUENODE} \
		-expectedValueUpgradedNode ${VALUENODEUPGRADED} -cmdHost ${CMDHOST} -expectedValueHost ${VALUEHOST} -expectedValueUpgradedHost ${VALUEHOSTUPGRADED} \
		-installUpgradeFlag ${INSTALLTYPE} -testCase ${TESTCASE}  -deployWorkload ${DEPLOYWORKLOAD}

.PHONY: test-suite
test-suite:
	@make remove-tf-state && make test-create ; sleep 5 && \
	  make remove-tf-state && make test-upgrade ${ARGVALUE} ; sleep 5 && \
	  make remove-tf-state && make test-version


#========================= TestCode Static Quality Check =========================#
.PHONY: vet-lint                   ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./${TESTDIR} && golangci-lint run --tests