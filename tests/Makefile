
##========================= Terraform Tests =========================#
include ./config.mk

tf-tests-up:
	@@cd ../ && docker build . -q -f ./tests/terraform/Dockerfile.build -t k3s-tf

.PHONY: tf-tests-run
tf-tests-run:
	@docker run -d --rm --name k3s-tf-test${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/k3s-io/k3s/tests/terraform/modules/k3scluster/config/.ssh/aws_key.pem \
      k3s-tf sh -c 'if [ -n "${ARGNAME}" ]; then \
                         go test -v -timeout=45m \
                           ./tests/${TESTDIR}/... \
                           -"${ARGNAME}"="${ARGVALUE}"; \
                       elif [ -z "${TESTDIR}" ]; then \
                         go test -v -timeout=45m \
                           ./tests/terraform/createcluster/...; \
                       else \
                         go test -v -timeout=45m \
                           ./tests/${TESTDIR}/...; \
                       fi'

.PHONY: tf-tests-logs
tf-tests-logs:
	@docker logs -f k3s-tf-test${IMGNAME}

.PHONY: tf-tests-down
tf-tests-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=k3s-tf*")
	@docker rm $$(docker ps -a -q --filter="name=k3s-tf*")

tf-tests-clean:
	@./terraform/scripts/delete_resources.sh

.PHONY: tf-tests-complete
tf-tests-complete: tf-tests-clean tf-tests-down tf-remove-state tf-tests-up tf-test-suite


#========================= Run terraform tests locally =========================#

.PHONY: tf-create
tf-create:
	@go test -timeout=45m -v ./terraform/createcluster/...

.PHONY: tf-upgrade
tf-upgrade:
	@go test -timeout=45m -v ./terraform/upgradecluster/... -${ARGNAME}=${ARGVALUE}

.PHONY: tf-remove-state
tf-remove-state:
	@rm -rf ./terraform/modules/k3scluster/.terraform
	@rm -rf ./terraform/modules/k3scluster/.terraform.lock.hcl ./terraform/modules/k3scluster/terraform.tfstate ./terraform/modules/k3scluster/terraform.tfstate.backup

.PHONY: tf-test-suite
tf-test-suite:
	@make tf-remove-state && make tf-create ; sleep 5 && \
	  make tf-remove-state && make tf-upgrade ${ARGNAME}=${ARGVALUE}

.PHONY: tf-test-suite-same-cluster
tf-test-suite-same-cluster:
	@make tf-create ; sleep 5 && make v ${ARGNAME}=${ARGVALUE}


#========================= TestCode Static Quality Check =========================#
.PHONY: vet-lint                   ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./${TESTDIR}
	@cd ${TESTDIR} && golangci-lint run --tests