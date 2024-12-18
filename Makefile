# Makefile for the Pluto Restore Assets project

.PHONY: build-local
build-local:
	eval $$(minikube -p minikube docker-env| source)
	docker build -t guardianmultimedia/pluto-restore-assets:DEV -f cmd/api/Dockerfile .
	docker build -t guardianmultimedia/pluto-restore-assets-worker:DEV -f cmd/worker/Dockerfile .

.PHONY: deploy-local
deploy-local:
	kubectl config use-context minikube
	kubectl delete pod -l service=pluto-project-restore

.PHONY: deploy-latest
deploy-latest: build-local deploy-local
