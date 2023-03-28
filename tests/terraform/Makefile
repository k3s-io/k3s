
##========================= Terraform Tests =========================#
include ./config.mk

TAGNAME ?= default
tf-up:
	@cd ../.. && docker build . -q -f ./tests/terraform/Dockerfile.build -t k3s-tf-${TAGNAME}

.PHONY: tf-run
tf-run:
	@docker run -d --name k3s-tf-test${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/k3s-io/k3s/tests/terraform/modules/k3scluster/config/.ssh/aws_key.pem \
      k3s-tf-${TAGNAME} sh -c 'cd ./tests/terraform ; \
      				  if [ -n "${ARGNAME}" ]; then \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/... \
                           -"${ARGNAME}"="${ARGVALUE}"; \
                       elif [ -z "${TESTDIR}" ]; then \
                         go test -v -timeout=45m \
                           ./createcluster/...; \
                       else \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/...; \
                       fi'

.PHONY: tf-logs
tf-logs:
	@docker logs -f k3s-tf-test${IMGNAME}

.PHONY: tf-down
tf-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=k3s-tf*")
	@docker rm $$(docker ps -a -q --filter="name=k3s-tf*") ; \
	 docker rmi --force $$(docker images -q --filter="reference=k3s-tf*")

tf-clean:
	@./scripts/delete_resources.sh

.PHONY: tf-complete
tf-complete: tf-clean tf-down tf-remove-state tf-up tf-run


#========================= Run terraform tests locally =========================#

.PHONY: tf-create
tf-create:
	@go test -timeout=45m -v ./createcluster/...

.PHONY: tf-upgrade
tf-upgrade:
	@go test -timeout=45m -v ./upgradecluster/... -${ARGNAME}=${ARGVALUE}

.PHONY: tf-remove-state
tf-remove-state:
	@rm -rf ./modules/k3scluster/.terraform
	@rm -rf ./modules/k3scluster/.terraform.lock.hcl ./modules/k3scluster/terraform.tfstate ./modules/k3scluster/terraform.tfstate.backup

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
	@go vet ./${TESTDIR} && golangci-lint run --tests